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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

var _ = Describe("Mutate functions", func() {
	Context("MutateSecret", func() {
		It("should copy missing keys from desired", func() {
			existing := &corev1.Secret{
				Data: map[string][]byte{
					"existing-key": []byte("existing-value"),
				},
			}
			desired := &corev1.Secret{
				Data: map[string][]byte{
					"existing-key": []byte("desired-value"),
					"new-key":      []byte("new-value"),
				},
			}

			err := MutateSecret("existing-key", "new-key")(existing, desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(existing.Data).To(HaveKeyWithValue("existing-key", []byte("existing-value")))
			Expect(existing.Data).To(HaveKeyWithValue("new-key", []byte("new-value")))
		})

		It("should preserve existing keys and not overwrite them", func() {
			existing := &corev1.Secret{
				Data: map[string][]byte{
					"key": []byte("existing-value"),
				},
			}
			desired := &corev1.Secret{
				Data: map[string][]byte{
					"key": []byte("desired-value"),
				},
			}

			err := MutateSecret("key")(existing, desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(existing.Data).To(HaveKeyWithValue("key", []byte("existing-value")))
		})

		It("should ignore keys not listed in the parameter", func() {
			existing := &corev1.Secret{
				Data: map[string][]byte{},
			}
			desired := &corev1.Secret{
				Data: map[string][]byte{
					"key1": []byte("value1"),
					"key2": []byte("value2"),
				},
			}

			err := MutateSecret("key1")(existing, desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(existing.Data).To(HaveKeyWithValue("key1", []byte("value1")))
			Expect(existing.Data).NotTo(HaveKey("key2"))
		})
	})

	Context("MutatePVC", func() {
		It("should copy Resources from desired", func() {
			existing := &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{},
			}
			desired := &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("10Gi"),
						},
					},
				},
			}

			err := MutatePVC()(existing, desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(existing.Spec.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceStorage, resource.MustParse("10Gi")))
		})
	})

	Context("MutateService", func() {
		It("should copy Spec and Labels from desired and merge Annotations", func() {
			existing := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"old-label": "value"},
					Annotations: map[string]string{"old-anno": "value"},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Port: 80}},
				},
			}
			desired := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"new-label": "value"},
					Annotations: map[string]string{"new-anno": "value"},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Port: 8080}},
				},
			}

			err := MutateService()(existing, desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(existing.Spec.Ports[0].Port).To(Equal(int32(8080)))
			Expect(existing.Labels).To(HaveKeyWithValue("new-label", "value"))
			Expect(existing.Labels).NotTo(HaveKey("old-label"))
			Expect(existing.Annotations).To(HaveKeyWithValue("old-anno", "value"))
			Expect(existing.Annotations).To(HaveKeyWithValue("new-anno", "value"))
		})
	})

	Context("MutateConfigMap", func() {
		It("should copy Data and Labels from desired and merge Annotations", func() {
			existing := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"old-label": "value"},
					Annotations: map[string]string{"old-anno": "value"},
				},
				Data: map[string]string{"old-key": "old-value"},
			}
			desired := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"new-label": "value"},
					Annotations: map[string]string{"new-anno": "value"},
				},
				Data: map[string]string{"new-key": "new-value"},
			}

			err := MutateConfigMap()(existing, desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(existing.Data).To(HaveKeyWithValue("new-key", "new-value"))
			Expect(existing.Labels).To(HaveKeyWithValue("new-label", "value"))
			Expect(existing.Labels).NotTo(HaveKey("old-label"))
			Expect(existing.Annotations).To(HaveKeyWithValue("old-anno", "value"))
			Expect(existing.Annotations).To(HaveKeyWithValue("new-anno", "value"))
		})
	})

	Context("MutateStatefulSet", func() {
		It("should copy Spec and Labels from desired and merge Annotations", func() {
			existing := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"old-label": "value"},
					Annotations: map[string]string{"old-anno": "value"},
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: int32Ptr(1),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{Name: "init", Image: "old-init-image"},
							},
							Containers: []corev1.Container{
								{Name: "main", Image: "old-main-image"},
							},
						},
					},
				},
			}
			desired := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"new-label": "value"},
					Annotations: map[string]string{"new-anno": "value"},
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: int32Ptr(3),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{Name: "init", Image: "new-init-image"},
							},
							Containers: []corev1.Container{
								{Name: "main", Image: "new-main-image"},
							},
						},
					},
				},
			}

			err := MutateStatefulSet()(existing, desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(*existing.Spec.Replicas).To(Equal(int32(3)))
			Expect(existing.Labels).To(HaveKeyWithValue("new-label", "value"))
			Expect(existing.Labels).NotTo(HaveKey("old-label"))
			Expect(existing.Annotations).To(HaveKeyWithValue("old-anno", "value"))
			Expect(existing.Annotations).To(HaveKeyWithValue("new-anno", "value"))
			Expect(existing.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(existing.Spec.Template.Spec.InitContainers[0].Image).To(Equal("new-init-image"))
			Expect(existing.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(existing.Spec.Template.Spec.Containers[0].Image).To(Equal("new-main-image"))
		})
	})

	Context("MutateDeployment", func() {
		It("should copy Spec and Labels from desired and merge Annotations", func() {
			existing := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"old-label": "value"},
					Annotations: map[string]string{"old-anno": "value"},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(1),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{Name: "init", Image: "old-init-image"},
							},
							Containers: []corev1.Container{
								{Name: "main", Image: "old-main-image"},
							},
						},
					},
				},
			}
			desired := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"new-label": "value"},
					Annotations: map[string]string{"new-anno": "value"},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(5),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{
								{Name: "init", Image: "new-init-image"},
							},
							Containers: []corev1.Container{
								{Name: "main", Image: "new-main-image"},
							},
						},
					},
				},
			}

			err := MutateDeployment()(existing, desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(*existing.Spec.Replicas).To(Equal(int32(5)))
			Expect(existing.Labels).To(HaveKeyWithValue("new-label", "value"))
			Expect(existing.Labels).NotTo(HaveKey("old-label"))
			Expect(existing.Annotations).To(HaveKeyWithValue("old-anno", "value"))
			Expect(existing.Annotations).To(HaveKeyWithValue("new-anno", "value"))
			Expect(existing.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(existing.Spec.Template.Spec.InitContainers[0].Image).To(Equal("new-init-image"))
			Expect(existing.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(existing.Spec.Template.Spec.Containers[0].Image).To(Equal("new-main-image"))
		})
	})

	Context("MutateJob", func() {
		It("should copy Spec and Labels from desired and merge Annotations while preserving the selector and generated template labels", func() {
			existing := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"old-label": "value"},
					Annotations: map[string]string{"old-anno": "value"},
				},
				Spec: batchv1.JobSpec{
					Parallelism: int32Ptr(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"generated": "selector"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"controller-uid":                     "abc-123",
								"job-name":                           "my-job",
								"batch.kubernetes.io/controller-uid": "abc-123",
								"batch.kubernetes.io/job-name":       "my-job",
							},
						},
					},
				},
			}
			desired := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"new-label": "value"},
					Annotations: map[string]string{"new-anno": "value"},
				},
				Spec: batchv1.JobSpec{
					Parallelism: int32Ptr(3),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/component": "migration",
							},
						},
					},
				},
			}

			err := MutateJob()(existing, desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(*existing.Spec.Parallelism).To(Equal(int32(3)))
			Expect(existing.Labels).To(HaveKeyWithValue("new-label", "value"))
			Expect(existing.Labels).NotTo(HaveKey("old-label"))
			Expect(existing.Annotations).To(HaveKeyWithValue("old-anno", "value"))
			Expect(existing.Annotations).To(HaveKeyWithValue("new-anno", "value"))
			Expect(existing.Spec.Selector).NotTo(BeNil())
			Expect(existing.Spec.Selector.MatchLabels).To(HaveKeyWithValue("generated", "selector"))
			Expect(existing.Spec.Template.Labels).To(HaveKeyWithValue("controller-uid", "abc-123"))
			Expect(existing.Spec.Template.Labels).To(HaveKeyWithValue("job-name", "my-job"))
			Expect(existing.Spec.Template.Labels).To(HaveKeyWithValue("batch.kubernetes.io/controller-uid", "abc-123"))
			Expect(existing.Spec.Template.Labels).To(HaveKeyWithValue("batch.kubernetes.io/job-name", "my-job"))
			Expect(existing.Spec.Template.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "migration"))
		})
	})

	Context("MutateMigration", func() {
		It("should copy Spec and Labels from desired and merge Annotations", func() {
			existing := &supabasev1alpha1.Migration{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"old-label": "value"},
					Annotations: map[string]string{"old-anno": "value", "shared-anno": "existing"},
				},
				Spec: supabasev1alpha1.MigrationSpec{
					DatabaseRef: supabasev1alpha1.DatabaseRef{
						Kind: "SingleDatabase",
						Name: "old-db",
					},
				},
			}
			desired := &supabasev1alpha1.Migration{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"new-label": "value"},
					Annotations: map[string]string{"new-anno": "value", "shared-anno": "desired"},
				},
				Spec: supabasev1alpha1.MigrationSpec{
					DatabaseRef: supabasev1alpha1.DatabaseRef{
						Kind: "SingleDatabase",
						Name: "new-db",
					},
				},
			}

			err := MutateMigration()(existing, desired)
			Expect(err).NotTo(HaveOccurred())
			Expect(existing.Spec.DatabaseRef.Name).To(Equal("new-db"))
			Expect(existing.Labels).To(HaveKeyWithValue("new-label", "value"))
			Expect(existing.Labels).NotTo(HaveKey("old-label"))
			Expect(existing.Annotations).To(HaveKeyWithValue("old-anno", "value"))
			Expect(existing.Annotations).To(HaveKeyWithValue("new-anno", "value"))
			Expect(existing.Annotations).To(HaveKeyWithValue("shared-anno", "desired"))
		})
	})

	Context("mergeStringMaps", func() {
		It("should preserve existing keys when desired has no annotations", func() {
			existing := map[string]string{"existing-anno": "yes"}
			merged := mergeStringMaps(existing, nil)
			Expect(merged).To(HaveKeyWithValue("existing-anno", "yes"))
		})

		It("should return nil when both maps are nil", func() {
			merged := mergeStringMaps(nil, nil)
			Expect(merged).To(BeNil())
		})
	})
})

func int32Ptr(i int32) *int32 {
	return &i
}
