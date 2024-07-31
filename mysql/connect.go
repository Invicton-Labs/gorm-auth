package gormauthmysql

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"

	"github.com/Invicton-Labs/go-stackerr"
	gormawsiam "github.com/Invicton-Labs/gorm-auth/aws/iam-auth"
	"github.com/Invicton-Labs/gorm-auth/connectors"
	"github.com/Invicton-Labs/gorm-auth/dialectors"
	gormauthmodels "github.com/Invicton-Labs/gorm-auth/models"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// A function signature for a callback function that gets the TLS configuration
// to use for a specific host.
type GetTlsConfigCallback func(ctx context.Context, host string) (*tls.Config, stackerr.Error)

func wrapMysqlConfigWithTls(sourceFunc connectors.GetMysqlConfigCallback, getTlsFunc GetTlsConfigCallback) connectors.GetMysqlConfigCallback {
	return func(ctx context.Context) (*mysql.Config, stackerr.Error) {
		// Get the base config
		mysqlConfig, err := sourceFunc(ctx)
		if err != nil {
			return nil, err
		}

		// Make a copy of it so we don't mess with any values
		// in the original value, since we got a pointer
		mysqlConfig = mysqlConfig.Clone()

		// Parse the address
		u, cerr := url.Parse(mysqlConfig.Addr)
		if cerr != nil {
			return nil, stackerr.Wrap(cerr)
		}

		if u.Host == "" {
			var err error
			u, err = url.Parse(fmt.Sprintf("db://%s", mysqlConfig.Addr))
			if err != nil {
				return nil, stackerr.Wrap(err)
			}
			u.Scheme = ""
		}

		if u.Host == "" {
			return nil, stackerr.Errorf("failed to parse host out of MySQL address '%s'", mysqlConfig.Addr)
		}

		hostWithoutPort := strings.SplitN(u.Host, ":", 2)[0]

		// Get the TLS config
		tlsConfig, err := getTlsFunc(ctx, hostWithoutPort)
		if err != nil {
			return nil, err
		}

		// Register the TLS config for this host
		if err := mysql.RegisterTLSConfig(u.Host, tlsConfig); err != nil {
			return nil, stackerr.Wrap(err)
		}

		// Ensure the MySQL config specifies the right TLS config
		mysqlConfig.TLSConfig = u.Host
		return mysqlConfig, nil
	}
}

type passwordAuthTypes interface {
	gormauthmodels.MysqlSecretPassword | gormauthmodels.MysqlSecretPasswordWithReadOnly
}
type MysqlAuthTypes interface {
	gormawsiam.RdsIamAuth | gormawsiam.RdsIamAuthWithReadOnly | passwordAuthTypes
}

// The input values for getting a standard MySQL GORM DB handle
type GetMysqlGormInput struct {
	// REQUIRED: Input values for the write dialector
	WriteDialectorInput dialectors.MysqlDialectorInput
	// OPTIONAL: Input values for the read dialector
	ReadDialectorInput dialectors.MysqlDialectorInput
	// OPTIONAL: A set of GORM options to use for the connections
	GormOptions []gorm.Option
	// OPTIONAL: A function that gets the TLS config to use for a
	// connection based on the given host name
	GetTlsConfigFunc GetTlsConfigCallback
}

type GetMysqlGormInputWithAuth[authType MysqlAuthTypes] struct {
	GetMysqlGormInput
	AuthSettings authType
}

