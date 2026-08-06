package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestParseSSLMode(t *testing.T) {
	tests := []struct {
		in   string
		want SSLMode
	}{
		{"DISABLED", SSLModeDisabled},
		{"PREFERRED", SSLModePreferred},
		{"REQUIRED", SSLModeRequired},
		{"VERIFY_CA", SSLModeVerifyCA},
		{"VERIFY_IDENTITY", SSLModeVerifyIdentity},
		{"verify_ca", SSLModeVerifyCA}, // case insensitive
	}
	for _, tt := range tests {
		got, err := parseSSLMode(tt.in)
		if err != nil {
			t.Errorf("parseSSLMode(%q) returns an error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseSSLMode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	for _, in := range []string{"", "ENABLED", "verify-ca"} {
		if _, err := parseSSLMode(in); err == nil {
			t.Errorf("parseSSLMode(%q) doesn't return an error", in)
		}
	}
}

func TestConfigureTLS(t *testing.T) {
	tests := []struct {
		name     string
		cfn      *config
		want     string // the value of the tls parameter in the DSN
		fallback bool   // whether the connection falls back to plaintext
	}{
		{
			name: "disabled",
			cfn:  &config{SSLMode: SSLModeDisabled},
			want: "false",
		},
		{
			name:     "preferred",
			cfn:      &config{SSLMode: SSLModePreferred},
			want:     tlsConfigNameInsecure,
			fallback: true,
		},
		{
			name: "required",
			cfn:  &config{SSLMode: SSLModeRequired},
			want: tlsConfigNameInsecure,
		},
		{
			name: "verify_ca",
			cfn:  &config{SSLMode: SSLModeVerifyCA},
			want: tlsConfigName,
		},
		{
			name: "verify_identity",
			cfn:  &config{SSLMode: SSLModeVerifyIdentity, Host: "db.example.com"},
			want: tlsConfigName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := mysql.NewConfig()
			dsn.Net = "tcp"
			dsn.Addr = "db.example.com:3306"
			if err := tt.cfn.configureTLS(dsn); err != nil {
				t.Fatal(err)
			}
			if dsn.TLSConfig != tt.want {
				t.Errorf("unexpected tls configure: got %q, want %q", dsn.TLSConfig, tt.want)
			}

			// the DSN must keep the TLS settings.
			// mysql.Config.FormatDSN serializes TLSConfig, but not TLS.
			parsed, err := mysql.ParseDSN(dsn.FormatDSN())
			if err != nil {
				t.Fatal(err)
			}
			if parsed.TLSConfig != tt.want {
				t.Errorf("the tls configure is lost in the DSN: got %q, want %q", parsed.TLSConfig, tt.want)
			}
			if tt.cfn.SSLMode != SSLModeDisabled && parsed.TLS == nil {
				t.Error("the TLS configure is not restored from the DSN")
			}
			if parsed.AllowFallbackToPlaintext != tt.fallback {
				t.Errorf("unexpected fallback to plaintext: got %t, want %t", parsed.AllowFallbackToPlaintext, tt.fallback)
			}
		})
	}
}

func TestCipherSuites(t *testing.T) {
	// MySQL 5.7 and MariaDB 10.1 can't use the elliptic curve key exchange,
	// so the cipher suites must contain the ones that use the RSA key exchange.
	// crypto/tls excludes them from the default since Go 1.22.
	want := map[uint16]bool{
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256: false,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384: false,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA:    false,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA:    false,
	}
	suites := cipherSuites()
	for _, id := range suites {
		if _, ok := want[id]; ok {
			want[id] = true
		}
	}
	for id, found := range want {
		if !found {
			t.Errorf("the cipher suite %s is not contained", tls.CipherSuiteName(id))
		}
	}

	// it must keep the modern cipher suites too.
	if len(suites) <= len(want) {
		t.Errorf("the cipher suites are too few: %d", len(suites))
	}
}

func TestConfigureTLS_error(t *testing.T) {
	tests := []struct {
		name string
		cfn  *config
	}{
		{
			name: "unknown ssl-mode",
			cfn:  &config{SSLMode: SSLMode("ENABLED")},
		},
		{
			name: "the ca certificate doesn't exist",
			cfn:  &config{SSLMode: SSLModeVerifyCA, SSLCA: filepath.Join(t.TempDir(), "not-exist.pem")},
		},
		{
			name: "verify_identity with the unix domain socket",
			cfn:  &config{SSLMode: SSLModeVerifyIdentity, Socket: "/tmp/mysql.sock"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := mysql.NewConfig()
			if err := tt.cfn.configureTLS(dsn); err == nil {
				t.Error("configureTLS doesn't return an error")
			}
		})
	}
}

func TestConfigureTLS_brokenCACertificate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(path, []byte("this is not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfn := &config{SSLMode: SSLModeVerifyCA, SSLCA: path}
	if err := cfn.configureTLS(mysql.NewConfig()); err == nil {
		t.Error("configureTLS doesn't return an error")
	}
}

func TestVerifyCA(t *testing.T) {
	caCert, caKey := newCertificate(t, "schemalex-deploy test ca", nil, nil)
	serverCert, _ := newCertificate(t, "db.example.com", caCert, caKey)

	// the certificate is signed by the ca.
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	if err := verifyCA(pool)([][]byte{serverCert.Raw}, nil); err != nil {
		t.Errorf("verifyCA rejects the valid certificate: %v", err)
	}

	// the certificate is not signed by any of the ca.
	if err := verifyCA(x509.NewCertPool())([][]byte{serverCert.Raw}, nil); err == nil {
		t.Error("verifyCA accepts the certificate signed by an unknown ca")
	}

	// the host name is not verified.
	otherCert, _ := newCertificate(t, "another.example.com", caCert, caKey)
	if err := verifyCA(pool)([][]byte{otherCert.Raw}, nil); err != nil {
		t.Errorf("verifyCA verifies the host name: %v", err)
	}

	// no certificate is sent.
	if err := verifyCA(pool)(nil, nil); err == nil {
		t.Error("verifyCA accepts the empty certificate chain")
	}
}

func TestTLSConfig_loadCACertificate(t *testing.T) {
	caCert, _ := newCertificate(t, "schemalex-deploy test ca", nil, nil)
	path := filepath.Join(t.TempDir(), "ca.pem")
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfn := &config{SSLMode: SSLModeVerifyIdentity, Host: "db.example.com", SSLCA: path}
	tlsConfig, err := cfn.tlsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if tlsConfig.RootCAs == nil {
		t.Fatal("the ca certificate is not loaded")
	}
	if tlsConfig.InsecureSkipVerify {
		t.Error("VERIFY_IDENTITY must verify the server certificate")
	}
	if tlsConfig.ServerName != "db.example.com" {
		t.Errorf("unexpected server name: got %q, want %q", tlsConfig.ServerName, "db.example.com")
	}
}

func TestTLSConfig_defaultServerName(t *testing.T) {
	// the driver connects to the localhost if the host is not specified,
	// and crypto/tls fails if the server name is empty.
	cfn := &config{SSLMode: SSLModeVerifyIdentity}
	tlsConfig, err := cfn.tlsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if tlsConfig.ServerName != "localhost" {
		t.Errorf("unexpected server name: got %q, want %q", tlsConfig.ServerName, "localhost")
	}
}

// newCertificate creates a certificate for testing.
// if parent is nil, it creates a self-signed ca certificate.
func newCertificate(t *testing.T, commonName string, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	if parent == nil {
		template.IsCA = true
		template.BasicConstraintsValid = true
		template.KeyUsage |= x509.KeyUsageCertSign
		parent = template
		parentKey = key
	} else {
		template.DNSNames = []string{commonName}
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}

	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key
}
