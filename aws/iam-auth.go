package gormaws

import (
	"context"
	"fmt"
	"regexp"

	"github.com/Invicton-Labs/gorm-auth/connectors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

var (
	rdsHostRegionRegexp *regexp.Regexp = regexp.MustCompile(`^[^.]+\.[^.]+\.([a-z]+-[a-z]+-[0-9]+)\.rds\.amazonaws\.com$`)
)

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
	// that we're connecting from. If this field is not
	// provide, the connection function will attempt to
	// parse the region from the RDS host name.
	Region string `json:"region"`
	// The AWS config to use for authentication/credentials
	AwsConfig aws.Config
}

func (ria *RdsIamAuth) getTokenGenerator(baseCfg *mysql.Config, host string, port int, username string) connectors.GetMysqlConfigCallback {

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
		if len(regionMatches) > 1 {
			dbRegion = regionMatches[1]
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

		cfg.User = username
		cfg.Passwd = authenticationToken
		cfg.Addr = fmt.Sprintf("%s:%d", host, port)
		cfg.DBName = ria.Database

		// IAM requires clear text authentication
		cfg.AllowCleartextPasswords = true
		// IAM requires native password authentication
		cfg.AllowNativePasswords = true

		return cfg, nil
	}

}

// GetReadOnlyTokenGenerator returns a generator function that generates RDS IAM auth tokens
// for use in new connections to the main/writer host specified in an RdsIamAuth struct.
func (ria *RdsIamAuth) GetTokenGenerator(baseCfg *mysql.Config) connectors.GetMysqlConfigCallback {
	return ria.getTokenGenerator(baseCfg, ria.Host, ria.Port, ria.Username)
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
func (ria *RdsIamAuthWithReadOnly) GetReadOnlyTokenGenerator(baseCfg *mysql.Config) connectors.GetMysqlConfigCallback {
	port := ria.PortReadOnly
	if port == 0 {
		port = ria.Port
	}
	username := ria.UsernameReadOnly
	if username == "" {
		username = ria.Username
	}
	return ria.getTokenGenerator(baseCfg, ria.HostReadOnly, port, username)
}
