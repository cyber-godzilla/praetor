// Package webtls manages the automatic self-signed certificate used by the
// shared web client when an operator does not provide a certificate pair.
package webtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	autoTLSDirectory    = "tls"
	autoCertificate     = "praetor-web-self-signed.crt"
	autoPrivateKey      = "praetor-web-self-signed.key"
	certificateMaxSize  = 1 << 20
	certificateLifetime = 5 * 365 * 24 * time.Hour
	renewBefore         = 30 * 24 * time.Hour
)

// Material describes the persistent automatic certificate selected for the
// web listener. Generated is true when this call created or renewed the pair.
type Material struct {
	CertificateFile string
	PrivateKeyFile  string
	Fingerprint     string
	DNSNames        []string
	IPAddresses     []string
	NotBefore       time.Time
	NotAfter        time.Time
	Generated       bool
}

type ensureOptions struct {
	stateDir   string
	listenAddr string
	now        func() time.Time
	random     io.Reader
	hostname   func() (string, error)
}

// Ensure creates or reuses a persistent self-signed server certificate below
// stateDir. The certificate is an encryption fallback, not a local trust
// authority; browsers are expected to report it as untrusted.
func Ensure(stateDir, listenAddr string) (Material, error) {
	return ensure(ensureOptions{
		stateDir:   stateDir,
		listenAddr: listenAddr,
		now:        time.Now,
		random:     rand.Reader,
		hostname:   os.Hostname,
	})
}

func ensure(options ensureOptions) (Material, error) {
	if options.stateDir == "" {
		return Material{}, errors.New("automatic TLS state directory is required")
	}
	if options.now == nil {
		options.now = time.Now
	}
	if options.random == nil {
		options.random = rand.Reader
	}
	if options.hostname == nil {
		options.hostname = os.Hostname
	}

	tlsDir := filepath.Join(options.stateDir, autoTLSDirectory)
	if err := ensurePrivateDirectory(tlsDir); err != nil {
		return Material{}, err
	}
	certificateFile := filepath.Join(tlsDir, autoCertificate)
	privateKeyFile := filepath.Join(tlsDir, autoPrivateKey)

	certificateExists, err := regularFileExists(certificateFile, false)
	if err != nil {
		return Material{}, err
	}
	privateKeyExists, err := regularFileExists(privateKeyFile, true)
	if err != nil {
		return Material{}, err
	}
	if certificateExists != privateKeyExists {
		return Material{}, fmt.Errorf("automatic TLS certificate pair is incomplete: expected both %s and %s", certificateFile, privateKeyFile)
	}

	now := options.now().UTC()
	if certificateExists {
		material, err := loadMaterial(certificateFile, privateKeyFile)
		if err != nil {
			return Material{}, fmt.Errorf("loading automatic TLS certificate: %w", err)
		}
		if !now.Before(material.NotBefore) && now.Before(material.NotAfter.Add(-renewBefore)) {
			return material, nil
		}
	}

	dnsNames, ipAddresses, err := inferredIdentities(options.listenAddr, options.hostname)
	if err != nil {
		return Material{}, err
	}
	certPEM, keyPEM, err := generateSelfSigned(now, dnsNames, ipAddresses, options.random)
	if err != nil {
		return Material{}, err
	}
	defer clear(certPEM)
	defer clear(keyPEM)

	// Install the key first. If the process is interrupted between these two
	// atomic replacements, the next startup fails closed on an incomplete pair
	// instead of silently replacing uncertain key material.
	if err := atomicWriteFile(privateKeyFile, keyPEM, 0o600); err != nil {
		return Material{}, fmt.Errorf("writing automatic TLS private key: %w", err)
	}
	if err := atomicWriteFile(certificateFile, certPEM, 0o644); err != nil {
		return Material{}, fmt.Errorf("writing automatic TLS certificate: %w", err)
	}
	material, err := loadMaterial(certificateFile, privateKeyFile)
	if err != nil {
		return Material{}, fmt.Errorf("validating generated automatic TLS certificate: %w", err)
	}
	material.Generated = true
	return material, nil
}

func ensurePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("creating automatic TLS directory: %w", err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return fmt.Errorf("examining automatic TLS directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("automatic TLS path is not a regular directory")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("securing automatic TLS directory: %w", err)
	}
	return nil
}

func regularFileExists(path string, private bool) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("examining %s: %w", path, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > certificateMaxSize {
		return false, fmt.Errorf("%s exceeds %d bytes", path, certificateMaxSize)
	}
	if private && info.Mode().Perm()&0o077 != 0 {
		return false, fmt.Errorf("automatic TLS private key %s must not be accessible by group or other users", path)
	}
	return true, nil
}

