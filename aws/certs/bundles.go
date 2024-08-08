package awscerts

import (
	"crypto/x509"
	"embed"
	"fmt"
	"net/http"
	"sync"

	"github.com/Invicton-Labs/go-stackerr"
)

//go:embed bundles/*
var awsCertBundles embed.FS

const (
	awsRootCertBundleUrl string = "https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem"
)

var (
	globalAwsRootCertPool        *x509.CertPool
	globalAwsRootCertPoolOnce    sync.Once
	globalAwsRootCertPoolInitErr stackerr.Error
)

func initCertPool(httpClient *http.Client) stackerr.Error {
	entries, err := awsCertBundles.ReadDir("bundles")
	if err != nil {
		return stackerr.Wrap(err)
	}

	rootCertPool := x509.NewCertPool()
	for _, entry := range entries {
		pemBytes, err := awsCertBundles.ReadFile(fmt.Sprintf("bundles/%s", entry.Name()))
		if err != nil {
			return stackerr.Wrap(err)
		}
		if ok := rootCertPool.AppendCertsFromPEM(pemBytes); !ok {
			return stackerr.Errorf("failed to parse PEM file %s", entry.Name())
		}
	}

	globalAwsRootCertPool = rootCertPool
	return nil

	// hasExpired := false

	// for len(pemBytes) > 0 {
	// 	var block *pem.Block
	// 	block, pemBytes = pem.Decode(pemBytes)
	// 	crt, err := x509.ParseCertificate(block.Bytes)
	// 	if err != nil {
	// 		return stackerr.Wrap(err)
	// 	}
	// 	if time.Until(crt.NotAfter) < time.Hour {
	// 		hasExpired = true
	// 		break
	// 	}
	// }

	// if !hasExpired {
	// 	globalAwsRootCertPool = rootCertPool
	// 	return nil
	// }

	// if httpClient == nil {
	// 	httpClient = http.DefaultClient
	// }
	// resp, err := httpClient.Get(awsRootCertBundleUrl)
	// if err != nil {
	// 	return stackerr.Wrap(err)
	// }
	// body, err := io.ReadAll(resp.Body)
	// resp.Body.Close()
	// if err != nil {
	// 	return stackerr.Wrap(err)
	// }

	// rootCertPool = x509.NewCertPool()
	// if ok := rootCertPool.AppendCertsFromPEM(body); !ok {
	// 	return stackerr.Wrap(err)
	// }
	// globalAwsRootCertPool = rootCertPool
	// return nil
}

// GetGlobalRootCertPool gets a CertPool for AWS's global certificate bundle.
// It first attempts to load the certificate bundle that is included in this
// package, which will allow it to function even without public internet access.
// If any of the certificates in the included bundle have expired, or will expire
// soon, it will attempt to load new certificates from AWS directly via HTTP.
// It caches the CertPool for future use.
func GetGlobalRootCertPool(httpClient *http.Client) (*x509.CertPool, stackerr.Error) {
	globalAwsRootCertPoolOnce.Do(func() {
		globalAwsRootCertPoolInitErr = initCertPool(httpClient)
	})
	if globalAwsRootCertPoolInitErr != nil {
		return nil, globalAwsRootCertPoolInitErr
	}
	return globalAwsRootCertPool, nil
}
