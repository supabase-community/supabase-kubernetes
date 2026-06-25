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
	"encoding/hex"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Random Generation", func() {
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
})
