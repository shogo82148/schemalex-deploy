package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// SSLMode is the desired security state of the connection to the server.
// It is compatible with the --ssl-mode option of the mysql(1).
// https://dev.mysql.com/doc/refman/8.0/en/connection-options.html#option_general_ssl-mode
type SSLMode string

const (
	// SSLModeDisabled establishes an unencrypted connection.
	SSLModeDisabled SSLMode = "DISABLED"

	// SSLModePreferred establishes an encrypted connection if the server supports it,
	// and falls back to an unencrypted connection otherwise.
	// It is the default, and it doesn't verify the server certificate.
	SSLModePreferred SSLMode = "PREFERRED"

	// SSLModeRequired establishes an encrypted connection, and fails if it is not available.
	// It doesn't verify the server certificate.
	SSLModeRequired SSLMode = "REQUIRED"

	// SSLModeVerifyCA establishes an encrypted connection,
	// and verifies the server certificate against the CA certificates.
	SSLModeVerifyCA SSLMode = "VERIFY_CA"

	// SSLModeVerifyIdentity establishes an encrypted connection,
	// and verifies the server certificate against the CA certificates
	// in addition to the host name of the server.
	SSLModeVerifyIdentity SSLMode = "VERIFY_IDENTITY"
)

// parseSSLMode parses the value of the -ssl-mode option.
func parseSSLMode(s string) (SSLMode, error) {
	mode := SSLMode(strings.ToUpper(s))
	switch mode {
	case SSLModeDisabled, SSLModePreferred, SSLModeRequired, SSLModeVerifyCA, SSLModeVerifyIdentity:
		return mode, nil
	}
	return "", fmt.Errorf("unknown ssl-mode: %q", s)
}

// tlsConfigName is the name of the TLS configuration registered to the mysql driver.
const tlsConfigName = "schemalex-deploy"

// configureTLS configures the TLS settings of dsn.
//
// The mysql driver serializes only [mysql.Config.TLSConfig] into the DSN,
// so the configuration needs to be registered by [mysql.RegisterTLSConfig]
// instead of setting [mysql.Config.TLS] directly.
func (cfn *config) configureTLS(dsn *mysql.Config) error {
	switch cfn.SSLMode {
	case SSLModeDisabled:
		dsn.TLSConfig = "false"
	case SSLModePreferred:
		// use TLS if the server supports it, and fall back to an unencrypted connection otherwise.
		dsn.TLSConfig = "preferred"
	case SSLModeRequired:
		// require TLS, but don't verify the server certificate.
		dsn.TLSConfig = "skip-verify"
	case SSLModeVerifyCA, SSLModeVerifyIdentity:
		tlsConfig, err := cfn.tlsConfig()
		if err != nil {
			return err
		}
		if err := mysql.RegisterTLSConfig(tlsConfigName, tlsConfig); err != nil {
			return fmt.Errorf("failed to register the tls configure: %w", err)
		}
		dsn.TLSConfig = tlsConfigName
	default:
		return fmt.Errorf("unknown ssl-mode: %q", string(cfn.SSLMode))
	}
	return nil
}

// tlsConfig builds the TLS configuration for VERIFY_CA and VERIFY_IDENTITY.
func (cfn *config) tlsConfig() (*tls.Config, error) {
	// if no CA certificate is given, the system certificate pool is used.
	var rootCAs *x509.CertPool
	if cfn.SSLCA != "" {
		data, err := os.ReadFile(cfn.SSLCA)
		if err != nil {
			return nil, fmt.Errorf("failed to read the ca certificate: %w", err)
		}
		rootCAs = x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("failed to parse the ca certificate %q", cfn.SSLCA)
		}
	}

	if cfn.SSLMode == SSLModeVerifyIdentity {
		if cfn.Socket != "" {
			return nil, errors.New("ssl-mode VERIFY_IDENTITY is not available with the unix domain socket")
		}
		serverName := cfn.Host
		if serverName == "" {
			// the connection goes to the localhost if the host is not specified.
			serverName = "localhost"
		}
		return &tls.Config{
			RootCAs:    rootCAs,
			ServerName: serverName,
			MinVersion: tls.VersionTLS12,
		}, nil
	}

	// VERIFY_CA verifies the certificate chain, but doesn't verify the host name.
	// crypto/tls has no such option, so we verify the chain by ourselves.
	return &tls.Config{
		RootCAs:               rootCAs,
		InsecureSkipVerify:    true, // the chain is verified in verifyCA.
		VerifyPeerCertificate: verifyCA(rootCAs),
		MinVersion:            tls.VersionTLS12,
	}, nil
}

// verifyCA returns a function that verifies the certificate chain without verifying the host name.
func verifyCA(rootCAs *x509.CertPool) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return errors.New("the server sent no certificate")
		}
		certs := make([]*x509.Certificate, 0, len(rawCerts))
		for _, rawCert := range rawCerts {
			cert, err := x509.ParseCertificate(rawCert)
			if err != nil {
				return fmt.Errorf("failed to parse the server certificate: %w", err)
			}
			certs = append(certs, cert)
		}

		intermediates := x509.NewCertPool()
		for _, cert := range certs[1:] {
			intermediates.AddCert(cert)
		}
		_, err := certs[0].Verify(x509.VerifyOptions{
			Roots:         rootCAs,
			Intermediates: intermediates,
		})
		if err != nil {
			return fmt.Errorf("failed to verify the server certificate: %w", err)
		}
		return nil
	}
}
