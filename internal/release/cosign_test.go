package release

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/url"
	"strings"
	"testing"
	"time"
)

type testPKI struct {
	rootPEM         []byte
	intermediatePEM []byte
	leafCert        *x509.Certificate
	leafKey         *ecdsa.PrivateKey
	leafPEM         []byte
	chainPEM        []byte // leaf + intermediate
}

func newTestPKI(t *testing.T, issuerOID string, sanURI string) testPKI {
	t.Helper()

	// Root CA
	rootKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"test"}, CommonName: "test-root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	rootCert, _ := x509.ParseCertificate(rootDER)

	// Intermediate CA
	intKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	intTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{Organization: []string{"test"}, CommonName: "test-intermediate"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}
	intDER, err := x509.CreateCertificate(rand.Reader, intTmpl, rootCert, &intKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	intCert, _ := x509.ParseCertificate(intDER)

	// Leaf cert with Fulcio-style extensions
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	issuerValue, _ := asn1.Marshal(issuerOID)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{Organization: []string{"test"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(-time.Minute),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		ExtraExtensions: []pkix.Extension{
			{Id: oidFulcioIssuer, Value: issuerValue},
		},
	}
	if sanURI != "" {
		u, err := url.Parse(sanURI)
		if err != nil {
			t.Fatal(err)
		}
		leafTmpl.URIs = []*url.URL{u}
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, intCert, &leafKey.PublicKey, intKey)
	if err != nil {
		t.Fatal(err)
	}
	leafCert, _ := x509.ParseCertificate(leafDER)

	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER})
	intPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intDER})
	leafPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})

	chain := append(leafPEMBytes, intPEMBytes...)

	return testPKI{
		rootPEM:         rootPEM,
		intermediatePEM: intPEMBytes,
		leafCert:        leafCert,
		leafKey:         leafKey,
		leafPEM:         leafPEMBytes,
		chainPEM:        chain,
	}
}

func signBlob(t *testing.T, key *ecdsa.PrivateKey, data []byte) []byte {
	t.Helper()
	h := sha256.Sum256(data)
	sig, err := ecdsa.SignASN1(rand.Reader, key, h[:])
	if err != nil {
		t.Fatal(err)
	}
	return sig
}

func withTestRoots(t *testing.T, pki testPKI) {
	t.Helper()
	origRoot, origInt := cosignRootPEM, cosignIntermediatePEM
	cosignRootPEM = string(pki.rootPEM)
	cosignIntermediatePEM = string(pki.intermediatePEM)
	t.Cleanup(func() { cosignRootPEM, cosignIntermediatePEM = origRoot, origInt })
}

const testWorkflowURI = "https://github.com/martinhg/capiko-ai/.github/workflows/release.yml@refs/tags/v1.0.0"

func TestVerifyCosignSignature(t *testing.T) {
	pki := newTestPKI(t, expectedOIDCIssuer, testWorkflowURI)
	withTestRoots(t, pki)

	data := []byte("abc123  capiko-ai_1.0.0_darwin_arm64.tar.gz\n")
	sig := signBlob(t, pki.leafKey, data)
	sigB64 := []byte(base64.StdEncoding.EncodeToString(sig))

	if err := verifyCosignSignature(data, sigB64, pki.chainPEM); err != nil {
		t.Fatalf("valid signature should pass: %v", err)
	}
}

func TestVerifyCosignSignatureBadSignature(t *testing.T) {
	pki := newTestPKI(t, expectedOIDCIssuer, testWorkflowURI)
	withTestRoots(t, pki)

	data := []byte("checksums content")
	sigB64 := []byte(base64.StdEncoding.EncodeToString([]byte("not-a-valid-signature")))

	if err := verifyCosignSignature(data, sigB64, pki.chainPEM); err == nil {
		t.Error("invalid signature should fail")
	}
}

func TestVerifyCosignSignatureTamperedData(t *testing.T) {
	pki := newTestPKI(t, expectedOIDCIssuer, testWorkflowURI)
	withTestRoots(t, pki)

	original := []byte("original checksums")
	sig := signBlob(t, pki.leafKey, original)
	sigB64 := []byte(base64.StdEncoding.EncodeToString(sig))

	tampered := []byte("tampered checksums")
	if err := verifyCosignSignature(tampered, sigB64, pki.chainPEM); err == nil {
		t.Error("tampered data should fail signature verification")
	}
}

