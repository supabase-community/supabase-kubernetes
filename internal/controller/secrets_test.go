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
	"crypto/x509"
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

		It("should return empty string when length is 0", func() {
			result, err := GenerateRandomHex(0)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
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

		It("should return empty string when length is 0", func() {
			result, err := GenerateRandomAlphanumeric(0)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
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

	Describe("GenerateOpaqueKey", func() {
		It("should produce the correct format", func() {
			key, err := GenerateOpaqueKey("sb_test_")
			Expect(err).NotTo(HaveOccurred())
			Expect(key).To(MatchRegexp(`^sb_test_[A-Za-z0-9_-]{22}_[A-Za-z0-9_-]{8}$`))
		})

		It("should generate unique values on successive calls", func() {
			key1, err := GenerateOpaqueKey("sb_test_")
			Expect(err).NotTo(HaveOccurred())
			key2, err := GenerateOpaqueKey("sb_test_")
			Expect(err).NotTo(HaveOccurred())
			Expect(key1).NotTo(Equal(key2))
		})

		It("should use the provided prefix", func() {
			key, err := GenerateOpaqueKey("custom_prefix_")
			Expect(err).NotTo(HaveOccurred())
			Expect(key).To(HavePrefix("custom_prefix_"))
		})
	})

	Describe("GenerateECP256Keypair", func() {
		It("should return a non-nil key with filled coordinates", func() {
			key, err := GenerateECP256Keypair()
			Expect(err).NotTo(HaveOccurred())
			Expect(key).NotTo(BeNil())
			Expect(key.X).NotTo(BeNil())
			Expect(key.Y).NotTo(BeNil())
			Expect(key.D).NotTo(BeNil())
		})

		It("should use the P-256 curve", func() {
			key, err := GenerateECP256Keypair()
			Expect(err).NotTo(HaveOccurred())
			Expect(key.Curve).To(Equal(elliptic.P256()))
		})
	})

	Describe("BuildJWTKeys", func() {
		It("should return a JSON array of 2 keys", func() {
			key, err := GenerateECP256Keypair()
			Expect(err).NotTo(HaveOccurred())
			kid, err := GenerateRandomHex(16)
			Expect(err).NotTo(HaveOccurred())

			privateJWK := ecPrivateKeyToJWK(key, kid)
			octJWK := symmetricKeyToJWK("test-secret")

			result, err := BuildJWTKeys(privateJWK, octJWK)
			Expect(err).NotTo(HaveOccurred())

			var keys []map[string]any
			Expect(json.Unmarshal([]byte(result), &keys)).To(Succeed())
			Expect(keys).To(HaveLen(2))
		})
	})

	Describe("BuildJWTJWKS", func() {
		It("should return a JSON object with a keys array of 2 elements", func() {
			key, err := GenerateECP256Keypair()
			Expect(err).NotTo(HaveOccurred())
			kid, err := GenerateRandomHex(16)
			Expect(err).NotTo(HaveOccurred())

			publicJWK := ecPublicKeyToJWK(key, kid)
			octJWK := symmetricKeyToJWK("test-secret")

			result, err := BuildJWTJWKS(publicJWK, octJWK)
			Expect(err).NotTo(HaveOccurred())

			var jwks map[string]any
			Expect(json.Unmarshal([]byte(result), &jwks)).To(Succeed())
			Expect(jwks).To(HaveKey("keys"))
		})
	})

	Describe("SignES256JWT", func() {
		var (
			key      *ecdsa.PrivateKey
			kid      string
			issuedAt time.Time
			expiry   time.Duration
		)

		BeforeEach(func() {
			var err error
			key, err = GenerateECP256Keypair()
			Expect(err).NotTo(HaveOccurred())
			kid, err = GenerateRandomHex(16)
			Expect(err).NotTo(HaveOccurred())
			issuedAt = time.Now()
			expiry = 24 * time.Hour * 365 * 10
		})

		It("should produce a valid ES256 JWT", func() {
			tokenStr, err := SignES256JWT(key, kid, "anon", issuedAt, expiry)
			Expect(err).NotTo(HaveOccurred())

			parsed, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				return &key.PublicKey, nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.Valid).To(BeTrue())
		})

		It("should include the kid in the token header", func() {
			tokenStr, err := SignES256JWT(key, kid, "anon", issuedAt, expiry)
			Expect(err).NotTo(HaveOccurred())

			parsed, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				return &key.PublicKey, nil
			}, jwt.WithValidMethods([]string{"ES256"}))
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.Header["kid"]).To(Equal(kid))
		})

		It("should include correct claims", func() {
			tokenStr, err := SignES256JWT(key, kid, "service_role", issuedAt, expiry)
			Expect(err).NotTo(HaveOccurred())

			parsed, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				return &key.PublicKey, nil
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
		var data SecretData

		BeforeEach(func() {
			var err error
			data, err = GenerateJWTSecretData(time.Now(), 24*time.Hour*365*10)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should contain jwt-secret as valid base64", func() {
			Expect(data).To(HaveKey("jwt-secret"))
			decoded, err := base64.StdEncoding.DecodeString(string(data["jwt-secret"]))
			Expect(err).NotTo(HaveOccurred())
			Expect(decoded).To(HaveLen(30))
		})

		It("should contain a valid anon-key HS256 JWT", func() {
			Expect(data).To(HaveKey("anon-key"))
			parsed, err := jwt.Parse(string(data["anon-key"]), func(t *jwt.Token) (any, error) {
				return data["jwt-secret"], nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.Valid).To(BeTrue())
		})

		It("should contain a valid service-key HS256 JWT", func() {
			Expect(data).To(HaveKey("service-key"))
			parsed, err := jwt.Parse(string(data["service-key"]), func(t *jwt.Token) (any, error) {
				return data["jwt-secret"], nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.Valid).To(BeTrue())
		})

		It("should contain jwt-keys as a JSON array with EC private key", func() {
			Expect(data).To(HaveKey("jwt-keys"))
			var keys []map[string]any
			Expect(json.Unmarshal(data["jwt-keys"], &keys)).To(Succeed())
			Expect(keys).To(HaveLen(2))
			Expect(keys[0]["kty"]).To(Equal("EC"))
			Expect(keys[0]).To(HaveKey("d"))
		})

		It("should contain jwt-jwks as a JSON object with keys array", func() {
			Expect(data).To(HaveKey("jwt-jwks"))
			var jwks map[string]any
			Expect(json.Unmarshal(data["jwt-jwks"], &jwks)).To(Succeed())
			Expect(jwks).To(HaveKey("keys"))
		})

		It("should contain a valid anon-key-asymmetric ES256 JWT", func() {
			Expect(data).To(HaveKey("anon-key-asymmetric"))
			Expect(data).To(HaveKey("jwt-jwks"))

			var jwks map[string]any
			Expect(json.Unmarshal(data["jwt-jwks"], &jwks)).To(Succeed())
			keys := jwks["keys"].([]any)
			ecJWK := keys[0].(map[string]any)

			xBytes, _ := base64.RawURLEncoding.DecodeString(ecJWK["x"].(string))
			yBytes, _ := base64.RawURLEncoding.DecodeString(ecJWK["y"].(string))
			pubKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(xBytes), Y: new(big.Int).SetBytes(yBytes)}

			parsed, err := jwt.Parse(string(data["anon-key-asymmetric"]), func(t *jwt.Token) (any, error) {
				return pubKey, nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.Valid).To(BeTrue())
		})

		It("should contain a valid service-key-asymmetric ES256 JWT", func() {
			Expect(data).To(HaveKey("service-key-asymmetric"))
			Expect(data).To(HaveKey("jwt-jwks"))

			var jwks map[string]any
			Expect(json.Unmarshal(data["jwt-jwks"], &jwks)).To(Succeed())
			keys := jwks["keys"].([]any)
			ecJWK := keys[0].(map[string]any)

			xBytes, _ := base64.RawURLEncoding.DecodeString(ecJWK["x"].(string))
			yBytes, _ := base64.RawURLEncoding.DecodeString(ecJWK["y"].(string))
			pubKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(xBytes), Y: new(big.Int).SetBytes(yBytes)}

			parsed, err := jwt.Parse(string(data["service-key-asymmetric"]), func(t *jwt.Token) (any, error) {
				return pubKey, nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.Valid).To(BeTrue())
		})

		It("should contain publishable-key with opaque format", func() {
			Expect(data).To(HaveKey("publishable-key"))
			Expect(string(data["publishable-key"])).To(MatchRegexp(`^sb_publishable_[A-Za-z0-9_-]{22}_[A-Za-z0-9_-]{8}$`))
		})

		It("should contain secret-key with opaque format", func() {
			Expect(data).To(HaveKey("secret-key"))
			Expect(string(data["secret-key"])).To(MatchRegexp(`^sb_secret_[A-Za-z0-9_-]{22}_[A-Za-z0-9_-]{8}$`))
		})
	})

	Describe("GenerateKeysSecretData", func() {
		var data SecretData

		BeforeEach(func() {
			var err error
			data, err = GenerateKeysSecretData()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should contain secret-key-base as valid hex of 128 chars", func() {
			Expect(data).To(HaveKey("secret-key-base"))
			Expect(data["secret-key-base"]).To(HaveLen(128))
			_, err := hex.DecodeString(string(data["secret-key-base"]))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should contain crypto-key as valid hex of 64 chars", func() {
			Expect(data).To(HaveKey("crypto-key"))
			Expect(data["crypto-key"]).To(HaveLen(64))
			_, err := hex.DecodeString(string(data["crypto-key"]))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should contain vault-enc-key as valid hex of 32 chars", func() {
			Expect(data).To(HaveKey("vault-enc-key"))
			Expect(data["vault-enc-key"]).To(HaveLen(32))
			_, err := hex.DecodeString(string(data["vault-enc-key"]))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should contain saml-private-key", func() {
			Expect(data).To(HaveKey("saml-private-key"))
			Expect(data["saml-private-key"]).NotTo(BeEmpty())
		})
	})

	Describe("GenerateSAMLPrivateKey", func() {
		It("should return a non-empty base64 string", func() {
			encoded, err := GenerateSAMLPrivateKey()
			Expect(err).NotTo(HaveOccurred())
			Expect(encoded).NotTo(BeEmpty())
		})

		It("should return a parseable RSA 2048+ bit key", func() {
			encoded, err := GenerateSAMLPrivateKey()
			Expect(err).NotTo(HaveOccurred())

			decoded, err := base64.StdEncoding.DecodeString(encoded)
			Expect(err).NotTo(HaveOccurred())
			key, err := x509.ParsePKCS1PrivateKey(decoded)
			Expect(err).NotTo(HaveOccurred())
			Expect(key.N.BitLen()).To(BeNumerically(">=", 2048))
		})
	})
})
