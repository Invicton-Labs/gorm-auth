package authenticators

import (
	"context"
	"fmt"

	"github.com/Invicton-Labs/go-stackerr"
	"github.com/Invicton-Labs/gorm-auth/dialectors"
	"github.com/go-sql-driver/mysql"
)

type PasswordCredentials struct {
	// The username to connect to the database with
	Username string `json:"username"`
	// The password to connect to the database with
	Password string `json:"password"`
}

type MysqlConnectionParametersPassword struct {
	// The host of the primary cluster
	Host string `json:"host"`
	// The port to connect to the primary cluster
	Port int `json:"port"`
	// The name of the database to connect to
	Schema string `json:"database"`
	// A function for dynamically retrieving the username/password
	GetCredentials func(ctx context.Context) (PasswordCredentials, stackerr.Error)
}

func (params *MysqlConnectionParametersPassword) UpdateDialectorSettings(dialectorInput dialectors.MysqlDialectorInput) (dialectors.MysqlDialectorInput, stackerr.Error) {
	return dialectorInput, nil
}

func (params *MysqlConnectionParametersPassword) UpdateConfigWithAuth(ctx context.Context, config mysql.Config) (*mysql.Config, stackerr.Error) {
	// Get the credentials
	creds, err := params.GetCredentials(ctx)
	if err != nil {
		return &config, err
	}
	config.Addr = fmt.Sprintf("%s:%d", params.Host, params.Port)
	config.DBName = params.Schema
	config.User = creds.Username
	config.Passwd = creds.Password
	return &config, nil
}
