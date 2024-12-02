/*
Copyright 2024.

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
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"

	certv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetCertificateSigningRequest generates a Certificate Signing Request (CSR)
// for a given common name, organization, and Subject Alternative Names (SANs).
//
// Parameters:
//   - commonName: The common name (CN) for the certificate request.
//   - organization: A slice of organization names (O) for the certificate request.
//   - san: A slice of DNS names to be included as Subject Alternative Names.
//   - signerName: The name of the signer (e.g., kubernetes.io/kube-apiserver-client).
//   - usages: A slice of KeyUsage values specifying the intended usages of the certificate.
//
// Returns:
//   - *certv1.CertificateSigningRequest: A populated CSR object ready for submission.
//   - []byte: The private key in PEM-encoded format corresponding to the CSR.
//   - error: An error if there was an issue generating the CSR or private key.
//
// The function generates an ECDSA private key, creates a CSR template with the provided
// details, and encodes the private key to PEM format. The resulting CSR is returned along
// with the private key PEM data.
//
// Example usage:
//
//	csr, privateKeyPEM, err := GetCertificateSigningRequest("example.com", []string{"ExampleOrg"}, []string{"example.com"}, "kubernetes.io/kubelet-serving", []certv1.KeyUsage{certv1.UsageServerAuth})
func GetCertificateSigningRequest(csrRequestName, commonName string, organization, san []string, signerName string) (*certv1.CertificateSigningRequest, []byte, error) {

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed generating private key")
	}
	csrTemplate := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: organization,
		},
		DNSNames: san,
	}

	createCertificateRequest, err := x509.CreateCertificateRequest(rand.Reader, &csrTemplate, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CSR: %v", err)
	}

	privateKeyPEM, err := encodePrivateKeyToPEM(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode private key: %v", err)
	}
	return (&certv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: csrRequestName,
			Labels: map[string]string{
				"managed-by": "ceph-csi-operator",
			},
		},
		Spec: certv1.CertificateSigningRequestSpec{
			Request:    createCertificateRequest,
			SignerName: signerName,
			Usages: []certv1.KeyUsage{
				certv1.UsageDigitalSignature,
				certv1.UsageKeyEncipherment,
				certv1.UsageServerAuth,
			},
		},
	}), privateKeyPEM, nil

}

func encodePrivateKeyToPEM(priv crypto.PrivateKey) ([]byte, error) {
	privBytes, err := x509.MarshalECPrivateKey(priv.(*ecdsa.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	return privPEM, nil
}
