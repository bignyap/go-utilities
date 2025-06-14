package database

type DatabaseConfig struct {
	Name                 string
	ConnectionString     *ConnectionString
	ConnectionPoolConfig *ConnectionPoolConfig
}

type Database struct {
	Config     *DatabaseConfig
	Connection *Connection
}

func NewDatabase(config *DatabaseConfig) *Database {
	return &Database{
		Config:     config,
		Connection: NewConnection(config.ConnectionString, config.ConnectionPoolConfig),
	}
}
