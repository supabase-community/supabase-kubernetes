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
	"crypto/x509"
	"encoding/base64"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Crypto", func() {
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
