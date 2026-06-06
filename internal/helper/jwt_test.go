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
	"time"

	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("JWT", func() {
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
})
