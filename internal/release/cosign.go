package release

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

// Sigstore trust anchor: root CA and intermediate CA from the sigstore public
// good instance (v1 root ceremony, Oct 2021). Fulcio issues short-lived
// signing certificates to workloads authenticated via OIDC — in our case,
// GitHub Actions. Update these if sigstore rotates its root.
var (
	cosignRootPEM = `
-----BEGIN CERTIFICATE-----
MIIB9zCCAXygAwIBAgIUALZNAPFdxHPwjeDloDwyYChAO/4wCgYIKoZIzj0EAwMw
KjEVMBMGA1UEChMMc2lnc3RvcmUuZGV2MREwDwYDVQQDEwhzaWdzdG9yZTAeFw0y
MTEwMDcxMzU2NTlaFw0zMTEwMDUxMzU2NThaMCoxFTATBgNVBAoTDHNpZ3N0b3Jl
LmRldjERMA8GA1UEAxMIc2lnc3RvcmUwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAAT7
XeFT4rb3PQGwS4IajtLk3/OlnpgangaBclYpsYBr5i+4ynB07ceb3LP0OIOZdxex
X69c5iVuyJRQ+Hz05yi+UF3uBWAlHpiS5sh0+H2GHE7SXrk1EC5m1Tr19L9gg92j
YzBhMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRY
wB5fkUWlZql6zJChkyLQKsXF+jAfBgNVHSMEGDAWgBRYwB5fkUWlZql6zJChkyLQ
KsXF+jAKBggqhkjOPQQDAwNpADBmAjEAj1nHeXZp+13NWBNa+EDsDP8G1WWg1tCM
WP/WHPqpaVo0jhsweNFZgSs0eE7wYI4qAjEA2WB9ot98sIkoF3vZYdd3/VtWB5b9
TNMea7Ix/stJ5TfcLLeABLE4BNJOsQ4vnBHJ
-----END CERTIFICATE-----`

	cosignIntermediatePEM = `
-----BEGIN CERTIFICATE-----
MIICGjCCAaGgAwIBAgIUALnViVfnU0brJasmRkHrn/UnfaQwCgYIKoZIzj0EAwMw
KjEVMBMGA1UEChMMc2lnc3RvcmUuZGV2MREwDwYDVQQDEwhzaWdzdG9yZTAeFw0y
MjA0MTMyMDA2MTVaFw0zMTEwMDUxMzU2NThaMDcxFTATBgNVBAoTDHNpZ3N0b3Jl
LmRldjEeMBwGA1UEAxMVc2lnc3RvcmUtaW50ZXJtZWRpYXRlMHYwEAYHKoZIzj0C
AQYFK4EEACIDYgAE8RVS/ysH+NOvuDZyPIZtilgUF9NlarYpAd9HP1vBBH1U5CV7
7LSS7s0ZiH4nE7Hv7ptS6LvvR/STk798LVgMzLlJ4HeIfF3tHSaexLcYpSASr1kS
0N/RgBJz/9jWCiXno3sweTAOBgNVHQ8BAf8EBAMCAQYwEwYDVR0lBAwwCgYIKwYB
BQUHAwMwEgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQU39Ppz1YkEZb5qNjp
KFWixi4YZD8wHwYDVR0jBBgwFoAUWMAeX5FFpWapesyQoZMi0CrFxfowCgYIKoZI
zj0EAwMDZwAwZAIwPCsQK4DYiZYDPIaDi5HFKnfxXx6ASSVmERfsynYBiX2X6SJR
nZU84/9DZdnFvvxmAjBOt6QpBlc4J/0DxvkTCqpclvziL6BCCPnjdlIB3Pu3BxsP
mygUY7Ii2zbdCdliiow=
-----END CERTIFICATE-----`
)

const (
	expectedOIDCIssuer = "https://token.actions.githubusercontent.com"
	expectedRepoURI    = "https://github.com/" + owner + "/" + repo
)

var oidFulcioIssuer = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}

// verifyCosignSignature checks that checksums were signed by a Fulcio-issued
// certificate tied to this project's GitHub Actions workflow:
//  1. Certificate chain anchors to the embedded sigstore root
//  2. OIDC issuer is GitHub Actions
//  3. SAN URI belongs to this repository's workflow
//  4. ECDSA signature over data is valid
func verifyCosignSignature(data, sigBase64, certPEM []byte) error {
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigBase64)))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	certs, err := parsePEMChain(certPEM)
	if err != nil {
		return err
	}
	if len(certs) == 0 {
		return fmt.Errorf("no certificates in signing certificate PEM")
	}
	leaf := certs[0]

	rootPool := x509.NewCertPool()
	if !rootPool.AppendCertsFromPEM([]byte(cosignRootPEM)) {
		return fmt.Errorf("failed to load embedded sigstore root certificate")
	}

	intermediatePool := x509.NewCertPool()
	intermediatePool.AppendCertsFromPEM([]byte(cosignIntermediatePEM))
	for _, c := range certs[1:] {
		intermediatePool.AddCert(c)
	}

	// Fulcio certs live ~10 minutes; by upgrade time they are expired. Use a
	// timestamp inside the cert's validity window for chain verification.
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         rootPool,
		Intermediates: intermediatePool,
		CurrentTime:   leaf.NotBefore.Add(time.Minute),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}); err != nil {
		return fmt.Errorf("certificate chain: %w", err)
	}

	if err := checkFulcioIdentity(leaf); err != nil {
		return err
	}

	pubKey, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("signing certificate uses unsupported key type (want ECDSA)")
	}
	h := sha256.Sum256(data)
	if !ecdsa.VerifyASN1(pubKey, h[:], sig) {
		return fmt.Errorf("signature does not match checksums content")
	}

	return nil
}

func checkFulcioIdentity(cert *x509.Certificate) error {
	issuer := certExtension(cert, oidFulcioIssuer)
	if issuer != expectedOIDCIssuer {
		return fmt.Errorf("OIDC issuer %q does not match expected %q", issuer, expectedOIDCIssuer)
	}

	for _, u := range cert.URIs {
		if strings.HasPrefix(u.String(), expectedRepoURI+"/") {
			return nil
		}
	}
	return fmt.Errorf("no SAN URI matching %s", expectedRepoURI)
}

func certExtension(cert *x509.Certificate, oid asn1.ObjectIdentifier) string {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(oid) {
			var val string
			if _, err := asn1.Unmarshal(ext.Value, &val); err == nil {
				return val
			}
		}
	}
	return ""
}

func parsePEMChain(data []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse certificate: %w", err)
		}
		certs = append(certs, cert)
	}
	return certs, nil
}
