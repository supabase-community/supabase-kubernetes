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

	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("MergeEnvVars", func() {
	It("returns base unchanged when overlay is empty", func() {
		base := []corev1.EnvVar{{Name: "A", Value: "1"}}
		Expect(MergeEnvVars(base, nil)).To(Equal(base))
	})

	It("appends overlay entries to base", func() {
		base := []corev1.EnvVar{{Name: "A", Value: "1"}}
		overlay := []corev1.EnvVar{{Name: "B", Value: "2"}}
		merged := MergeEnvVars(base, overlay)
		Expect(merged).To(Equal([]corev1.EnvVar{
			{Name: "A", Value: "1"},
			{Name: "B", Value: "2"},
		}))
	})

	It("overrides base entries by name", func() {
		base := []corev1.EnvVar{{Name: "A", Value: "1"}, {Name: "B", Value: "2"}}
		overlay := []corev1.EnvVar{{Name: "A", Value: "override"}}
		merged := MergeEnvVars(base, overlay)
		Expect(merged).To(Equal([]corev1.EnvVar{
			{Name: "A", Value: "override"},
			{Name: "B", Value: "2"},
		}))
	})

	It("preserves ValueFrom from overlay", func() {
		base := []corev1.EnvVar{{Name: "A", Value: "1"}}
		overlay := []corev1.EnvVar{
			{
				Name: "A",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "secret"},
						Key:                  "key",
					},
				},
			},
		}
		merged := MergeEnvVars(base, overlay)
		Expect(merged).To(HaveLen(1))
		Expect(merged[0].Value).To(BeEmpty())
		Expect(merged[0].ValueFrom.SecretKeyRef.Name).To(Equal("secret"))
		Expect(merged[0].ValueFrom.SecretKeyRef.Key).To(Equal("key"))
	})

	It("returns only overlay when base is empty", func() {
		overlay := []corev1.EnvVar{{Name: "A", Value: "1"}}
		Expect(MergeEnvVars(nil, overlay)).To(Equal(overlay))
	})
})
