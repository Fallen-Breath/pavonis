package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/stretchr/testify/require"
)

func writeTestCertificate(t *testing.T, dir, host string) (config.TLSCertificateConfig, *x509.Certificate) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), DNSNames: []string{host},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	key, err := x509.MarshalPKCS8PrivateKey(private)
	require.NoError(t, err)
	files := config.TLSCertificateConfig{CertFile: filepath.Join(dir, host+".crt"), KeyFile: filepath.Join(dir, host+".key")}
	require.NoError(t, os.WriteFile(files.CertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	require.NoError(t, os.WriteFile(files.KeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600))
	return files, cert
}

func testClientHello(host string) *tls.ClientHelloInfo {
	return &tls.ClientHelloInfo{
		ServerName:        host,
		SupportedVersions: []uint16{tls.VersionTLS13},
		SignatureSchemes:  []tls.SignatureScheme{tls.Ed25519},
	}
}

func TestTLSCertificatesRejectMissingAndUnknownSNI(t *testing.T) {
	files, _ := writeTestCertificate(t, t.TempDir(), "a.test")
	certificates, err := newTLSCertificates([]config.TLSCertificateConfig{files})
	require.NoError(t, err)
	for _, host := range []string{"", "unknown.test"} {
		cert, _ := certificates.selectCertificate(testClientHello(host))
		require.Nil(t, cert, "SNI %q", host)
	}
}

func TestTLSCertificatesReloadIndependently(t *testing.T) {
	dir := t.TempDir()
	a, oldA := writeTestCertificate(t, dir, "a.test")
	b, _ := writeTestCertificate(t, dir, "b.test")
	certificates, err := newTLSCertificates([]config.TLSCertificateConfig{a, b})
	require.NoError(t, err)
	// Keep the old A key in place until the second reload.
	nextA, newA := writeTestCertificate(t, t.TempDir(), "a.test")
	copyFile := func(dst, src string) {
		t.Helper()
		data, err := os.ReadFile(src)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(dst, data, 0600))
	}
	copyFile(a.CertFile, nextA.CertFile)
	_, newB := writeTestCertificate(t, dir, "b.test")
	check := func(host string, want *x509.Certificate) {
		t.Helper()
		got, err := certificates.selectCertificate(testClientHello(host))
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, want.Raw, got.Certificate[0])
	}
	require.NoError(t, certificates.reload())
	check("a.test", oldA)
	check("b.test", newB)
	copyFile(a.KeyFile, nextA.KeyFile)
	require.NoError(t, certificates.reload())
	check("a.test", newA)
	check("b.test", newB)
}
