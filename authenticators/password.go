package authenticators

import (
	"context"
	"fmt"

	"github.com/Invicton-Labs/go-stackerr"
	gormauthmysql "github.com/Invicton-Labs/gorm-auth/mysql"
	"github.com/go-sql-driver/mysql"
)

type DatabaseCredentials struct {
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
	GetCredentials func(ctx context.Context) (DatabaseCredentials, stackerr.Error)
}

func (params *MysqlConnectionParametersPassword) GetAuthParameters(ctx context.Context) (gormauthmysql.AuthParameters, stackerr.Error) {
	config := mysql.NewConfig()
	creds, err := params.GetCredentials(ctx)
	if err != nil {
		return gormauthmysql.AuthParameters{}, err
	}
	config.Addr = fmt.Sprintf("%s:%d", params.Host, params.Port)
	config.DBName = params.Schema
	config.User = creds.Username
	config.Passwd = creds.Password
	return gormauthmysql.AuthParameters{
		Host:     params.Host,
		Port:     params.Port,
		Schema:   params.Schema,
		Username: creds.Username,
		Password: creds.Password,
	}, nil
}
