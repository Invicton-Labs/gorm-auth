package gormauthawsiam

import (
	"context"
	"fmt"
	"regexp"

	"github.com/Invicton-Labs/go-stackerr"
	"github.com/Invicton-Labs/gorm-auth/connectors"
	gormauthmodels "github.com/Invicton-Labs/gorm-auth/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/go-sql-driver/mysql"
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
	// Include all standard MySQL secret fields
	gormauthmodels.MysqlSecret
	// The username to connect with
	Username string `json:"username"`
	// This is the region that the database is in, not
	// that we're connecting from. If this field is not
	// provide, the connection function will attempt to
	// parse the region from the RDS host name.
	Region string `json:"region"`
	// The AWS config to use for authentication/credentials
	AwsConfig aws.Config
}

func getTokenGenerator(region string, host string, port int, username string, schema string, credentials aws.CredentialsProvider, getConfig connectors.GetMysqlConfigCallback) connectors.GetMysqlConfigCallback {
	dbRegion := region
	if dbRegion == "" {
		// If no region was specified, try to extract it from the hostname
		regionMatches := rdsHostRegionRegexp.FindStringSubmatch(host)
		if len(regionMatches) > 1 {
			dbRegion = regionMatches[1]
		}
	}

	if dbRegion == "" {
		panic(fmt.Sprintf("no database region was provided, and it could not be determined from the host name (%s)", host))
	}

	return func(ctx context.Context) (*mysql.Config, stackerr.Error) {
		authenticationToken, err := auth.BuildAuthToken(
			ctx,
			fmt.Sprintf("%s:%d", host, port),
			dbRegion,
			username,
			credentials,
		)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}

		config, err := getConfig(ctx)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		var cfg *mysql.Config
		if config != nil {
			cfg = config.Clone()
		} else {
			cfg = mysql.NewConfig()
		}

		cfg.User = username
		cfg.Passwd = authenticationToken
		cfg.Addr = fmt.Sprintf("%s:%d", host, port)
		cfg.DBName = schema

		// IAM requires clear text authentication
		cfg.AllowCleartextPasswords = true
		// IAM requires native password authentication
		cfg.AllowNativePasswords = true

		return cfg, nil
	}
}

// GetReadOnlyTokenGenerator returns a generator function that generates RDS IAM auth tokens
// for use in new connections to the main/writer host specified in an RdsIamAuth struct.
func (ria *RdsIamAuth) GetTokenGenerator(getConfig connectors.GetMysqlConfigCallback) connectors.GetMysqlConfigCallback {
	return getTokenGenerator(ria.Region, ria.Host, ria.Port, ria.Username, ria.Schema, ria.AwsConfig.Credentials, getConfig)
}

// RdsIamAuthWithReadOnly is an extension of RdsIamAuth that adds fields for
// separate read-only connections. This is useful since most managed RDS
// custers have read-only endpoints that support horizontal scaling.
type RdsIamAuthWithReadOnly struct {
	RdsIamAuth
	// The host for the read-only connection (required)
	HostReadOnly string `json:"host_read_only"`
	// If this is empty, it will use the same region
	// as the write cluter.
	RegionReadOnly string `json:"region_read_only"`
	// If this is empty, it will use the same port as the
	// write cluster.
	PortReadOnly int `json:"port_read_only"`
	// If this is empty, it will use the same username
	// as the write cluter.
	UsernameReadOnly string `json:"username_read_only"`
}

// GetReadOnlyTokenGenerator returns a generator function that generates RDS IAM auth tokens
// for use in new connections to the read-only host specified in an RdsIamAuthWithReadOnly struct.
func (ria *RdsIamAuthWithReadOnly) GetReadOnlyTokenGenerator(getConfig connectors.GetMysqlConfigCallback) connectors.GetMysqlConfigCallback {
	host := ria.HostReadOnly
	if host == "" {
		host = ria.Host
	}
	port := ria.PortReadOnly
	if port == 0 {
		port = ria.Port
	}
	username := ria.UsernameReadOnly
	if username == "" {
		username = ria.Username
	}

	// Use a specified read-only region if possible
	region := ria.RegionReadOnly
	if region == "" {
		// If a read-only region isn't specified, try to get it from the host (if it matches AWS cluster format)
		regionMatches := rdsHostRegionRegexp.FindStringSubmatch(ria.HostReadOnly)
		if len(regionMatches) > 1 {
			region = regionMatches[1]
		}
	}

	// If we still don't know the region, use the write region
	if region == "" {
		region = ria.Region
	}

	return getTokenGenerator(region, host, port, username, ria.Schema, ria.AwsConfig.Credentials, getConfig)
}