func loadMaterial(certificateFile, privateKeyFile string) (Material, error) {
	pair, err := tls.LoadX509KeyPair(certificateFile, privateKeyFile)
	if err != nil {
		return Material{}, err
	}
	if len(pair.Certificate) == 0 {
		return Material{}, errors.New("certificate file contains no certificate")
	}
	certificate, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return Material{}, fmt.Errorf("parsing certificate: %w", err)
	}
	if err := certificate.CheckSignature(certificate.SignatureAlgorithm, certificate.RawTBSCertificate, certificate.Signature); err != nil {
		return Material{}, fmt.Errorf("certificate is not self-signed: %w", err)
	}
	fingerprint := sha256.Sum256(certificate.Raw)
	ipAddresses := make([]string, 0, len(certificate.IPAddresses))
	for _, address := range certificate.IPAddresses {
		ipAddresses = append(ipAddresses, address.String())
	}
	return Material{
		CertificateFile: certificateFile,
		PrivateKeyFile:  privateKeyFile,
		Fingerprint:     colonHex(fingerprint[:]),
		DNSNames:        append([]string(nil), certificate.DNSNames...),
		IPAddresses:     ipAddresses,
		NotBefore:       certificate.NotBefore,
		NotAfter:        certificate.NotAfter,
	}, nil
}

func generateSelfSigned(now time.Time, dnsNames []string, ipAddresses []net.IP, random io.Reader) ([]byte, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), random)
	if err != nil {
		return nil, nil, fmt.Errorf("generating automatic TLS private key: %w", err)
	}
	serial, err := randomSerial(random)
	if err != nil {
		return nil, nil, err
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("encoding automatic TLS public key: %w", err)
	}
	subjectKeyID := sha256.Sum256(publicKeyDER)
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "Praetor Web Automatic Self-Signed Certificate",
			Organization: []string{"Praetor"},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(certificateLifetime),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		SubjectKeyId:          append([]byte(nil), subjectKeyID[:20]...),
		DNSNames:              append([]string(nil), dnsNames...),
		IPAddresses:           append([]net.IP(nil), ipAddresses...),
	}
	certificateDER, err := x509.CreateCertificate(random, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("creating automatic TLS certificate: %w", err)
	}
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("encoding automatic TLS private key: %w", err)
	}
	defer clear(privateKeyDER)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		clear(certPEM)
		clear(keyPEM)
		return nil, nil, errors.New("encoding automatic TLS PEM data")
	}
	return certPEM, keyPEM, nil
}

func randomSerial(random io.Reader) (*big.Int, error) {
	serialBytes := make([]byte, 16)
	if _, err := io.ReadFull(random, serialBytes); err != nil {
		return nil, fmt.Errorf("generating automatic TLS certificate serial: %w", err)
	}
	serialBytes[0] &= 0x7f
	serial := new(big.Int).SetBytes(serialBytes)
	if serial.Sign() == 0 {
		serial.SetInt64(1)
	}
	return serial, nil
}

func inferredIdentities(listenAddr string, hostname func() (string, error)) ([]string, []net.IP, error) {
	dnsSet := map[string]struct{}{"localhost": {}}
	ipSet := map[string]net.IP{
		"127.0.0.1": net.ParseIP("127.0.0.1"),
		"::1":       net.ParseIP("::1"),
	}
	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing listen address for automatic TLS: %w", err)
	}
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	if zoneIndex := strings.LastIndexByte(host, '%'); zoneIndex > 0 {
		host = host[:zoneIndex]
	}
	if address := net.ParseIP(host); address != nil {
		if !address.IsUnspecified() {
			ipSet[address.String()] = address
		}
	} else if validDNSName(host) {
		dnsSet[strings.ToLower(host)] = struct{}{}
	}
	if systemHostname, err := hostname(); err == nil {
		systemHostname = strings.TrimSpace(strings.TrimSuffix(systemHostname, "."))
		if validDNSName(systemHostname) {
			dnsSet[strings.ToLower(systemHostname)] = struct{}{}
		}
	}

	dnsNames := make([]string, 0, len(dnsSet))
	for name := range dnsSet {
		dnsNames = append(dnsNames, name)
	}
	sort.Strings(dnsNames)
	ipStrings := make([]string, 0, len(ipSet))
	for address := range ipSet {
		ipStrings = append(ipStrings, address)
	}
	sort.Strings(ipStrings)
	ipAddresses := make([]net.IP, 0, len(ipStrings))
	for _, address := range ipStrings {
		ipAddresses = append(ipAddresses, ipSet[address])
	}
	return dnsNames, ipAddresses, nil
}

func validDNSName(name string) bool {
	if name == "" || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
				(character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".tls-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return err
	}
	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func colonHex(value []byte) string {
	encoded := strings.ToUpper(hex.EncodeToString(value))
	parts := make([]string, 0, len(encoded)/2)
	for index := 0; index < len(encoded); index += 2 {
		parts = append(parts, encoded[index:index+2])
	}
	return strings.Join(parts, ":")
}
