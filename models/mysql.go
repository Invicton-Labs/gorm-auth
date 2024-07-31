package gormauthmodels

import (
	"context"

	"github.com/Invicton-Labs/go-stackerr"
)

type MysqlConnectionParameters struct {
	// The host of the primary cluster
	Host string `json:"host"`
	// The port to connect to the primary cluster
	Port int `json:"port"`
	// The name of the database to connect to
	Schema string `json:"database"`
}

type DatabaseCredentials struct {
	// The username to connect to the database with
	Username string `json:"username"`
	// The password to connect to the database with
	Password string `json:"password"`
}

type MysqlConnectionParametersPassword struct {
	MysqlConnectionParameters
	// A function for dynamically retrieving the username/password
	GetCredentials func(ctx context.Context) (DatabaseCredentials, stackerr.Error)
}

// MysqlSecretPasswordWithReadOnly is an extension of MysqlSecretPassword
// that adds fields for separate read-only connections.
type MysqlSecretPasswordWithReadOnly struct {
	MysqlConnectionParametersPassword
	// The host for the read-only connection (required).
	// If not provided, the write host will be used for
	// read-only connections.
	HostReadOnly string `json:"host_read_only"`
	// The port for connecting to the read-only cluster.
	// If not provided, the write port will be used for
	// read-only connections.
	PortReadOnly int `json:"port_read_only"`
	// A function for dynamically retrieving the username/password for the read-only connection.
	// If not provided, the credentials function for the write host will be used for
	// read-only connections.
	GetCredentialsReadOnly func(ctx context.Context) (DatabaseCredentials, stackerr.Error)
}
