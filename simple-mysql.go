package gormauth

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// A function signature for a callback function that gets the TLS configuration
// to use for a specific host.
type GetTlsConfigCallback func(ctx context.Context, host string) (*tls.Config, error)

func wrapMysqlConfigWithTls(sourceFunc GetMysqlConfigCallback, getTlsFunc GetTlsConfigCallback) GetMysqlConfigCallback {
	return func(ctx context.Context) (*mysql.Config, error) {
		// Get the base config
		mysqlConfig, err := sourceFunc(ctx)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		// Make a copy of it so we don't mess with any values
		// in the original value, since we got a pointer
		mysqlConfig = mysqlConfig.Clone()

		// Parse the address
		u, err := url.Parse(mysqlConfig.Addr)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if u.Host == "" {
			u, err = url.Parse(fmt.Sprintf("db://%s", mysqlConfig.Addr))
			if err != nil {
				return nil, errors.WithStack(err)
			}
			u.Scheme = ""
		}

		if u.Host == "" {
			return nil, errors.Errorf("failed to parse host out of MySQL address '%s'", mysqlConfig.Addr)
		}

		hostWithoutPort := strings.SplitN(u.Host, ":", 2)[0]

		// Get the TLS config
		tlsConfig, err := getTlsFunc(ctx, hostWithoutPort)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		// Register the TLS config for this host
		if err := mysql.RegisterTLSConfig(u.Host, tlsConfig); err != nil {
			return nil, errors.WithStack(err)
		}

		// Ensure the MySQL config specifies the right TLS config
		mysqlConfig.TLSConfig = u.Host
		return mysqlConfig, nil
	}
}

type GetMysqlGormInputBase struct {
	GormOptions         []gorm.Option
	GormMysqlConfig     gormmysql.Config
	WriteDialectorInput DialectorInput
	ReadDialectorInput  DialectorInput
	GetTlsConfigFunc    GetTlsConfigCallback
}

type GetMysqlGormInput struct {
	GetMysqlGormInputBase
	WriteConfigFunc GetMysqlConfigCallback
	ReadConfigFunc  GetMysqlConfigCallback
}

func GetMysqlGorm(
	ctx context.Context,
	input GetMysqlGormInput,
) (*gorm.DB, error) {

	if input.WriteConfigFunc == nil {
		panic("the `input.WriterConfigFunc` value must not be nil")
	}

	writeDialectorInput := input.WriteDialectorInput.Clone()
	readDialectorInput := input.ReadDialectorInput.Clone()

	// If a TLS function is provided, wrap the config functions
	if input.GetTlsConfigFunc != nil {
		input.WriteConfigFunc = wrapMysqlConfigWithTls(input.WriteConfigFunc, input.GetTlsConfigFunc)
		if input.ReadConfigFunc != nil {
			input.ReadConfigFunc = wrapMysqlConfigWithTls(input.ReadConfigFunc, input.GetTlsConfigFunc)
		}
	}

	// Create the writer dialector
	writerDialector := NewDialector(MysqlDialectorInput{
		DialectorInput:         writeDialectorInput,
		GormMysqlConfig:        input.GormMysqlConfig,
		GetMysqlConfigCallback: input.WriteConfigFunc,
	})

	db, err := gorm.Open(writerDialector, input.GormOptions...)
	if err != nil {
		return nil, err
	}

	if input.ReadConfigFunc != nil {
		// Create the reader dialector
		readDialector := NewDialector(MysqlDialectorInput{
			DialectorInput:         readDialectorInput,
			GormMysqlConfig:        input.GormMysqlConfig,
			GetMysqlConfigCallback: input.ReadConfigFunc,
		})

		// Register the reader dialector, if there is one
		if err := db.Use(dbresolver.Register(dbresolver.Config{
			Replicas: []gorm.Dialector{
				readDialector,
			},
			// The policy doesn't do anything, since there's only
			// one writer dialector and one reader dialector.
			Policy: dbresolver.RandomPolicy{},
		})); err != nil {
			return nil, err
		}
	}

	return db, nil
}
