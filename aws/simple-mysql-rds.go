package gormaws

import (
	"context"

	"github.com/Invicton-Labs/go-stackerr"
	gormauth "github.com/Invicton-Labs/gorm-auth"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type authTypes interface {
	RdsIamAuth | RdsIamAuthWithReadOnly
}

// GetRdsIamMysqlGormInput is an input that contains everything
// needed for a standard connection to an AWS RDS cluster with
// IAM authentication enabled.
type GetRdsIamMysqlGormInput[AuthType authTypes] struct {
	gormauth.GetMysqlGormInput
	MysqlConfig  *mysql.Config
	AuthSettings AuthType
}

// GetRdsIamMysqlGorm gets a GORM DB using IAM authentication for
// an RDS cluster. It automatically sets the TLS configuration for
// RDS by loading the root certificates from AWS via HTTP.
func GetRdsIamMysqlGorm[AuthType authTypes](
	ctx context.Context,
	input GetRdsIamMysqlGormInput[AuthType],
) (*gorm.DB, stackerr.Error) {

	var writeAuthSettings RdsIamAuth
	var readAuthSettings RdsIamAuthWithReadOnly
	hasReader := false
	if authSettings, ok := any(input.AuthSettings).(RdsIamAuth); ok {
		writeAuthSettings = authSettings
	} else {
		readAuthSettings = any(input.AuthSettings).(RdsIamAuthWithReadOnly)
		writeAuthSettings = readAuthSettings.RdsIamAuth
		hasReader = true
	}

	// If no credential source is provided, use the default AWS config
	// from environment variables.
	if writeAuthSettings.AwsConfig.Credentials == nil && writeAuthSettings.AwsConfig.Region == "" {
		defaultAwsConfig, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		writeAuthSettings.AwsConfig = defaultAwsConfig
		readAuthSettings.AwsConfig = defaultAwsConfig
	}

	// RDS IAM Auth rotates tokens very frequently, so just get a new token
	// on each new connection.
	input.WriteDialectorInput.ShouldReconfigureCallback = nil
	input.ReadDialectorInput.ShouldReconfigureCallback = nil

	// If no TLS config func is provided, use the default AWS TLS
	// config func.
	if input.GetTlsConfigFunc == nil {
		input.GetTlsConfigFunc = GetTlsConfig
	}

	input.WriteDialectorInput.GetMysqlConfigCallback = writeAuthSettings.GetTokenGenerator(input.MysqlConfig)

	if hasReader {
		input.ReadDialectorInput.GetMysqlConfigCallback = readAuthSettings.GetReadOnlyTokenGenerator(input.MysqlConfig)
	}

	return gormauth.GetMysqlGorm(ctx, input.GetMysqlGormInput)
}
