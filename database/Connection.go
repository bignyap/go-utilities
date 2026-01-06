package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// Driver represents a supported SQL driver.
type Driver string

const (
	PostgresDriver Driver = "postgres"
	MySQLDriver    Driver = "mysql"
	SQLiteDriver   Driver = "sqlite3"
)

func ParseDriver(input string) (Driver, error) {
	switch strings.ToLower(input) {
	case "postgres":
		return PostgresDriver, nil
	case "mysql":
		return MySQLDriver, nil
	case "sqlite", "sqlite3":
		return SQLiteDriver, nil
	default:
		return "", fmt.Errorf("unsupported driver: %s", input)
	}
}

type ConnectionString struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	Options  map[string]string
}

type ConnectionPoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration
	EnableTelemetry bool // Enable OpenTelemetry tracing for database operations
}

func DefaultPoolConfig() *ConnectionPoolConfig {
	return &ConnectionPoolConfig{
		MaxOpenConns:    30,
		MaxIdleConns:    10,
		ConnMaxIdleTime: 5 * time.Minute,
		ConnMaxLifetime: 10 * time.Minute,
	}
}

type Connection struct {
	Driver           Driver
	ConnectionString *ConnectionString
	PoolConfig       *ConnectionPoolConfig
	DB               *sql.DB
	PgxPool          *pgxpool.Pool
}

func NewConnectionString(
	host, port, user, password, database string,
	options map[string]string,
) *ConnectionString {
	return &ConnectionString{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Database: database,
		Options:  options,
	}
}

func NewConnectionPoolConfig(
	maxOpenConns int,
	maxIdleConns int,
	connMaxIdleTime time.Duration,
	connMaxLifetime time.Duration,
) *ConnectionPoolConfig {
	return &ConnectionPoolConfig{
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxIdleTime: connMaxIdleTime,
		ConnMaxLifetime: connMaxLifetime,
	}
}

func NewConnection(driver Driver, cs *ConnectionString, pool *ConnectionPoolConfig) (*Connection, error) {
	if cs == nil {
		return nil, errors.New("connection string cannot be nil")
	}
	if pool == nil {
		pool = DefaultPoolConfig()
	}
	return &Connection{
		Driver:           driver,
		ConnectionString: cs,
		PoolConfig:       pool,
	}, nil
}

func (cs *ConnectionString) DSN(driver Driver) string {
	switch driver {
	case PostgresDriver:
		optionString := ""
		for key, value := range cs.Options {
			optionString += fmt.Sprintf("%s=%s ", key, value)
		}
		return fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s %s",
			cs.Host, cs.Port, cs.User, cs.Password, cs.Database, optionString,
		)
	case MySQLDriver:
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", cs.User, cs.Password, cs.Host, cs.Port, cs.Database)
		if len(cs.Options) > 0 {
			dsn += "?"
			for key, val := range cs.Options {
				dsn += fmt.Sprintf("%s=%s&", key, val)
			}
			dsn = dsn[:len(dsn)-1]
		}
		return dsn
	case SQLiteDriver:
		return cs.Database
	default:
		return ""
	}
}

func (c *Connection) Connect() error {
	dsn := c.ConnectionString.DSN(c.Driver)
	if dsn == "" {
		return fmt.Errorf("invalid or unsupported driver: %s", c.Driver)
	}

	if c.Driver == PostgresDriver {
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			return fmt.Errorf("failed to parse pgx DSN: %w", err)
		}

		cfg.MaxConns = int32(c.PoolConfig.MaxOpenConns)

		// Add OpenTelemetry tracing if enabled
		if c.PoolConfig.EnableTelemetry {
			cfg.ConnConfig.Tracer = otelpgx.NewTracer()
		}

		ctx := context.Background()
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			return fmt.Errorf("failed to create pgx pool: %w", err)
		}

		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("failed to ping pgx pool: %w", err)
		}

		c.PgxPool = pool
		return nil
	}

	db, err := sql.Open(string(c.Driver), dsn)
	if err != nil {
		return fmt.Errorf("failed to open DB using driver %s: %w", c.Driver, err)
	}

	db.SetMaxOpenConns(c.PoolConfig.MaxOpenConns)
	db.SetMaxIdleConns(c.PoolConfig.MaxIdleConns)
	db.SetConnMaxIdleTime(c.PoolConfig.ConnMaxIdleTime)
	db.SetConnMaxLifetime(c.PoolConfig.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping DB: %w", err)
	}

	c.DB = db
	return nil
}

func (c *Connection) Close() error {
	if c.PgxPool != nil {
		c.PgxPool.Close()
	}
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}

func (c *Connection) GetSQLDB() *sql.DB {
	return c.DB
}

func (c *Connection) GetPgxPool() *pgxpool.Pool {
	return c.PgxPool
}

func (c *Connection) GetConnectionString() *ConnectionString {
	return c.ConnectionString
}
