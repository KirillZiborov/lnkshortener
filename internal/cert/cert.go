// Package cert provides utilities for generating self-signed TLS certificates
// and private keys for use in HTTPS servers.
package cert

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"time"
)

// Constants for storing certificate data.
const (
	// CertificateFilePath specifies the file path for storing generated TLS certificate.
	CertificateFilePath = "server.crt"
	// KeyFilePath specifies the file path for storing generated private key.
	KeyFilePath = "server.key"
)

// CreateCertificate generates a self-signed certificate and private key in PEM format.
// It writes the certificate and key to the provided file paths or returns an error.
func CreateCertificate(certFile, keyFile string) error {
	// Create a certificate template.
	cert := &x509.Certificate{
		// Unique certificate number.
		SerialNumber: big.NewInt(1658),
		// Owner's info.
		Subject: pkix.Name{
			Organization: []string{"KRLZ"},
			Country:      []string{"RU"},
		},
		// Allow the certificate to be used for 127.0.0.1 and ::1
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		// The certificate is valid starting from the creation time.
		NotBefore: time.Now(),
		// Certificate validity period — 10 years.
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		// Set key usage for digital signatures and both client and server authentication.
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	// Generate a new private RSA key with a length of 4096 bits.
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	// Create an x.509 certificate.
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}

	// Encode the certificate in PEM format.
	var certPEM bytes.Buffer
	pem.Encode(&certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	// Encode the private key in PEM format.
	var privateKeyPEM bytes.Buffer
	pem.Encode(&privateKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Save the certificate to the specified file.
	err = os.WriteFile(certFile, certPEM.Bytes(), 0600)
	if err != nil {
		return err
	}

	// Save the private key to the specified file.
	err = os.WriteFile(keyFile, privateKeyPEM.Bytes(), 0600)
	if err != nil {
		return err
	}

	return nil
}
