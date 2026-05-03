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

package controller

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	alphanumericCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	opaqueKeyProjectRef = "supabase-self-hosted"
)

// GenerateRandomHex generates a cryptographically secure random hex string.
// The numBytes parameter specifies the number of random bytes; the returned
// string will be 2*numBytes characters long.
func GenerateRandomHex(numBytes int) (string, error) {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateRandomAlphanumeric generates a cryptographically secure random
// alphanumeric string of the specified length.
func GenerateRandomAlphanumeric(length int) (string, error) {
	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(alphanumericCharset)))
	for i := range result {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("generating random index: %w", err)
		}
		result[i] = alphanumericCharset[idx.Int64()]
	}
	return string(result), nil
}

// GenerateJWTToken creates an HS256-signed JWT with the given role, secret,
// issued-at time, and expiry duration.
func GenerateJWTToken(secret string, role string, issuedAt time.Time, expiry time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"role": role,
		"iss":  "supabase",
		"iat":  issuedAt.Unix(),
		"exp":  issuedAt.Add(expiry).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("signing JWT for role %q: %w", role, err)
	}
	return signed, nil
}

// SecretData holds the key-value pairs for a single Kubernetes Secret.
type SecretData map[string][]byte

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
func ecPrivateKeyToJWK(key *ecdsa.PrivateKey, kid string) map[string]any {
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
func ecPublicKeyToJWK(key *ecdsa.PrivateKey, kid string) map[string]any {
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
func symmetricKeyToJWK(secret string) map[string]any {
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

// SignES256JWT signs a JWT with ES256 using the given EC private key.
func SignES256JWT(privateKey *ecdsa.PrivateKey, kid string, role string, issuedAt time.Time, expiry time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"role": role,
		"iss":  "supabase",
		"iat":  issuedAt.Unix(),
		"exp":  issuedAt.Add(expiry).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("signing ES256 JWT for role %q: %w", role, err)
	}
	return signed, nil
}

// GenerateOpaqueKey generates an opaque API key with the given prefix.
// Format: <prefix><22-char-random>_<8-char-checksum>
// Checksum: SHA-256 of "supabase-self-hosted|<prefix><random>" truncated to 8 base64url chars.
func GenerateOpaqueKey(prefix string) (string, error) {
	randomBytes := make([]byte, 17)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generating random bytes for opaque key: %w", err)
	}
	random := base64urlEncode(randomBytes)[:22]
	intermediate := prefix + random

	hash := sha256.Sum256([]byte(opaqueKeyProjectRef + "|" + intermediate))
	checksum := base64urlEncode(hash[:])[:8]

	return intermediate + "_" + checksum, nil
}

// GenerateJWTSecretData generates all key material for the JWT secret (11 keys).
func GenerateJWTSecretData(now time.Time, jwtExpiry time.Duration) (SecretData, error) {
	jwtSecretBytes := make([]byte, 30)
	if _, err := rand.Read(jwtSecretBytes); err != nil {
		return nil, fmt.Errorf("generating jwt-secret bytes: %w", err)
	}
	jwtSecret := base64.StdEncoding.EncodeToString(jwtSecretBytes)

	anonKey, err := GenerateJWTToken(jwtSecret, "anon", now, jwtExpiry)
	if err != nil {
		return nil, fmt.Errorf("generating anon-key: %w", err)
	}

	serviceKey, err := GenerateJWTToken(jwtSecret, "service_role", now, jwtExpiry)
	if err != nil {
		return nil, fmt.Errorf("generating service-key: %w", err)
	}

	ecKey, err := GenerateECP256Keypair()
	if err != nil {
		return nil, fmt.Errorf("generating EC P-256 keypair: %w", err)
	}

	kid, err := GenerateRandomHex(16)
	if err != nil {
		return nil, fmt.Errorf("generating kid: %w", err)
	}

	ecPrivateJWK := ecPrivateKeyToJWK(ecKey, kid)
	ecPublicJWK := ecPublicKeyToJWK(ecKey, kid)
	octJWK := symmetricKeyToJWK(jwtSecret)

	jwtKeys, err := BuildJWTKeys(ecPrivateJWK, octJWK)
	if err != nil {
		return nil, fmt.Errorf("building jwt-keys: %w", err)
	}

	jwtJWKS, err := BuildJWTJWKS(ecPublicJWK, octJWK)
	if err != nil {
		return nil, fmt.Errorf("building jwt-jwks: %w", err)
	}

	anonKeyAsym, err := SignES256JWT(ecKey, kid, "anon", now, jwtExpiry)
	if err != nil {
		return nil, fmt.Errorf("generating anon-key-asymmetric: %w", err)
	}

	serviceKeyAsym, err := SignES256JWT(ecKey, kid, "service_role", now, jwtExpiry)
	if err != nil {
		return nil, fmt.Errorf("generating service-key-asymmetric: %w", err)
	}

	publishableKey, err := GenerateOpaqueKey("sb_publishable_")
	if err != nil {
		return nil, fmt.Errorf("generating publishable-key: %w", err)
	}

	secretKey, err := GenerateOpaqueKey("sb_secret_")
	if err != nil {
		return nil, fmt.Errorf("generating secret-key: %w", err)
	}

	publishableKeysJSON, _ := json.Marshal(map[string]string{"default": publishableKey})
	secretKeysJSON, _ := json.Marshal(map[string]string{"default": secretKey})

	return SecretData{
		"jwt-secret":             []byte(jwtSecret),
		"anon-key":               []byte(anonKey),
		"service-key":            []byte(serviceKey),
		"jwt-keys":               []byte(jwtKeys),
		"jwt-jwks":               []byte(jwtJWKS),
		"anon-key-asymmetric":    []byte(anonKeyAsym),
		"service-key-asymmetric": []byte(serviceKeyAsym),
		"publishable-key":        []byte(publishableKey),
		"secret-key":             []byte(secretKey),
		"publishable-keys-json":  publishableKeysJSON,
		"secret-keys-json":       secretKeysJSON,
	}, nil
}

// GenerateDashboardSecretData generates the data for the Dashboard secret.
func GenerateDashboardSecretData() (SecretData, error) {
	password, err := GenerateRandomAlphanumeric(32)
	if err != nil {
		return nil, fmt.Errorf("generating dashboard password: %w", err)
	}

	return SecretData{
		"username": []byte("supabase"),
		"password": []byte(password),
	}, nil
}

// GenerateRealtimeSecretData generates the data for the Realtime secret.
func GenerateRealtimeSecretData() (SecretData, error) {
	secretKeyBase, err := GenerateRandomHex(64)
	if err != nil {
		return nil, fmt.Errorf("generating realtime secret-key-base: %w", err)
	}

	return SecretData{
		"secret-key-base": []byte(secretKeyBase),
	}, nil
}

// GenerateMetaSecretData generates the data for the Meta crypto secret.
func GenerateMetaSecretData() (SecretData, error) {
	cryptoKey, err := GenerateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generating meta crypto-key: %w", err)
	}

	return SecretData{
		"crypto-key": []byte(cryptoKey),
	}, nil
}

