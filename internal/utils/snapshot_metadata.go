/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	CommonName = "ceph-csi-operator-snapshot-metadata"
)

// createCACertificate creates a self-signed CA certificate and private key using RSA.
func CreateCACertificate() (*x509.Certificate, *rsa.PrivateKey, error) {
	// Generate a private key for the CA
	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096) // 4096-bit RSA key
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA private key: %v", err)
	}

	// Create a CA certificate template
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: CommonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year validity
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Self-sign the CA certificate
	caCertBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CA certificate: %v", err)
	}

	caCert, err := x509.ParseCertificate(caCertBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %v", err)
	}
	return caCert, caPrivateKey, nil
}

// createServerCertificate creates a server certificate signed by the given CA using RSA.
func CreateServerCertificate(caCert *x509.Certificate, caPrivateKey *rsa.PrivateKey, driverNamespace string) ([]byte, []byte, error) {
	// Generate a private key for the server
	serverPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096) // 4096-bit RSA key
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate server private key: %v", err)
	}

	// Create a server certificate template
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		DNSNames: []string{
			fmt.Sprintf(".%s", driverNamespace),
			fmt.Sprintf("%s.%s", CommonName, driverNamespace),
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour), // 1 year validity
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	// Sign the server certificate with the CA certificate
	serverCertBytes, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create server certificate: %v", err)
	}

	// Marshal the server private key
	serverKeyBytes := x509.MarshalPKCS1PrivateKey(serverPrivateKey)

	return serverCertBytes, serverKeyBytes, nil
}

func GetCertificateExpiry(tlsSecret *corev1.Secret) (time.Time, error) {
	CertPEM := tlsSecret.Data["tls.crt"]
	block, _ := pem.Decode(CertPEM)
	if block == nil {
		return time.Time{}, fmt.Errorf("failed to decode CA certificate PEM")
	}
	Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	return Cert.NotAfter, nil
}

func GetCACertFromSecret(tlsSecret *corev1.Secret) (*x509.Certificate, error) {
	caCertPEM := tlsSecret.Data["ca.crt"]
	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}
	return caCert, nil
}