func TestVerifyCosignSignatureWrongIssuer(t *testing.T) {
	pki := newTestPKI(t, "https://evil.example.com", testWorkflowURI)
	withTestRoots(t, pki)

	data := []byte("checksums")
	sig := signBlob(t, pki.leafKey, data)
	sigB64 := []byte(base64.StdEncoding.EncodeToString(sig))

	err := verifyCosignSignature(data, sigB64, pki.chainPEM)
	if err == nil {
		t.Fatal("wrong OIDC issuer should fail")
	}
	if got := err.Error(); !strings.Contains(got, "OIDC issuer") {
		t.Errorf("error should mention OIDC issuer, got: %s", got)
	}
}

func TestVerifyCosignSignatureWrongRepo(t *testing.T) {
	pki := newTestPKI(t, expectedOIDCIssuer,
		"https://github.com/evil/repo/.github/workflows/release.yml@refs/tags/v1.0.0")
	withTestRoots(t, pki)

	data := []byte("checksums")
	sig := signBlob(t, pki.leafKey, data)
	sigB64 := []byte(base64.StdEncoding.EncodeToString(sig))

	err := verifyCosignSignature(data, sigB64, pki.chainPEM)
	if err == nil {
		t.Fatal("wrong repo in SAN should fail")
	}
	if got := err.Error(); !strings.Contains(got, "SAN URI") {
		t.Errorf("error should mention SAN URI, got: %s", got)
	}
}

func TestVerifyCosignSignatureNoSAN(t *testing.T) {
	pki := newTestPKI(t, expectedOIDCIssuer, "")
	withTestRoots(t, pki)

	data := []byte("checksums")
	sig := signBlob(t, pki.leafKey, data)
	sigB64 := []byte(base64.StdEncoding.EncodeToString(sig))

	if err := verifyCosignSignature(data, sigB64, pki.chainPEM); err == nil {
		t.Error("missing SAN URI should fail")
	}
}

func TestVerifyCosignSignatureUntrustedCA(t *testing.T) {
	pki := newTestPKI(t, expectedOIDCIssuer, testWorkflowURI)
	// Do NOT call withTestRoots — use the real sigstore roots, which won't
	// trust the test PKI chain.

	data := []byte("checksums")
	sig := signBlob(t, pki.leafKey, data)
	sigB64 := []byte(base64.StdEncoding.EncodeToString(sig))

	err := verifyCosignSignature(data, sigB64, pki.chainPEM)
	if err == nil {
		t.Fatal("untrusted CA should fail chain verification")
	}
	if got := err.Error(); !strings.Contains(got, "certificate chain") {
		t.Errorf("error should mention certificate chain, got: %s", got)
	}
}

func TestVerifyCosignSignatureInvalidBase64(t *testing.T) {
	pki := newTestPKI(t, expectedOIDCIssuer, testWorkflowURI)
	withTestRoots(t, pki)

	if err := verifyCosignSignature([]byte("data"), []byte("%%%not-base64"), pki.chainPEM); err == nil {
		t.Error("invalid base64 signature should fail")
	}
}

func TestVerifyCosignSignatureNoCerts(t *testing.T) {
	if err := verifyCosignSignature([]byte("data"), []byte("dGVzdA=="), []byte("not a PEM")); err == nil {
		t.Error("no certificates should fail")
	}
}

func TestParsePEMChain(t *testing.T) {
	pki := newTestPKI(t, expectedOIDCIssuer, testWorkflowURI)

	certs, err := parsePEMChain(pki.chainPEM)
	if err != nil {
		t.Fatal(err)
	}
	if len(certs) != 2 {
		t.Fatalf("expected 2 certs (leaf + intermediate), got %d", len(certs))
	}
}

func TestCertExtension(t *testing.T) {
	pki := newTestPKI(t, "https://test.example.com", testWorkflowURI)

	got := certExtension(pki.leafCert, oidFulcioIssuer)
	if got != "https://test.example.com" {
		t.Errorf("certExtension = %q, want %q", got, "https://test.example.com")
	}

	missing := certExtension(pki.leafCert, asn1.ObjectIdentifier{1, 2, 3, 4, 5})
	if missing != "" {
		t.Errorf("missing OID should return empty, got %q", missing)
	}
}

func TestCheckFulcioIdentity(t *testing.T) {
	good := newTestPKI(t, expectedOIDCIssuer, testWorkflowURI)
	if err := checkFulcioIdentity(good.leafCert); err != nil {
		t.Errorf("valid identity should pass: %v", err)
	}

	badIssuer := newTestPKI(t, "https://evil.example.com", testWorkflowURI)
	if err := checkFulcioIdentity(badIssuer.leafCert); err == nil {
		t.Error("wrong issuer should fail")
	}

	badRepo := newTestPKI(t, expectedOIDCIssuer,
		"https://github.com/evil/repo/.github/workflows/release.yml@refs/tags/v1.0.0")
	if err := checkFulcioIdentity(badRepo.leafCert); err == nil {
		t.Error("wrong repo should fail")
	}
}
