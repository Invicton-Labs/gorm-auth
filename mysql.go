package gormauth

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"

	"github.com/Invicton-Labs/go-stackerr"
	"github.com/Invicton-Labs/gorm-auth/authenticators"
	"github.com/Invicton-Labs/gorm-auth/connectors"
	"github.com/Invicton-Labs/gorm-auth/dialectors"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

var defaultMysqlConfig mysql.Config = *(mysql.NewConfig())

// A function signature for a callback function that gets the TLS configuration
// to use for a specific host.
type GetTlsConfigCallback func(ctx context.Context, host string) (*tls.Config, stackerr.Error)

type ConnectionParameters struct {
	DialectorInput dialectors.MysqlDialectorInput
	// OPTIONAL: A function that gets the TLS config to use for a
	// connection based on the given host name
	GetTlsConfigFunc GetTlsConfigCallback
	AuthSettings     authenticators.AuthenticationSettings
}

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

// The input values for getting a standard MySQL GORM DB handle
type GetMysqlGormInput struct {
	// Input values for write connections
	WriteConnectionParameters []*ConnectionParameters
	// Input values for read connections
	ReadConnectionParameters []*ConnectionParameters
	// OPTIONAL: A set of GORM options to use for all connections
	GormOptions []gorm.Option
	// OPTIONAL: The policy to use for connecting to read replicas.
	// If not provided, the Random policy will be used.
	ReplicaPolicy dbresolver.Policy
}

func wrapConfigCallback(callback connectors.GetMysqlConfigCallback, authSettings authenticators.AuthenticationSettings, getTlsConfigFunc GetTlsConfigCallback) connectors.GetMysqlConfigCallback {
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
		config, err := authSettings.UpdateConfigWithAuth(ctx, *config)
		if err != nil {
			return nil, err
		}

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
	writerDialectors := make([]gorm.Dialector, len(input.WriteConnectionParameters))
	if len(input.WriteConnectionParameters) > 0 {
		for idx := range input.WriteConnectionParameters {
			// If the authenticator also needs to make changes to the dialector input, make those changes
			var err stackerr.Error
			input.WriteConnectionParameters[idx].DialectorInput, err = input.WriteConnectionParameters[idx].AuthSettings.UpdateDialectorSettings(input.WriteConnectionParameters[idx].DialectorInput)
			if err != nil {
				return nil, err
			}
			// Wrap the config callback to apply the authentication parameters and TLS config
			input.WriteConnectionParameters[idx].DialectorInput.GetMysqlConfigCallback = wrapConfigCallback(input.WriteConnectionParameters[idx].DialectorInput.GetMysqlConfigCallback, input.WriteConnectionParameters[idx].AuthSettings, input.WriteConnectionParameters[idx].GetTlsConfigFunc)
			writerDialectors[idx] = dialectors.NewDialector(input.WriteConnectionParameters[idx].DialectorInput)
		}
	}

	readerDialectors := make([]gorm.Dialector, len(input.ReadConnectionParameters))
	if len(input.ReadConnectionParameters) > 0 {
		for idx := range input.ReadConnectionParameters {
			// If the authenticator also needs to make changes to the dialector input, make those changes
			var err stackerr.Error
			input.ReadConnectionParameters[idx].DialectorInput, err = input.ReadConnectionParameters[idx].AuthSettings.UpdateDialectorSettings(input.ReadConnectionParameters[idx].DialectorInput)
			if err != nil {
				return nil, err
			}
			// Wrap the config callback to apply the authentication parameters and TLS config
			input.ReadConnectionParameters[idx].DialectorInput.GetMysqlConfigCallback = wrapConfigCallback(input.ReadConnectionParameters[idx].DialectorInput.GetMysqlConfigCallback, input.ReadConnectionParameters[idx].AuthSettings, input.ReadConnectionParameters[idx].GetTlsConfigFunc)
			readerDialectors[idx] = dialectors.NewDialector(input.ReadConnectionParameters[idx].DialectorInput)
		}
	}

	// We need to select a primary dialector for things
	var mainConnection gorm.Dialector
	if len(writerDialectors) > 0 {
		mainConnection = writerDialectors[0]
		// TODO: it's unclear if the default should still be included in the Sources list (https://github.com/go-gorm/gorm/issues/7145)
		//writerDialectors = writerDialectors[1:]
	} else if len(readerDialectors) > 0 {
		mainConnection = readerDialectors[0]
		// TODO: it's unclear if the default should still be included in the Replica list (https://github.com/go-gorm/gorm/issues/7145)
		//readerDialectors = readerDialectors[1:]
	}

	// Create the database without a dialector, so
	// no connection is opened automatically. We do this
	// because we don't want a write connection to be opened
	// if we end up only needing a read connection, and
	// vice-versa.
	db, cerr := gorm.Open(mainConnection, input.GormOptions...)
	if cerr != nil {
		return nil, stackerr.Wrap(cerr)
	}

	// If there are multiple dialectors, we need a DBResolver.
	// If not, we can just use the default dialector for everything.
	if len(writerDialectors)+len(readerDialectors) > 1 {
		policy := input.ReplicaPolicy
		if policy == nil {
			policy = dbresolver.StrictRoundRobinPolicy()
		}
		// Register the dialectors
		if err := db.Use(dbresolver.Register(dbresolver.Config{
			Sources:  writerDialectors,
			Replicas: readerDialectors,
			Policy:   policy,
		})); err != nil {
			return nil, stackerr.Wrap(err)
		}
	}

	return db, nil
}
