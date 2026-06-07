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

package reconciler

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("EnsureResource", func() {
	var (
		ctx context.Context
		c   client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		c = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	})

	Context("when the resource does not exist", func() {
		It("should create the resource", func() {
			desired := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{"key": "value"},
			}

			obj, res, err := EnsureResource(ctx, c, desired, nil, func(existing, desired *corev1.ConfigMap) error {
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(ResultCreated))
			Expect(obj).NotTo(BeNil())

			found := &corev1.ConfigMap{}
			Expect(c.Get(ctx, client.ObjectKey{Name: "test-cm", Namespace: "default"}, found)).To(Succeed())
			Expect(found.Data).To(HaveKeyWithValue("key", "value"))
		})

		It("should create a Service with correct spec", func() {
			desired := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Port: 80}},
				},
			}

			obj, res, err := EnsureResource(ctx, c, desired, nil, func(existing, desired *corev1.Service) error {
				if existing.Spec.Ports == nil {
					existing.Spec.Ports = []corev1.ServicePort{{Port: 80}}
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(ResultCreated))
			Expect(obj).NotTo(BeNil())

			found := &corev1.Service{}
			Expect(c.Get(ctx, client.ObjectKey{Name: "test-svc", Namespace: "default"}, found)).To(Succeed())
			Expect(found.Spec.Ports).To(HaveLen(1))
			Expect(found.Spec.Ports[0].Port).To(Equal(int32(80)))
		})
	})

	Context("when the resource already exists", func() {
		It("should update the resource when data differs", func() {
			existing := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{"key": "old-value"},
			}
			Expect(c.Create(ctx, existing)).To(Succeed())

			desired := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{"key": "new-value"},
			}

			obj, res, err := EnsureResource(ctx, c, desired, nil, func(existing, desired *corev1.ConfigMap) error {
				existing.Data = desired.Data
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(ResultUpdated))
			Expect(obj).NotTo(BeNil())

			found := &corev1.ConfigMap{}
			Expect(c.Get(ctx, client.ObjectKey{Name: "test-cm", Namespace: "default"}, found)).To(Succeed())
			Expect(found.Data).To(HaveKeyWithValue("key", "new-value"))
		})

		It("should return unchanged when data is identical", func() {
			existing := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{"key": "value"},
			}
			Expect(c.Create(ctx, existing)).To(Succeed())

			desired := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{"key": "value"},
			}

			obj, res, err := EnsureResource(ctx, c, desired, nil, func(existing, desired *corev1.ConfigMap) error {
				existing.Data = desired.Data
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(ResultUnchanged))
			Expect(obj).NotTo(BeNil())
		})

		It("should pass existing data to mutateFn", func() {
			existing := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
					Labels:    map[string]string{"existing-label": "yes"},
				},
				Data: map[string]string{"key": "old"},
			}
			Expect(c.Create(ctx, existing)).To(Succeed())

			desired := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
			}

			var receivedLabels map[string]string
			var receivedData map[string]string

			obj, _, err := EnsureResource(ctx, c, desired, nil, func(existing, desired *corev1.ConfigMap) error {
				receivedLabels = existing.Labels
				receivedData = existing.Data
				existing.Data = map[string]string{"key": "new"}
				return nil
			})
			Expect(obj).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(receivedLabels).To(HaveKeyWithValue("existing-label", "yes"))
			Expect(receivedData).To(HaveKeyWithValue("key", "old"))
		})

		It("should preserve existing annotations unless explicitly changed", func() {
			existing := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-cm",
					Namespace:   "default",
					Annotations: map[string]string{"existing-anno": "yes"},
				},
				Data: map[string]string{"key": "old"},
			}
			Expect(c.Create(ctx, existing)).To(Succeed())

			desired := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "default",
				},
				Data: map[string]string{"key": "new"},
			}

			obj, _, err := EnsureResource(ctx, c, desired, nil, func(existing, desired *corev1.ConfigMap) error {
				existing.Data = desired.Data
				return nil
			})
			Expect(obj).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			found := &corev1.ConfigMap{}
			Expect(c.Get(ctx, client.ObjectKey{Name: "test-cm", Namespace: "default"}, found)).To(Succeed())
			Expect(found.Annotations).To(HaveKeyWithValue("existing-anno", "yes"))
			Expect(found.Data).To(HaveKeyWithValue("key", "new"))
		})
	})

	Context("with owner references", func() {
		It("should set owner reference when owner is provided", func() {
			owner := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "owner",
					Namespace: "default",
					UID:       "owner-uid",
				},
			}

			desired := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "child",
					Namespace: "default",
				},
				Data: map[string]string{"key": "value"},
			}

			obj, _, err := EnsureResource(ctx, c, desired, owner, func(existing, desired *corev1.ConfigMap) error {
				return nil
			})
			Expect(obj).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			found := &corev1.ConfigMap{}
			Expect(c.Get(ctx, client.ObjectKey{Name: "child", Namespace: "default"}, found)).To(Succeed())
			Expect(found.OwnerReferences).To(HaveLen(1))
			Expect(found.OwnerReferences[0].UID).To(Equal(types.UID("owner-uid")))
		})

		It("should not set owner reference when owner is nil", func() {
			desired := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "orphan",
					Namespace: "default",
				},
				Data: map[string]string{"key": "value"},
			}

			obj, _, err := EnsureResource(ctx, c, desired, nil, func(existing, desired *corev1.ConfigMap) error {
				return nil
			})
			Expect(obj).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			found := &corev1.ConfigMap{}
			Expect(c.Get(ctx, client.ObjectKey{Name: "orphan", Namespace: "default"}, found)).To(Succeed())
			Expect(found.OwnerReferences).To(BeEmpty())
		})
	})

	Context("error handling", func() {
		It("should return error for invalid owner reference", func() {
			owner := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "owner",
					Namespace: "other-ns",
					UID:       "owner-uid",
				},
			}

			desired := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "child",
					Namespace: "default",
				},
			}

			obj, _, err := EnsureResource(ctx, c, desired, owner, func(existing, desired *corev1.ConfigMap) error {
				return nil
			})
			Expect(obj).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})
})
