package examples

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Invicton-Labs/go-stackerr"
	gormauth "github.com/Invicton-Labs/gorm-auth"
	gormaws "github.com/Invicton-Labs/gorm-auth/aws"
	"github.com/Invicton-Labs/gorm-auth/dialectors"
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
	var iamAuthSettings gormaws.RdsIamAuthWithReadOnly
	if err := json.Unmarshal([]byte(AwsRdsIamAuthJson), &iamAuthSettings); err != nil {
		return nil, stackerr.Wrap(err)
	}

	// Load the default AWS config from the environment variables.
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	iamAuthSettings.AwsConfig = awsCfg

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

		// We don't need to set the GetMysqlConfigCallback value because
		// a custom IAM auth function will be added automatically
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

		// We don't need to set the GetMysqlConfigCallback value because
		// a custom IAM auth function will be added automatically
	}

	// Create an input for the creation function
	input := gormaws.GetRdsIamMysqlGormInput[gormaws.RdsIamAuthWithReadOnly]{
		GetMysqlGormInput: gormauth.GetMysqlGormInput{
			GormOptions: []gorm.Option{
				gormConfig,
			},
			WriteDialectorInput: writeDialectorInput,
			ReadDialectorInput:  readDialectorInput,
			// Since we're doing IAM auth, we must be connecting to
			// an RDS cluster. RDS clusters use AWS's root CAs for signing
			// the TLS certificates, so use the helper function that
			// gets a TLS config that trusts AWS's root CAs.
			GetTlsConfigFunc: gormaws.GetTlsConfig,
		},
		MysqlConfig:  mysqlConfig,
		AuthSettings: iamAuthSettings,
	}

	// Get the GORM DB
	return gormaws.GetRdsIamMysqlGorm(ctx, input)
}
