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
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Secret Generation", func() {
	Describe("GenerateRandomHex", func() {
		It("should generate a hex string of correct length", func() {
			result, err := GenerateRandomHex(32)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(64))
		})

		It("should generate valid hex characters", func() {
			result, err := GenerateRandomHex(16)
			Expect(err).NotTo(HaveOccurred())
			_, decodeErr := hex.DecodeString(result)
			Expect(decodeErr).NotTo(HaveOccurred())
		})

		It("should generate unique values on successive calls", func() {
			result1, err := GenerateRandomHex(32)
			Expect(err).NotTo(HaveOccurred())
			result2, err := GenerateRandomHex(32)
			Expect(err).NotTo(HaveOccurred())
			Expect(result1).NotTo(Equal(result2))
		})
	})

	Describe("GenerateRandomAlphanumeric", func() {
		It("should generate a string of correct length", func() {
			result, err := GenerateRandomAlphanumeric(20)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(20))
		})

		It("should only contain alphanumeric characters", func() {
			result, err := GenerateRandomAlphanumeric(100)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(MatchRegexp(`^[a-zA-Z0-9]+$`))
		})

		It("should generate unique values on successive calls", func() {
			result1, err := GenerateRandomAlphanumeric(32)
			Expect(err).NotTo(HaveOccurred())
			result2, err := GenerateRandomAlphanumeric(32)
			Expect(err).NotTo(HaveOccurred())
			Expect(result1).NotTo(Equal(result2))
		})
	})

	Describe("GenerateJWTToken", func() {
		var (
			secret   string
			issuedAt time.Time
			expiry   time.Duration
		)

		BeforeEach(func() {
			secret = "test-secret-key-for-jwt-signing"
			issuedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			expiry = 24 * time.Hour * 365 * 10
		})

		It("should produce a valid HS256 JWT", func() {
			tokenStr, err := GenerateJWTToken(secret, "anon", issuedAt, expiry)
			Expect(err).NotTo(HaveOccurred())
			Expect(tokenStr).NotTo(BeEmpty())

			parsed, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				Expect(t.Method.Alg()).To(Equal("HS256"))
				return []byte(secret), nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.Valid).To(BeTrue())
		})

		It("should include correct claims", func() {
			tokenStr, err := GenerateJWTToken(secret, "service_role", issuedAt, expiry)
			Expect(err).NotTo(HaveOccurred())

			parsed, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				return []byte(secret), nil
			})
			Expect(err).NotTo(HaveOccurred())

			claims, ok := parsed.Claims.(jwt.MapClaims)
			Expect(ok).To(BeTrue())
			Expect(claims["role"]).To(Equal("service_role"))
			Expect(claims["iss"]).To(Equal("supabase"))

			iat, err := claims.GetIssuedAt()
			Expect(err).NotTo(HaveOccurred())
			Expect(iat.Unix()).To(Equal(issuedAt.Unix()))

			exp, err := claims.GetExpirationTime()
			Expect(err).NotTo(HaveOccurred())
			Expect(exp.Unix()).To(Equal(issuedAt.Add(expiry).Unix()))
		})
	})

	Describe("GenerateJWTSecretData", func() {
		It("should contain all 11 required keys", func() {
			data, err := GenerateJWTSecretData(time.Now(), 24*time.Hour*365*10)
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(HaveKey("jwt-secret"))
			Expect(data).To(HaveKey("anon-key"))
			Expect(data).To(HaveKey("service-key"))
			Expect(data).To(HaveKey("jwt-keys"))
			Expect(data).To(HaveKey("jwt-jwks"))
			Expect(data).To(HaveKey("anon-key-asymmetric"))
			Expect(data).To(HaveKey("service-key-asymmetric"))
			Expect(data).To(HaveKey("publishable-key"))
			Expect(data).To(HaveKey("secret-key"))
			Expect(data).To(HaveKey("publishable-keys-json"))
			Expect(data).To(HaveKey("secret-keys-json"))
		})

		It("should produce valid jwt-keys and jwt-jwks", func() {
			data, err := GenerateJWTSecretData(time.Now(), 24*time.Hour*365*10)
			Expect(err).NotTo(HaveOccurred())

			var keys []map[string]any
			Expect(json.Unmarshal(data["jwt-keys"], &keys)).To(Succeed())
			Expect(keys).To(HaveLen(2))
			Expect(keys[0]["kty"]).To(Equal("EC"))
			Expect(keys[0]).To(HaveKey("d"))

			var jwks map[string]any
			Expect(json.Unmarshal(data["jwt-jwks"], &jwks)).To(Succeed())
			Expect(jwks).To(HaveKey("keys"))
		})

		It("should produce ES256 JWTs verifiable with the EC public key from JWKS", func() {
			data, err := GenerateJWTSecretData(time.Now(), 24*time.Hour*365*10)
			Expect(err).NotTo(HaveOccurred())

			var jwks map[string]any
			Expect(json.Unmarshal(data["jwt-jwks"], &jwks)).To(Succeed())
			keys := jwks["keys"].([]any)
			ecJWK := keys[0].(map[string]any)

			xBytes, _ := base64.RawURLEncoding.DecodeString(ecJWK["x"].(string))
			yBytes, _ := base64.RawURLEncoding.DecodeString(ecJWK["y"].(string))
			pubKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(xBytes), Y: new(big.Int).SetBytes(yBytes)}

			anonAsym, err := jwt.Parse(string(data["anon-key-asymmetric"]), func(t *jwt.Token) (any, error) {
				return pubKey, nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(anonAsym.Valid).To(BeTrue())
		})
	})

	Describe("GenerateDashboardSecretData", func() {
		It("should contain username and password keys", func() {
			data, err := GenerateDashboardSecretData()
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(HaveKey("username"))
			Expect(data).To(HaveKey("password"))
		})
	})

	Describe("GenerateRealtimeSecretData", func() {
		It("should contain secret-key-base", func() {
			data, err := GenerateRealtimeSecretData()
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(HaveKey("secret-key-base"))
			Expect(data["secret-key-base"]).To(HaveLen(128))
		})
	})

	Describe("GenerateMetaSecretData", func() {
		It("should contain crypto-key", func() {
			data, err := GenerateMetaSecretData()
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(HaveKey("crypto-key"))
			Expect(data["crypto-key"]).To(HaveLen(64))
		})
	})

	Describe("GenerateKeysSecretData", func() {
		It("should include crypto-key and secret-key-base", func() {
			data, err := GenerateKeysSecretData()
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(HaveKey("crypto-key"))
			Expect(data).To(HaveKey("secret-key-base"))
			Expect(data["crypto-key"]).To(HaveLen(64))
			Expect(data["secret-key-base"]).To(HaveLen(128))
		})
	})

	Describe("GenerateStorageS3SecretData", func() {
		It("should contain access-key-id and secret-access-key", func() {
			data, err := GenerateStorageS3SecretData()
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(HaveKey("access-key-id"))
			Expect(data).To(HaveKey("secret-access-key"))
			Expect(data["access-key-id"]).To(HaveLen(20))
			Expect(data["secret-access-key"]).To(HaveLen(40))
		})
	})
})
