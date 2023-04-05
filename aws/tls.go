package gormaws

import (
	"context"
	"crypto/tls"

	"github.com/Invicton-Labs/go-stackerr"
	awscerts "github.com/Invicton-Labs/gorm-auth/aws/certs"
)

// GetTlsConfig will get a *tls.Config that trusts the AWS Root CAs
// for the given host.
func GetTlsConfig(ctx context.Context, host string) (*tls.Config, stackerr.Error) {
	rootCaPool, err := awscerts.GetGlobalRootCertPool(nil)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return &tls.Config{
		RootCAs:    rootCaPool,
		ServerName: host,
	}, nil
}
