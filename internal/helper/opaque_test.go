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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Opaque Key", func() {
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
})
