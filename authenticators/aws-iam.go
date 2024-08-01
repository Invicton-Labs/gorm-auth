package authenticators

import (
	"context"
	"fmt"
	"regexp"

	"github.com/Invicton-Labs/go-stackerr"
	"github.com/Invicton-Labs/gorm-auth/dialectors"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/go-sql-driver/mysql"
)

var (
	rdsHostRegionRegexp *regexp.Regexp = regexp.MustCompile(`^[^.]+\.[^.]+\.([a-z]+-[a-z]+-[0-9]+)\.rds\.amazonaws\.com$`)
)

type MysqlConnectionParametersAwsIam struct {
	// The host of the primary cluster
	Host string `json:"host"`
	// The port to connect to the primary cluster
	Port int `json:"port"`
	// The name of the database to connect to
	Schema string `json:"database"`
	// The username to connect with
	Username string `json:"username"`
	// This is the region that the database is in, not
	// that we're connecting from. If this field is not
	// provideed, the connection function will attempt to
	// parse the region from the host name.
	Region string `json:"region"`
	// The AWS config to use for authentication/credentials
	AwsCredentials aws.CredentialsProvider
}

func (params *MysqlConnectionParametersAwsIam) UpdateDialectorSettings(dialectorInput dialectors.MysqlDialectorInput) (dialectors.MysqlDialectorInput, stackerr.Error) {
	// IAM auth rotates tokens frequently, so a new token should be used each time
	dialectorInput.ShouldReconfigureCallback = nil
	return dialectorInput, nil
}

func (params *MysqlConnectionParametersAwsIam) UpdateConfigWithAuth(ctx context.Context, config mysql.Config) (*mysql.Config, stackerr.Error) {
	if params.Region == "" {
		// If no region was specified, try to extract it from the hostname
		regionMatches := rdsHostRegionRegexp.FindStringSubmatch(params.Host)
		if len(regionMatches) > 1 {
			params.Region = regionMatches[1]
		}
	}
	if params.Region == "" {
		return &config, stackerr.Errorf("no database region was provided, and it could not be determined from the host name (%s)", params.Host)
	}

	// If no credential source is provided, use the default AWS config
	// from environment variables.
	if params.AwsCredentials == nil {
		defaultAwsConfig, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		params.AwsCredentials = defaultAwsConfig.Credentials
	}

	addr := fmt.Sprintf("%s:%d", params.Host, params.Port)
	authenticationToken, err := auth.BuildAuthToken(
		ctx,
		addr,
		params.Region,
		params.Username,
		params.AwsCredentials,
	)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}

	config.User = params.Username
	config.Passwd = authenticationToken
	config.Addr = addr
	config.DBName = params.Schema

	// IAM requires clear text authentication
	config.AllowCleartextPasswords = true
	// IAM requires native password authentication
	config.AllowNativePasswords = true

	return &config, nil
}
