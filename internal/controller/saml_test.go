package controller

import (
	"crypto/x509"
	"encoding/base64"
	"testing"
)

func TestGenerateSAMLSecretData(t *testing.T) {
	data, err := GenerateSAMLSecretData()
	if err != nil {
		t.Fatalf("GenerateSAMLSecretData failed: %v", err)
	}
	encoded, ok := data["private-key"]
	if !ok {
		t.Fatal("missing private-key in secret data")
	}
	decoded, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}
	key, err := x509.ParsePKCS1PrivateKey(decoded)
	if err != nil {
		t.Fatalf("failed to parse PKCS1 private key: %v", err)
	}
	if key.N.BitLen() < 2048 {
		t.Fatalf("key too short: %d bits", key.N.BitLen())
	}
}
