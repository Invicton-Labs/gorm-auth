package examples

import (
	"context"
	"encoding/json"
	"time"

	gormauth "github.com/Invicton-Labs/gorm-auth"
	gormaws "github.com/Invicton-Labs/gorm-auth/aws"
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

func AwsRdsMysqlIamAuth(ctx context.Context) (*gorm.DB, error) {

	// Unmarshal the JSON into a struct that can be used for
	// generating tokens.
	var iamAuthSettings gormaws.RdsIamAuthWithReadOnly
	if err := json.Unmarshal([]byte(AwsRdsIamAuthJson), &iamAuthSettings); err != nil {
		return nil, err
	}

	// Load the default AWS config from the environment variables.
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
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
	writeDialectorInput := gormauth.DialectorInput{
		// Some general GORM settings for the connection management
		MaxOpenConns: &writeMaxOpenConnections,
		// ...several other settings available
	}

	// The maximum number of connections we can have open to the
	// read host.
	readMaxOpenConnections := 3
	readDialectorInput := gormauth.DialectorInput{
		// Some general GORM settings for the connection management
		MaxOpenConns: &readMaxOpenConnections,
		// ...several other settings available
	}

	return gormaws.GetRdsIamMysqlGorm(ctx, gormaws.GetRdsIamMysqlGormInput[gormaws.RdsIamAuthWithReadOnly]{
		GetMysqlGormInputBase: gormauth.GetMysqlGormInputBase{
			GormOptions: []gorm.Option{
				gormConfig,
			},
			GormMysqlConfig:     gormMysqlConfig,
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
	})
}
