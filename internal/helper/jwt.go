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
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

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
