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
	"crypto/elliptic"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("JWK", func() {
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

			privateJWK := ECPrivateKeyToJWK(key, kid)
			octJWK := SymmetricKeyToJWK("test-secret")

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

			publicJWK := ECPublicKeyToJWK(key, kid)
			octJWK := SymmetricKeyToJWK("test-secret")

			result, err := BuildJWTJWKS(publicJWK, octJWK)
			Expect(err).NotTo(HaveOccurred())

			var jwks map[string]any
			Expect(json.Unmarshal([]byte(result), &jwks)).To(Succeed())
			Expect(jwks).To(HaveKey("keys"))
		})
	})
})
