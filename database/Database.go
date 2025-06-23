package database

type DatabaseConfig struct {
	Name                 string
	Driver               Driver
	ConnectionString     *ConnectionString
	ConnectionPoolConfig *ConnectionPoolConfig
}

type Database struct {
	Config     *DatabaseConfig
	Connection *Connection
}

func NewDatabase(config *DatabaseConfig) (*Database, error) {

	conn, err := NewConnection(config.Driver, config.ConnectionString, config.ConnectionPoolConfig)
	if err != nil {
		return nil, err
	}
	return &Database{
		Config:     config,
		Connection: conn,
	}, nil
}
