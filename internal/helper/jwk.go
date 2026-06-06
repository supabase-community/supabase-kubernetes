/*
Copyright 2026.

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

package helper

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// GenerateECP256Keypair generates a new ECDSA P-256 private key.
func GenerateECP256Keypair() (*ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating EC P-256 keypair: %w", err)
	}
	return key, nil
}

// base64urlEncode encodes bytes to base64url without padding (RFC 7515).
func base64urlEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// ecPrivateKeyToJWK converts an EC P-256 private key to a JWK map (with "d" parameter).
func ECPrivateKeyToJWK(key *ecdsa.PrivateKey, kid string) map[string]any {
	xBytes := key.X.FillBytes(make([]byte, 32))
	yBytes := key.Y.FillBytes(make([]byte, 32))
	dBytes := key.D.FillBytes(make([]byte, 32))

	return map[string]any{
		"kty":     "EC",
		"kid":     kid,
		"use":     "sig",
		"key_ops": []string{"sign", "verify"},
		"alg":     "ES256",
		"ext":     true,
		"crv":     "P-256",
		"x":       base64urlEncode(xBytes),
		"y":       base64urlEncode(yBytes),
		"d":       base64urlEncode(dBytes),
	}
}

// ecPublicKeyToJWK converts an EC P-256 public key to a JWK map (no "d" parameter).
func ECPublicKeyToJWK(key *ecdsa.PrivateKey, kid string) map[string]any {
	xBytes := key.X.FillBytes(make([]byte, 32))
	yBytes := key.Y.FillBytes(make([]byte, 32))

	return map[string]any{
		"kty":     "EC",
		"kid":     kid,
		"use":     "sig",
		"key_ops": []string{"verify"},
		"alg":     "ES256",
		"ext":     true,
		"crv":     "P-256",
		"x":       base64urlEncode(xBytes),
		"y":       base64urlEncode(yBytes),
	}
}

// symmetricKeyToJWK converts a symmetric secret to an "oct" JWK.
func SymmetricKeyToJWK(secret string) map[string]any {
	return map[string]any{
		"kty": "oct",
		"k":   base64urlEncode([]byte(secret)),
		"alg": "HS256",
	}
}

// BuildJWTKeys constructs the JSON array of JWKs for Auth to sign tokens.
// Contains: EC private key + symmetric key.
func BuildJWTKeys(ecPrivateJWK, octJWK map[string]any) (string, error) {
	keys := []map[string]any{ecPrivateJWK, octJWK}
	data, err := json.Marshal(keys)
	if err != nil {
		return "", fmt.Errorf("marshaling JWT_KEYS: %w", err)
	}
	return string(data), nil
}

// BuildJWTJWKS constructs the JWKS object for services to verify tokens.
// Contains: EC public key + symmetric key.
func BuildJWTJWKS(ecPublicJWK, octJWK map[string]any) (string, error) {
	jwks := map[string]any{
		"keys": []map[string]any{ecPublicJWK, octJWK},
	}
	data, err := json.Marshal(jwks)
	if err != nil {
		return "", fmt.Errorf("marshaling JWT_JWKS: %w", err)
	}
	return string(data), nil
}
