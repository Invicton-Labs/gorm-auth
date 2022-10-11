package gormaws

import (
	"context"
	"crypto/tls"

	awscerts "github.com/Invicton-Labs/gorm-auth/aws/certs"
	"github.com/pkg/errors"
)

// GetTlsConfig will get a *tls.Config that trusts the AWS Root CAs
// for the given host.
func GetTlsConfig(ctx context.Context, host string) (*tls.Config, error) {
	rootCaPool, err := awscerts.GetGlobalRootCertPool(nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &tls.Config{
		RootCAs:    rootCaPool,
		ServerName: host,
	}, nil
}
