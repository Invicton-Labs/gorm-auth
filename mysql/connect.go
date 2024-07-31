package gormauthmysql

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"

	"github.com/Invicton-Labs/go-stackerr"
	"github.com/Invicton-Labs/gorm-auth/connectors"
	"github.com/Invicton-Labs/gorm-auth/dialectors"
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

type AuthenticationSettings interface {
	GetAuthParameters(ctx context.Context) (*mysql.Config, stackerr.Error)
}

type ConnectionParameters struct {
	DialectorInput dialectors.MysqlDialectorInput
	// OPTIONAL: A function that gets the TLS config to use for a
	// connection based on the given host name
	GetTlsConfigFunc GetTlsConfigCallback
	AuthSettings     AuthenticationSettings
}

// The input values for getting a standard MySQL GORM DB handle
type GetMysqlGormInput struct {
	// Input values for write connections
	WriteConnectionParameters *ConnectionParameters
	// Input values for read connections
	ReadConnectionParameters []*ConnectionParameters
	// OPTIONAL: A set of GORM options to use for all connections
	GormOptions []gorm.Option
	// OPTIONAL: The policy to use for connecting to read replicas.
	// If not provided, the Random policy will be used.
	ReplicaPolicy dbresolver.Policy
}

func wrapConfigCallback(callback connectors.GetMysqlConfigCallback, authSettings AuthenticationSettings, getTlsConfigFunc GetTlsConfigCallback) connectors.GetMysqlConfigCallback {
	f := func(ctx context.Context) (*mysql.Config, stackerr.Error) {
		var config *mysql.Config
		if callback != nil {
			var err stackerr.Error
			config, err = callback(ctx)
			if err != nil {
				return nil, err
			}
		} else {
			config = mysql.NewConfig()
		}

		// Get the authentication parameters
		authConfig, err := authSettings.GetAuthParameters(ctx)
		if err != nil {
			return nil, err
		}

		config.Addr = authConfig.Addr
		config.User = authConfig.User
		config.Passwd = authConfig.Passwd
		config.DBName = authConfig.DBName

		// TODO: also need to set other config params that are non-default

		return config, nil
	}

	if getTlsConfigFunc != nil {
		f = wrapMysqlConfigWithTls(f, getTlsConfigFunc)
	}

	return f
}

func GetMysqlGorm(
	ctx context.Context,
	input GetMysqlGormInput,
) (*gorm.DB, stackerr.Error) {
	var writerDialector gorm.Dialector
	readerDialectors := make([]gorm.Dialector, len(input.ReadConnectionParameters))
	replicaDialectors := []gorm.Dialector{}
	if input.WriteConnectionParameters != nil {
		// Wrap the config callback to apply the authentication parameters and TLS config
		input.WriteConnectionParameters.DialectorInput.GetMysqlConfigCallback = wrapConfigCallback(input.WriteConnectionParameters.DialectorInput.GetMysqlConfigCallback, input.WriteConnectionParameters.AuthSettings, input.WriteConnectionParameters.GetTlsConfigFunc)
		writerDialector = dialectors.NewDialector(input.WriteConnectionParameters.DialectorInput)
	}
	if len(input.ReadConnectionParameters) > 0 {
		for idx, _ := range input.ReadConnectionParameters {
			// Wrap the config callback to apply the authentication parameters and TLS config
			input.ReadConnectionParameters[idx].DialectorInput.GetMysqlConfigCallback = wrapConfigCallback(input.ReadConnectionParameters[idx].DialectorInput.GetMysqlConfigCallback, input.ReadConnectionParameters[idx].AuthSettings, input.ReadConnectionParameters[idx].GetTlsConfigFunc)
			readerDialectors[idx] = dialectors.NewDialector(input.ReadConnectionParameters[idx].DialectorInput)
		}
	}

	var db *gorm.DB
	var cerr error
	if writerDialector != nil {
		// If there's a writer dialector, use that as main
		db, cerr = gorm.Open(writerDialector, input.GormOptions...)
	} else if len(readerDialectors) > 0 {
		// Otherwise, use the first reader as main
		db, cerr = gorm.Open(readerDialectors[0], input.GormOptions...)
		// And use the remaining readers as replicas
		replicaDialectors = readerDialectors[1:]
	} else {
		return nil, stackerr.Errorf("At least one of a writer or reader input must be provided")
	}
	if cerr != nil {
		return nil, stackerr.Wrap(cerr)
	}

	// If there are any extra read replicas, add them
	if len(replicaDialectors) > 0 {
		policy := input.ReplicaPolicy
		if policy == nil {
			policy = dbresolver.RandomPolicy{}
		}
		// Register the replica reader dialectors
		if err := db.Use(dbresolver.Register(dbresolver.Config{
			Replicas: replicaDialectors,
			Policy:   policy,
		})); err != nil {
			return nil, stackerr.Wrap(err)
		}
	}

	return db, nil
}

