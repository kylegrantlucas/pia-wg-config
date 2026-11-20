package pia

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"strings"
	"testing"
)

// TestEmbeddedCACert pins the embedded PIA CA so it cannot be swapped silently.
func TestEmbeddedCACert(t *testing.T) {
	const wantSHA256 = "1fd25658456eab3041fba77ccd398ab8124edcc1b8b2fc1d55fdf6b1bbfc9d70"

	block, rest := pem.Decode(piaCACert)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("embedded CA is not a PEM certificate")
	}
	if extra, _ := pem.Decode(rest); extra != nil {
		t.Fatal("embedded CA contains more than one PEM block")
	}

	sum := sha256.Sum256(block.Bytes)
	if got := hex.EncodeToString(sum[:]); got != wantSHA256 {
		t.Fatalf("embedded CA SHA-256 = %s, want %s", got, wantSHA256)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse embedded CA: %v", err)
	}
	if !cert.IsCA {
		t.Fatal("embedded certificate is not a CA")
	}
}

// TestExecutePIARequestErrorOmitsToken checks that a failed addKey request
// does not leak the token (carried in the URL query) through the error.
func TestExecutePIARequestErrorOmitsToken(t *testing.T) {
	const token = "SECRET-TOKEN-VALUE"
	p := &PIAClient{}
	server := Server{Cn: "Server-1-0a", IP: "127.0.0.1"}

	// Port 1 on localhost is closed, so the dial fails.
	resp, err := p.executePIARequest(server, "https://Server-1-0a:1/addKey?pt="+token+"&pubkey=x")
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error leaks the token: %v", err)
	}
	if !strings.Contains(err.Error(), server.Cn) {
		t.Fatalf("error should name the server: %v", err)
	}
}