func GetMysqlGormPassword[authType passwordAuthTypes](
	ctx context.Context,
	input GetMysqlGormInputWithAuth[authType],
) (*gorm.DB, stackerr.Error) {

	var writeAuthSettings gormauthmodels.MysqlSecretPassword
	var readAuthSettings gormauthmodels.MysqlSecretPasswordWithReadOnly
	hasReader := false

	if authSettings, ok := any(input.AuthSettings).(gormauthmodels.MysqlSecretPassword); ok {
		writeAuthSettings = authSettings
	} else if authSettings, ok := any(input.AuthSettings).(gormauthmodels.MysqlSecretPasswordWithReadOnly); ok {
		writeAuthSettings = authSettings.MysqlSecretPassword
		readAuthSettings = authSettings
		hasReader = true
	}

	if input.WriteDialectorInput.GetMysqlConfigCallback != nil {
		// Wrap the write dialector with a function that adds
		// the appropriate connection parameters.
		input.WriteDialectorInput.GetMysqlConfigCallback = func(ctx context.Context) (*mysql.Config, stackerr.Error) {
			cfg, err := input.WriteDialectorInput.GetMysqlConfigCallback(ctx)
			if err != nil {
				return nil, err
			}
			creds, err := writeAuthSettings.GetCredentials(ctx)
			if err != nil {
				return nil, err
			}
			cfg.User = creds.Username
			cfg.Passwd = creds.Password
			cfg.Addr = fmt.Sprintf("%s:%d", writeAuthSettings.Host, writeAuthSettings.Port)
			cfg.DBName = writeAuthSettings.Schema
			return cfg, nil
		}
	}
	if input.ReadDialectorInput.GetMysqlConfigCallback != nil && hasReader {
		port := readAuthSettings.PortReadOnly
		if port == 0 {
			port = readAuthSettings.Port
		}
		// Wrap the read dialector with a function that adds
		// the appropriate connection parameters.
		input.ReadDialectorInput.GetMysqlConfigCallback = func(ctx context.Context) (*mysql.Config, stackerr.Error) {
			cfg, err := input.ReadDialectorInput.GetMysqlConfigCallback(ctx)
			if err != nil {
				return nil, err
			}
			var creds gormauthmodels.DatabaseCredentials
			if readAuthSettings.GetCredentialsReadOnly != nil {
				creds, err = readAuthSettings.GetCredentialsReadOnly(ctx)
			} else {
				creds, err = readAuthSettings.GetCredentials(ctx)
			}
			if err != nil {
				return nil, err
			}
			cfg.User = creds.Username
			cfg.Passwd = creds.Password
			cfg.Addr = fmt.Sprintf("%s:%d", readAuthSettings.HostReadOnly, port)
			cfg.DBName = readAuthSettings.Schema
			return cfg, nil
		}
	}

	// If the input type doesn't include a read-only connection,
	// set the config callback as nil so read-only connections
	// are never used.
	if !hasReader {
		input.ReadDialectorInput.GetMysqlConfigCallback = nil
	}

	return GetMysqlGorm(ctx, input.GetMysqlGormInput)
}

// Gets a GORM DB handle for a MySQL database
func GetMysqlGorm(
	ctx context.Context,
	input GetMysqlGormInput,
) (*gorm.DB, stackerr.Error) {

	if input.WriteDialectorInput.GetMysqlConfigCallback == nil {
		panic("the `input.WriteDialectorInput.GetMysqlConfigCallback` value must not be nil")
	}

	// If a TLS function is provided, wrap the config functions
	if input.GetTlsConfigFunc != nil {
		input.WriteDialectorInput.GetMysqlConfigCallback = wrapMysqlConfigWithTls(input.WriteDialectorInput.GetMysqlConfigCallback, input.GetTlsConfigFunc)
		if input.ReadDialectorInput.GetMysqlConfigCallback != nil {
			input.ReadDialectorInput.GetMysqlConfigCallback = wrapMysqlConfigWithTls(input.ReadDialectorInput.GetMysqlConfigCallback, input.GetTlsConfigFunc)
		}
	}

	// Create the writer dialector
	writerDialector := dialectors.NewDialector(input.WriteDialectorInput)

	db, err := gorm.Open(writerDialector, input.GormOptions...)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}

	if input.ReadDialectorInput.GetMysqlConfigCallback != nil {
		// Create the reader dialector
		readDialector := dialectors.NewDialector(input.ReadDialectorInput)

		// Register the reader dialector, if there is one
		if err := db.Use(dbresolver.Register(dbresolver.Config{
			Replicas: []gorm.Dialector{
				readDialector,
			},
			// The policy doesn't do anything, since there's only
			// one writer dialector and one reader dialector.
			Policy: dbresolver.RandomPolicy{},
		})); err != nil {
			return nil, stackerr.Wrap(err)
		}
	}

	return db, nil
}
