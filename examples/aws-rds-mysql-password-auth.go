package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	gormauth "github.com/Invicton-Labs/gorm-auth"
	gormaws "github.com/Invicton-Labs/gorm-auth/aws"
	"github.com/Invicton-Labs/gorm-auth/dialectors"
	gormmysql "gorm.io/driver/mysql"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type MysqlSecret struct {
	Host         string `json:"host"`
	HostReadOnly string `json:"host_read_only"`
	Port         int    `json:"port"`
	Database     string `json:"database"`
	Username     string `json:"username"`
	Password     string `json:"password"`
}

// A function for determining whether we need to reload credentials
func checkIfNewCredentialsNeeded(ctx context.Context) (bool, error) {
	// In here, you'd determine whether we need to fetch new credentials
	// for the next database connection. Generally, this would be some
	// sort of mechanism for determining whether credentials have
	// been rotated (and thus, must be reloaded) since the last connection
	// was made.

	// For our example, we will always re-fetch the credentials
	return true, nil
}

// A function for getting the connection secret
func getSecret(ctx context.Context) (secret MysqlSecret, err error) {
	// For this example, we're loading the secret from a static value,
	// but in real usage you'd be loading it from some external vault
	// (e.g. AWS Secrets Manager, AWS SSM Parameter Store, etc.).

	secretJson := []byte(`
	{
		"host": "mycluster.cluster-123456789012.us-east-1.rds.amazonaws.com",
		"host_read_only": "mycluster-ro.cluster-123456789012.us-east-1.rds.amazonaws.com",
		"port": 3306,
		"database": "myschema",
		"username": "api-user",
		"password": "magic"
	}
	`)

	// Unmarshal the JSON into a struct
	if err := json.Unmarshal([]byte(secretJson), &secret); err != nil {
		return secret, err
	}

	return secret, nil
}

// This is an example function that shows how you might get the base
// MySQL configuration, before any authentication is added
func getBaseConfig() (*mysql.Config, error) {
	cfg := mysql.NewConfig()
	cfg.Loc = time.UTC
	cfg.Collation = "utf8mb4_0900_ai_ci"
	return cfg, nil
}

// A function for dynamically creating a MySQL config for connections
// to the writer instance.
func createMysqlConfigWrite(ctx context.Context) (*mysql.Config, error) {
	// Load the secret
	secret, err := getSecret(ctx)
	if err != nil {
		return nil, err
	}

	// Start with a default MySQL config
	config, err := getBaseConfig()
	if err != nil {
		return nil, err
	}

	// Set the fields needed for authentication. Note that we're
	// using the `Host` field of the secret, which represents the
	// writer/master cluster host.
	config.Addr = fmt.Sprintf("%s:%d", secret.Host, secret.Port)
	config.DBName = secret.Database
	config.User = secret.Username
	config.Passwd = secret.Password

	return config, nil
}

// A function for dynamically creating a MySQL config for connections
// to the writer instance.
func createMysqlConfigRead(ctx context.Context) (*mysql.Config, error) {
	// Load the secret
	secret, err := getSecret(ctx)
	if err != nil {
		return nil, err
	}

	// Start with a default MySQL config
	config, err := getBaseConfig()
	if err != nil {
		return nil, err
	}

	// Set the fields needed for authentication. Note that we're
	// using the `ReadOnlyHost` field of the secret, which represents the
	// cluster's read replica host.
	config.Addr = fmt.Sprintf("%s:%d", secret.HostReadOnly, secret.Port)
	config.DBName = secret.Database
	config.User = secret.Username
	config.Passwd = secret.Password

	return config, nil
}

func AwsRdsMysqlPasswordAuth(ctx context.Context) (*gorm.DB, error) {

	gormConfig := &gorm.Config{
		// Insert GORM general settings here
		CreateBatchSize: 1000,
		// ... many other settings available
	}

	gormMysqlConfig := gormmysql.Config{
		// Insert MySql-specific GORM settings here
		DefaultStringSize: 256,
		// ... many other settings available
	}

	// The maximum number of connections we can have open to the
	// write host.
	writeMaxOpenConnections := 3
	writeDialectorInput := dialectors.MysqlDialectorInput{
		DialectorInput: dialectors.DialectorInput{
			// Set the function that checks if new credentials should be loaded
			ShouldReconfigureCallback: checkIfNewCredentialsNeeded,

			// Some general GORM settings for the connection management
			MaxOpenConns: &writeMaxOpenConnections,
			// ...several other settings available
		},

		// Set the GORM-specific MySQL settings to use for this dialector
		GormMysqlConfig: gormMysqlConfig,

		// Set the function that dynamically gets the config for connecting
		// to the write host.
		GetMysqlConfigCallback: createMysqlConfigWrite,
	}

	// The maximum number of connections we can have open to the
	// read host.
	readMaxOpenConnections := 3
	readDialectorInput := dialectors.MysqlDialectorInput{
		DialectorInput: dialectors.DialectorInput{
			// Set the function that checks if new credentials should be loaded
			ShouldReconfigureCallback: checkIfNewCredentialsNeeded,

			// Some general GORM settings for the connection management
			MaxOpenConns: &readMaxOpenConnections,
			// ...several other settings available
		},
		// Set the GORM-specific MySQL settings to use for this dialector
		GormMysqlConfig: gormMysqlConfig,

		// Set the function that dynamically gets the config for connecting
		// to the read host.
		GetMysqlConfigCallback: createMysqlConfigRead,
	}

	return gormauth.GetMysqlGorm(ctx, gormauth.GetMysqlGormInput{
		GormOptions: []gorm.Option{
			gormConfig,
		},
		WriteDialectorInput: writeDialectorInput,
		ReadDialectorInput:  readDialectorInput,
		// In this example, our database host is AWS, so we use the
		// helper function that gets the AWS TLS config for us
		GetTlsConfigFunc: gormaws.GetTlsConfig,
	})
}