// GenerateKeysSecretData generates the shared keys secret data.
// This combines keys previously split across realtime and meta secrets.
func GenerateKeysSecretData() (SecretData, error) {
	realtimeData, err := GenerateRealtimeSecretData()
	if err != nil {
		return nil, err
	}
	metaData, err := GenerateMetaSecretData()
	if err != nil {
		return nil, err
	}

	combined := SecretData{}
	maps.Copy(combined, realtimeData)
	maps.Copy(combined, metaData)
	return combined, nil
}

// GenerateStorageS3SecretData generates the data for the Storage S3 protocol secret.
func GenerateStorageS3SecretData() (SecretData, error) {
	accessKeyID, err := GenerateRandomAlphanumeric(20)
	if err != nil {
		return nil, fmt.Errorf("generating storage access-key-id: %w", err)
	}

	secretAccessKey, err := GenerateRandomAlphanumeric(40)
	if err != nil {
		return nil, fmt.Errorf("generating storage secret-access-key: %w", err)
	}

	return SecretData{
		"access-key-id":     []byte(accessKeyID),
		"secret-access-key": []byte(secretAccessKey),
	}, nil
}

// GenerateSAMLPrivateKeySecretData generates a SAML private key in base64-encoded PKCS#1 DER format.
func GenerateSAMLPrivateKeySecretData() (SecretData, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generating SAML RSA private key: %w", err)
	}

	der := x509.MarshalPKCS1PrivateKey(privateKey)
	encoded := base64.StdEncoding.EncodeToString(der)

	return SecretData{
		"private-key": []byte(encoded),
	}, nil
}
