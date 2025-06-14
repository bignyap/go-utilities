package database

import (
	"database/sql"
	"fmt"
	"time"
)

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
	ConnectionString *ConnectionString
	PoolConfig       *ConnectionPoolConfig
	DB               *sql.DB
}

func (c *ConnectionString) String() string {
	optionString := ""
	for key, value := range c.Options {
		optionString += fmt.Sprintf("%s=%s ", key, value)
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s %s",
		c.Host, c.Port, c.User, c.Password, c.Database, optionString,
	)
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

func NewConnection(cs *ConnectionString, pool *ConnectionPoolConfig) *Connection {
	if pool == nil {
		pool = DefaultPoolConfig()
	}
	return &Connection{
		ConnectionString: cs,
		PoolConfig:       pool,
	}
}

func (c *Connection) Connect() error {
	db, err := sql.Open("postgres", c.ConnectionString.String())
	if err != nil {
		return fmt.Errorf("failed to open DB: %w", err)
	}

	// Apply connection pool settings
	db.SetMaxOpenConns(c.PoolConfig.MaxOpenConns)
	db.SetMaxIdleConns(c.PoolConfig.MaxIdleConns)
	db.SetConnMaxIdleTime(c.PoolConfig.ConnMaxIdleTime)
	db.SetConnMaxLifetime(c.PoolConfig.ConnMaxLifetime)

	// Test the connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping DB: %w", err)
	}

	c.DB = db
	return nil
}

func (c *Connection) Close() error {
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}

func (c *Connection) GetDB() *sql.DB {
	return c.DB
}

func (c *Connection) GetConnectionString() *ConnectionString {
	return c.ConnectionString
}
