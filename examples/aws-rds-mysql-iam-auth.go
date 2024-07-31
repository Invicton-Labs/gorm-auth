package examples

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Invicton-Labs/go-stackerr"
	gormauthaws "github.com/Invicton-Labs/gorm-auth/aws"
	gormauthawsiam "github.com/Invicton-Labs/gorm-auth/aws/iam-auth"
	"github.com/Invicton-Labs/gorm-auth/dialectors"
	gormauthmysql "github.com/Invicton-Labs/gorm-auth/mysql"
	"github.com/aws/aws-sdk-go-v2/config"
	gormmysql "gorm.io/driver/mysql"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const (
	// This is an example JSON configuration that you might load from
	// Secrets Manager or SSM Parameter Store.
	AwsRdsIamAuthJson string = `
{
	"host": "mycluster.cluster-123456789012.us-east-1.rds.amazonaws.com",
	"read_only_host": "mycluster-ro.cluster-123456789012.us-east-1.rds.amazonaws.com",
	"port": 3306,
	"database": "myschema",
	"region": "ca-central-1",
	"username": "api-user"
}
`
)

func AwsRdsMysqlIamAuth(ctx context.Context) (*gorm.DB, stackerr.Error) {

	// Unmarshal the JSON into a struct that can be used for
	// generating tokens.
	var iamAuthSettings gormauthawsiam.RdsIamAuthWithReadOnly
	if err := json.Unmarshal([]byte(AwsRdsIamAuthJson), &iamAuthSettings); err != nil {
		return nil, stackerr.Wrap(err)
	}

	// Load the default AWS config from the environment variables.
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	iamAuthSettings.AwsConfig = awsCfg

	// These are configuration options for GORM itself,
	// separate from any database-specific options.
	gormConfig := &gorm.Config{
		// Insert GORM general settings here
		CreateBatchSize: 1000,
		// ... many other settings available
	}

	// These are configuration options for GORM's interaction
	// with MySQL databases.
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

	// The maximum number of connections we can have open to the
	// write host.
	writeMaxOpenConnections := 3
	writeDialectorInput := dialectors.MysqlDialectorInput{
		DialectorInput: dialectors.DialectorInput{
			// Some general GORM settings for the connection management
			MaxOpenConns: &writeMaxOpenConnections,
			// ...several other settings available
		},

		// Set the GORM-specific MySQL settings to use for this dialector
		GormMysqlConfig: gormMysqlConfig,

		// Set a function that returns the MySQL config to use. This
		// allows changing parameters for each new connection, if desired.
		// The host/port/user/password fields don't need to be provided
		// because they are overwritten by the IAM authentication system.
		GetMysqlConfigCallback: func(ctx context.Context) (*mysql.Config, stackerr.Error) {
			return mysqlConfig, nil
		},
	}

	// The maximum number of connections we can have open to the
	// read host.
	readMaxOpenConnections := 3
	readDialectorInput := dialectors.MysqlDialectorInput{
		DialectorInput: dialectors.DialectorInput{
			// Some general GORM settings for the connection management
			MaxOpenConns: &readMaxOpenConnections,
			// ...several other settings available
		},

		// Set the GORM-specific MySQL settings to use for this dialector
		GormMysqlConfig: gormMysqlConfig,

		// Set a function that returns the MySQL config to use. This
		// allows changing parameters for each new connection, if desired.
		// The host/port/user/password fields don't need to be provided
		// because they are overwritten by the IAM authentication system.
		GetMysqlConfigCallback: func(ctx context.Context) (*mysql.Config, stackerr.Error) {
			return mysqlConfig, nil
		},
	}

	// Create an input for the creation function
	input := gormauthmysql.GetMysqlGormInputWithAuth[gormauthawsiam.RdsIamAuthWithReadOnly]{
		GetMysqlGormInput: gormauthmysql.GetMysqlGormInput{
			GormOptions: []gorm.Option{
				gormConfig,
			},
			WriteDialectorInput: writeDialectorInput,
			ReadDialectorInput:  readDialectorInput,
			// Since we're doing IAM auth, we must be connecting to
			// an RDS cluster. RDS clusters use AWS's root CAs for signing
			// the TLS certificates, so use the helper function that
			// gets a TLS config that trusts AWS's root CAs.
			GetTlsConfigFunc: gormauthaws.GetTlsConfig,
		},
		AuthSettings: iamAuthSettings,
	}

	// Get the GORM DB
	return gormauthaws.GetRdsIamMysqlGorm(ctx, input)
}