// func GetMysqlGormPassword[authType passwordAuthTypes](
// 	ctx context.Context,
// 	input GetMysqlGormInputWithAuth[authType],
// ) (*gorm.DB, stackerr.Error) {

// 	var writeAuthSettings gormauthmodels.MysqlSecretPassword
// 	var readAuthSettings gormauthmodels.MysqlSecretPasswordWithReadOnly
// 	hasReader := false

// 	if authSettings, ok := any(input.AuthSettings).(gormauthmodels.MysqlSecretPassword); ok {
// 		writeAuthSettings = authSettings
// 	} else if authSettings, ok := any(input.AuthSettings).(gormauthmodels.MysqlSecretPasswordWithReadOnly); ok {
// 		writeAuthSettings = authSettings.MysqlSecretPassword
// 		readAuthSettings = authSettings
// 		hasReader = true
// 	}

// 	if input.WriteDialectorInput.GetMysqlConfigCallback != nil {
// 		// Wrap the write dialector with a function that adds
// 		// the appropriate connection parameters.
// 		input.WriteDialectorInput.GetMysqlConfigCallback = func(ctx context.Context) (*mysql.Config, stackerr.Error) {
// 			cfg, err := input.WriteDialectorInput.GetMysqlConfigCallback(ctx)
// 			if err != nil {
// 				return nil, err
// 			}
// 			creds, err := writeAuthSettings.GetCredentials(ctx)
// 			if err != nil {
// 				return nil, err
// 			}
// 			cfg.User = creds.Username
// 			cfg.Passwd = creds.Password
// 			cfg.Addr = fmt.Sprintf("%s:%d", writeAuthSettings.Host, writeAuthSettings.Port)
// 			cfg.DBName = writeAuthSettings.Schema
// 			return cfg, nil
// 		}
// 	}
// 	if input.ReadDialectorInput.GetMysqlConfigCallback != nil && hasReader {
// 		port := readAuthSettings.PortReadOnly
// 		if port == 0 {
// 			port = readAuthSettings.Port
// 		}
// 		// Wrap the read dialector with a function that adds
// 		// the appropriate connection parameters.
// 		input.ReadDialectorInput.GetMysqlConfigCallback = func(ctx context.Context) (*mysql.Config, stackerr.Error) {
// 			cfg, err := input.ReadDialectorInput.GetMysqlConfigCallback(ctx)
// 			if err != nil {
// 				return nil, err
// 			}
// 			var creds gormauthmodels.DatabaseCredentials
// 			if readAuthSettings.GetCredentialsReadOnly != nil {
// 				creds, err = readAuthSettings.GetCredentialsReadOnly(ctx)
// 			} else {
// 				creds, err = readAuthSettings.GetCredentials(ctx)
// 			}
// 			if err != nil {
// 				return nil, err
// 			}
// 			cfg.User = creds.Username
// 			cfg.Passwd = creds.Password
// 			cfg.Addr = fmt.Sprintf("%s:%d", readAuthSettings.HostReadOnly, port)
// 			cfg.DBName = readAuthSettings.Schema
// 			return cfg, nil
// 		}
// 	}

// 	// If the input type doesn't include a read-only connection,
// 	// set the config callback as nil so read-only connections
// 	// are never used.
// 	if !hasReader {
// 		input.ReadDialectorInput.GetMysqlConfigCallback = nil
// 	}

// 	return GetMysqlGorm(ctx, input.GetMysqlGormInput)
// }
