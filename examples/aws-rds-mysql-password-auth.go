package examples

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Invicton-Labs/go-stackerr"
	gormauth "github.com/Invicton-Labs/gorm-auth"
	"github.com/Invicton-Labs/gorm-auth/authenticators"
	gormauthaws "github.com/Invicton-Labs/gorm-auth/aws"
	"github.com/Invicton-Labs/gorm-auth/dialectors"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/plugin/dbresolver"

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
func checkIfNewCredentialsNeeded(ctx context.Context) (bool, stackerr.Error) {
	// In here, you'd determine whether we need to fetch new credentials
	// for the next database connection. Generally, this would be some
	// sort of mechanism for determining whether credentials have
	// been rotated (and thus, must be reloaded) since the last connection
	// was made.

	// For our example, we will always re-fetch the credentials
	return true, nil
}

// A function for getting the connection secret
func getSecret(_ context.Context) (secret MysqlSecret, err stackerr.Error) {
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
		return secret, stackerr.Wrap(err)
	}

	return secret, nil
}

func getCredentials(ctx context.Context) (credentials authenticators.PasswordCredentials, err stackerr.Error) {
	// Load the secret
	secret, err := getSecret(ctx)
	if err != nil {
		return credentials, err
	}
	credentials.Username = secret.Username
	credentials.Password = secret.Password
	return credentials, nil
}

func AwsRdsMysqlPasswordAuth(ctx context.Context) (*gorm.DB, stackerr.Error) {

	gormConfig := &gorm.Config{
		// Insert GORM general settings here
		CreateBatchSize: 1000,
		// ... many other settings available
	}

	gormMysqlConfig := gormmysql.Config{
		// Insert MySQL-specific GORM settings here
		DefaultStringSize: 256,
		// ... many other settings available
	}

	// This is a configuration for MySQL connections. It relates
	// to settings used by the MySQL driver, not GORM.
	// We start with the default config
	mysqlConfig := mysql.NewConfig()

	// And customize some fields
	mysqlConfig.Loc = time.UTC
	mysqlConfig.Collation = "utf8mb4_0900_ai_ci"
	// ... other settings available

	// Load the database secret
	secret, err := getSecret(ctx)
	if err != nil {
		return nil, err
	}

	// Create a password-based authentication object
	authenticator := &authenticators.MysqlConnectionParametersPassword{
		Host:           secret.Host,
		Port:           secret.Port,
		Schema:         secret.Database,
		GetCredentials: getCredentials,
	}

	// The maximum number of connections we can have open to the
	// write host.
	writeMaxOpenConnections := 3
	writeInputs := []*gormauth.ConnectionParameters{
		{
			DialectorInput: dialectors.MysqlDialectorInput{
				DialectorInput: dialectors.DialectorInput{
					// Set the function that checks if new credentials should be loaded
					ShouldReconfigureCallback: checkIfNewCredentialsNeeded,

					// Some general GORM settings for the connection management
					MaxOpenConns: &writeMaxOpenConnections,
					// ...several other settings available
				},

				// Set the GORM-specific MySQL settings to use for this dialector
				GormMysqlConfig: gormMysqlConfig,

				// Set a function that returns the MySQL config to use. This
				// allows changing parameters for each new connection, if desired.
				// The host/port/user/password fields don't need to be provided
				// because they are overwritten by the password authentication system.
				GetMysqlConfigCallback: func(ctx context.Context) (*mysql.Config, stackerr.Error) {
					return mysqlConfig, nil
				},
			},
			// Use the AWS TLS configuration (change if you're connecting elsewhere)
			GetTlsConfigFunc: gormauthaws.GetTlsConfig,
			AuthSettings:     authenticator,
		},
	}

	// The maximum number of connections we can have open to the
	// read host.
	readMaxOpenConnections := 3
	readInputs := []*gormauth.ConnectionParameters{
		{
			DialectorInput: dialectors.MysqlDialectorInput{
				DialectorInput: dialectors.DialectorInput{
					// Set the function that checks if new credentials should be loaded
					ShouldReconfigureCallback: checkIfNewCredentialsNeeded,

					// Some general GORM settings for the connection management
					MaxOpenConns: &readMaxOpenConnections,
					// ...several other settings available
				},
				// Set the GORM-specific MySQL settings to use for this dialector
				GormMysqlConfig: gormMysqlConfig,

				// Set a function that returns the MySQL config to use. This
				// allows changing parameters for each new connection, if desired.
				// The host/port/user/password fields don't need to be provided
				// because they are overwritten by the password authentication system.
				GetMysqlConfigCallback: func(ctx context.Context) (*mysql.Config, stackerr.Error) {
					return mysqlConfig, nil
				},
			},
			// Use the AWS TLS configuration (change if you're connecting elsewhere)
			GetTlsConfigFunc: gormauthaws.GetTlsConfig,
			// For this demo, use the same authenticator for both write and read
			AuthSettings: authenticator,
		},
	}

	return gormauth.GetMysqlGorm(ctx, gormauth.GetMysqlGormInput{
		WriteConnectionParameters: writeInputs,
		ReadConnectionParameters:  readInputs,
		GormOptions: []gorm.Option{
			gormConfig,
		},
		// The policy to use to select which read replica to connect to.
		// In this example, it doesn't do anything since there only is
		// one read replica.
		ReplicaPolicy: dbresolver.StrictRoundRobinPolicy(),
	})
}
