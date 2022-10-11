package gormaws

import (
	"context"
	"crypto/tls"
	"fmt"
	"regexp"

	gormauth "github.com/Invicton-Labs/gorm-auth"
	awscerts "github.com/Invicton-Labs/gorm-auth/aws/certs"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

var (
	rdsHostRegionRegexp *regexp.Regexp = regexp.MustCompile(`^[^.]+\.[^.]+\.([a-z]+-[a-z]+-[0-9]+)\.rds\.amazonaws\.com$`)
)

type GetConfigInput struct {
	BaseConfig *aws.Config
	RoleArn    *string
}

// func GetConfig(ctx context.Context, input GetConfigInput) (cfg aws.Config, err error) {
// 	if ctx == nil {
// 		ctx = context.Background()
// 	}

// 	if baseConfig == nil {
// 		cfg, err = config.LoadDefaultConfig(ctx)
// 		if err != nil {
// 			return aws.Config{}, errors.WithStack(err)
// 		}
// 	} else {
// 		cfg = cfg.Copy()
// 	}

// 	cfg.ConfigSources

// 	envConf, err := config.NewEnvConfig()
// 	if err != nil {
// 		return aws.Config{}, errors.WithStack(err)
// 	}

// 	if input.RoleArn != nil {
// 		envConf.RoleARN = *input.RoleArn
// 	}

// 	aws.NewConfig()

// 	stsClient := sts.NewFromConfig(cfg)
// 	provider := stscreds.NewAssumeRoleProvider(stsClient, role)
// 	credsCache := aws.NewCredentialsCache(provider)

// 	roleCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithCredentialsProvider(credsCache))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	cfg = roleCfg
// 	return cfg
// }

// RdsIamAuth is a struct that contains all of the information necessary
// for connecting to an AWS RDS cluster with IAM authentication. You
// can unmarshal JSON directly into this struct if you have a matching
// Secrets Manager secret or SSM Parameter, or you can set each field
// individually. If you unmarshal it from JSON, you must still set the
// AwsConfig field separately.
type RdsIamAuth struct {
	// The host of the primary cluster
	Host string `json:"host"`
	// The port to connect to the primary cluster
	Port int `json:"port"`
	// The username to connect with
	Username string `json:"username"`
	// The name of the database to connect to
	Database string `json:"database"`
	// This is the region that the database is in, not
	// that we're connecting from
	Region string `json:"region"`
	// The AWS config to use for authentication/credentials
	AwsConfig aws.Config
}

func (ria *RdsIamAuth) getTokenGenerator(baseCfg *mysql.Config, registerTlsConfig bool, host string, port int, username string) gormauth.GetMysqlConfigCallback {

	if host == "" {
		panic("no host was provided for connecting to the database")
	}
	if port == 0 {
		panic("no port was provided for connecting to the database")
	}
	if username == "" {
		panic("no username was provided for connecting to the database")
	}

	dbRegion := ria.Region
	if dbRegion == "" {
		regionMatches := rdsHostRegionRegexp.FindStringSubmatch(host)
		if len(regionMatches) > 0 {
			dbRegion = regionMatches[0]
		}
	}

	if dbRegion == "" {
		panic(fmt.Sprintf("no database region was provided, and it could not be determined from the host name (%s)", host))
	}

	var cfg *mysql.Config
	if baseCfg != nil {
		cfg = baseCfg.Clone()
	} else {
		cfg = mysql.NewConfig()
	}

	credentials := ria.AwsConfig.Credentials

	return func(ctx context.Context) (*mysql.Config, error) {
		authenticationToken, err := auth.BuildAuthToken(
			ctx,
			fmt.Sprintf("%s:%d", host, port),
			dbRegion,
			username,
			credentials,
		)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if err != nil {
			return nil, errors.WithStack(err)
		}

		cfg.User = username
		cfg.Passwd = authenticationToken
		cfg.Addr = fmt.Sprintf("%s:%d", host, port)
		cfg.DBName = ria.Database

		if registerTlsConfig {
			certPool, err := awscerts.GetGlobalRootCertPool(nil)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			mysql.RegisterTLSConfig(host, &tls.Config{
				RootCAs:    certPool,
				ServerName: host,
			})
		}

		// TODO: test configurations of AllowCleartextPasswords and AllowNativePasswords

		return cfg, nil
	}

}

// GetReadOnlyTokenGenerator returns a generator function that generates RDS IAM auth tokens
// for use in new connections to the main/writer host specified in an RdsIamAuth struct.
func (ria *RdsIamAuth) GetTokenGenerator(baseCfg *mysql.Config, registerTlsConfig bool) gormauth.GetMysqlConfigCallback {
	return ria.getTokenGenerator(baseCfg, registerTlsConfig, ria.Host, ria.Port, ria.Username)
}

// RdsIamAuthWithReadOnly is an extension of RdsIamAuth that adds fields for
// separate read-only connections. This is useful since most managed RDS
// custers have read-only endpoints that support horizontal scaling.
type RdsIamAuthWithReadOnly struct {
	RdsIamAuth
	HostReadOnly string `json:"host_read_only"`
	// If this is empty, it will use the same port as the
	// write cluster.
	PortReadOnly int `json:"port_read_only"`
	// If this is empty, it will use the same username
	// as the write cluter.
	UsernameReadOnly string `json:"username_read_only"`
}

// GetReadOnlyTokenGenerator returns a generator function that generates RDS IAM auth tokens
// for use in new connections to the read-only host specified in an RdsIamAuthWithReadOnly struct.
func (ria *RdsIamAuthWithReadOnly) GetReadOnlyTokenGenerator(baseCfg *mysql.Config, registerTlsConfig bool) gormauth.GetMysqlConfigCallback {
	port := ria.PortReadOnly
	if port == 0 {
		port = ria.Port
	}
	username := ria.UsernameReadOnly
	if username == "" {
		username = ria.Username
	}
	return ria.getTokenGenerator(baseCfg, registerTlsConfig, ria.HostReadOnly, port, username)
}
