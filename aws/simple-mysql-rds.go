package gormauthaws

import (
	"context"

	"github.com/Invicton-Labs/go-stackerr"
	gormauthawsiam "github.com/Invicton-Labs/gorm-auth/aws/iam-auth"
	gormauthmysql "github.com/Invicton-Labs/gorm-auth/mysql"
	"github.com/aws/aws-sdk-go-v2/config"
	"gorm.io/gorm"
)

type authTypes interface {
	gormauthawsiam.RdsIamAuth | gormauthawsiam.RdsIamAuthWithReadOnly
}

// GetRdsIamMysqlGorm gets a GORM DB using IAM authentication for
// an RDS cluster. It automatically sets the TLS configuration for
// RDS by loading the root certificates from AWS via HTTP.
func GetRdsIamMysqlGorm[AuthType authTypes](
	ctx context.Context,
	input gormauthmysql.GetMysqlGormInputWithAuth[AuthType],
) (*gorm.DB, stackerr.Error) {

	var writeAuthSettings gormauthawsiam.RdsIamAuth
	var readAuthSettings gormauthawsiam.RdsIamAuthWithReadOnly
	hasReader := false
	if authSettings, ok := any(input.AuthSettings).(gormauthawsiam.RdsIamAuth); ok {
		writeAuthSettings = authSettings
	} else {
		readAuthSettings = any(input.AuthSettings).(gormauthawsiam.RdsIamAuthWithReadOnly)
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

	input.WriteDialectorInput.GetMysqlConfigCallback = writeAuthSettings.GetTokenGenerator(input.WriteDialectorInput.GetMysqlConfigCallback)

	if hasReader {
		input.ReadDialectorInput.GetMysqlConfigCallback = readAuthSettings.GetReadOnlyTokenGenerator(input.ReadDialectorInput.GetMysqlConfigCallback)
	} else {
		// If there's no read config, remove the reader config callback
		input.ReadDialectorInput.GetMysqlConfigCallback = nil
	}

	return gormauthmysql.GetMysqlGorm(ctx, input.GetMysqlGormInput)
}
