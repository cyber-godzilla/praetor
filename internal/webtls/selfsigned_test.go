package webtls

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestEnsureGeneratesAndReusesPersistentSelfSignedCertificate(t *testing.T) {
	stateDir := t.TempDir()
	now := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	options := ensureOptions{
		stateDir:   stateDir,
		listenAddr: "192.0.2.20:8787",
		now:        func() time.Time { return now },
		hostname:   func() (string, error) { return "praetor-host", nil },
	}

	first, err := ensure(options)
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	if !first.Generated {
		t.Fatal("first ensure did not report certificate generation")
	}
	if first.Fingerprint == "" {
		t.Fatal("generated certificate has no fingerprint")
	}
	if !slices.Contains(first.DNSNames, "localhost") || !slices.Contains(first.DNSNames, "praetor-host") {
		t.Fatalf("DNS SANs = %v", first.DNSNames)
	}
	if !slices.Contains(first.IPAddresses, "127.0.0.1") || !slices.Contains(first.IPAddresses, "192.0.2.20") || !slices.Contains(first.IPAddresses, "::1") {
		t.Fatalf("IP SANs = %v", first.IPAddresses)
	}
	if !first.NotBefore.Equal(now.Add(-5*time.Minute)) || !first.NotAfter.Equal(now.Add(certificateLifetime)) {
		t.Fatalf("validity = %s through %s", first.NotBefore, first.NotAfter)
	}

	assertMode(t, filepath.Dir(first.CertificateFile), 0o700)
	assertMode(t, first.CertificateFile, 0o644)
	assertMode(t, first.PrivateKeyFile, 0o600)
	if _, err := tls.LoadX509KeyPair(first.CertificateFile, first.PrivateKeyFile); err != nil {
		t.Fatalf("loading generated pair: %v", err)
	}
	firstCertificate := readFile(t, first.CertificateFile)
	firstKey := readFile(t, first.PrivateKeyFile)

	second, err := ensure(options)
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if second.Generated {
		t.Fatal("second ensure unexpectedly regenerated certificate")
	}
	if second.Fingerprint != first.Fingerprint {
		t.Fatalf("fingerprint changed: %q != %q", second.Fingerprint, first.Fingerprint)
	}
	if !bytes.Equal(readFile(t, second.CertificateFile), firstCertificate) || !bytes.Equal(readFile(t, second.PrivateKeyFile), firstKey) {
		t.Fatal("persistent automatic TLS pair changed on reuse")
	}
}

func TestEnsureRenewsCertificateNearExpiry(t *testing.T) {
	stateDir := t.TempDir()
	now := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	options := ensureOptions{
		stateDir:   stateDir,
		listenAddr: "127.0.0.1:8787",
		now:        func() time.Time { return now },
		hostname:   func() (string, error) { return "", errors.New("unavailable") },
	}
	first, err := ensure(options)
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	firstCertificate := readFile(t, first.CertificateFile)
	firstKey := readFile(t, first.PrivateKeyFile)

	now = first.NotAfter.Add(-renewBefore / 2)
	renewed, err := ensure(options)
	if err != nil {
		t.Fatalf("renew ensure: %v", err)
	}
	if !renewed.Generated {
		t.Fatal("near-expiry certificate was not renewed")
	}
	if bytes.Equal(readFile(t, renewed.CertificateFile), firstCertificate) || bytes.Equal(readFile(t, renewed.PrivateKeyFile), firstKey) {
		t.Fatal("renewal reused the old certificate or key")
	}
	if !renewed.NotAfter.After(first.NotAfter) {
		t.Fatalf("renewed expiry %s is not after %s", renewed.NotAfter, first.NotAfter)
	}
}

func TestEnsureRejectsIncompleteOrUnsafePair(t *testing.T) {
	t.Run("incomplete", func(t *testing.T) {
		stateDir := t.TempDir()
		tlsDir := filepath.Join(stateDir, autoTLSDirectory)
		if err := os.MkdirAll(tlsDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tlsDir, autoCertificate), []byte("certificate"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Ensure(stateDir, "127.0.0.1:8787")
		if err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("incomplete pair error = %v", err)
		}
	})

	t.Run("permissive private key", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not expose Unix permission bits")
		}
		stateDir := t.TempDir()
		material, err := Ensure(stateDir, "127.0.0.1:8787")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(material.PrivateKeyFile, 0o644); err != nil {
			t.Fatal(err)
		}
		_, err = Ensure(stateDir, "127.0.0.1:8787")
		if err == nil || !strings.Contains(err.Error(), "group or other") {
			t.Fatalf("permissive key error = %v", err)
		}
	})

	t.Run("symlink private key", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink setup is not portable on Windows")
		}
		stateDir := t.TempDir()
		material, err := Ensure(stateDir, "127.0.0.1:8787")
		if err != nil {
			t.Fatal(err)
		}
		keyCopy := material.PrivateKeyFile + ".copy"
		if err := os.Rename(material.PrivateKeyFile, keyCopy); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(keyCopy, material.PrivateKeyFile); err != nil {
			t.Fatal(err)
		}
		_, err = Ensure(stateDir, "127.0.0.1:8787")
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("symlink key error = %v", err)
		}
	})
}

func TestEnsureRejectsCorruptExistingPair(t *testing.T) {
	stateDir := t.TempDir()
	material, err := Ensure(stateDir, "127.0.0.1:8787")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(material.CertificateFile, []byte("not a certificate"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Ensure(stateDir, "127.0.0.1:8787")
	if err == nil || !strings.Contains(err.Error(), "loading automatic TLS certificate") {
		t.Fatalf("corrupt pair error = %v", err)
	}
}

func TestGeneratedCertificateIsSelfSignedServerCertificate(t *testing.T) {
	material, err := Ensure(t.TempDir(), "127.0.0.1:8787")
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(readFile(t, material.CertificateFile))
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("generated file does not contain a certificate PEM block")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if certificate.IsCA {
		t.Fatal("automatic server certificate unexpectedly acts as a CA")
	}
	if err := certificate.CheckSignature(certificate.SignatureAlgorithm, certificate.RawTBSCertificate, certificate.Signature); err != nil {
		t.Fatalf("certificate is not self-signed: %v", err)
	}
	if !slices.Contains(certificate.ExtKeyUsage, x509.ExtKeyUsageServerAuth) {
		t.Fatalf("extended key usage = %v", certificate.ExtKeyUsage)
	}
}

func TestInferredIdentitiesDoNotRequireConfiguredSANs(t *testing.T) {
	dnsNames, ipAddresses, err := inferredIdentities("0.0.0.0:8787", func() (string, error) {
		return "praetor.internal.example", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(dnsNames, "localhost") || !slices.Contains(dnsNames, "praetor.internal.example") {
		t.Fatalf("DNS names = %v", dnsNames)
	}
	for _, address := range ipAddresses {
		if address.IsUnspecified() {
			t.Fatalf("wildcard listen address became a SAN: %v", ipAddresses)
		}
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != want {
		t.Fatalf("%s mode = %04o, want %04o", path, info.Mode().Perm(), want)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
