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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

var _ = Describe("SingleDatabase Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-single-db"
		const timeout = 30 * time.Second
		const interval = 250 * time.Millisecond

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			singleDB := &platformv1alpha1.SingleDatabase{}
			err := k8sClient.Get(ctx, typeNamespacedName, singleDB)
			if err == nil {
				return
			}
			resource := &platformv1alpha1.SingleDatabase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: platformv1alpha1.SingleDatabaseSpec{
					Image: "supabase/postgres:17.6.1.084",
					Storage: platformv1alpha1.VolumeClaimTemplateSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &platformv1alpha1.SingleDatabase{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should set Ready condition and create dependent resources", func() {
			singleDB := &platformv1alpha1.SingleDatabase{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, singleDB)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(singleDB.Status.Conditions, ConditionTypeReady)).To(BeTrue())
				g.Expect(singleDB.Status.ServiceName).To(Equal("test-single-db-db"))
				g.Expect(singleDB.Status.SecretName).To(Equal("test-single-db-credentials"))
			}, timeout, interval).Should(Succeed())
		})

		It("should add owner reference when pvcDeletionPolicy changes to Delete", func() {
			singleDB := &platformv1alpha1.SingleDatabase{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, singleDB)).To(Succeed())
			singleDB.Spec.PVCDeletionPolicy = platformv1alpha1.PVCDeletionPolicyRetain
			Expect(k8sClient.Update(ctx, singleDB)).To(Succeed())

			pvc := &corev1.PersistentVolumeClaim{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db-db-data", Namespace: "default"}, pvc)).To(Succeed())
				g.Expect(pvc.OwnerReferences).To(BeEmpty())
			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Get(ctx, typeNamespacedName, singleDB)).To(Succeed())
			singleDB.Spec.PVCDeletionPolicy = platformv1alpha1.PVCDeletionPolicyDelete
			Expect(k8sClient.Update(ctx, singleDB)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db-db-data", Namespace: "default"}, pvc)).To(Succeed())
				g.Expect(pvc.OwnerReferences).To(HaveLen(1))
				g.Expect(pvc.OwnerReferences[0].UID).To(Equal(singleDB.UID))
			}, timeout, interval).Should(Succeed())
		})

		It("should remove owner reference when pvcDeletionPolicy changes to Retain", func() {
			pvc := &corev1.PersistentVolumeClaim{}
			singleDB := &platformv1alpha1.SingleDatabase{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db-db-data", Namespace: "default"}, pvc)).To(Succeed())
				g.Expect(pvc.OwnerReferences).To(HaveLen(1))
			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Get(ctx, typeNamespacedName, singleDB)).To(Succeed())
			singleDB.Spec.PVCDeletionPolicy = platformv1alpha1.PVCDeletionPolicyRetain
			Expect(k8sClient.Update(ctx, singleDB)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db-db-data", Namespace: "default"}, pvc)).To(Succeed())
				g.Expect(pvc.OwnerReferences).To(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When reconciling with advanced configuration", func() {
		const resourceName = "test-single-db-advanced"
		const timeout = 30 * time.Second
		const interval = 250 * time.Millisecond

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			singleDB := &platformv1alpha1.SingleDatabase{}
			err := k8sClient.Get(ctx, typeNamespacedName, singleDB)
			if err == nil {
				return
			}
			resource := &platformv1alpha1.SingleDatabase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: platformv1alpha1.SingleDatabaseSpec{
					Image: "supabase/postgres:17.6.1.084",
					Storage: platformv1alpha1.VolumeClaimTemplateSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "dedicated",
							Operator: corev1.TolerationOpEqual,
							Value:    "postgres",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					PodAnnotations: map[string]string{
						"prometheus.io/scrape": "true",
					},
					PodLabels: map[string]string{
						"tier": "database",
					},
					ImagePullPolicy:               corev1.PullIfNotPresent,
					TerminationGracePeriodSeconds: func() *int64 { v := int64(120); return &v }(),
					ContainerSecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: func() *bool { v := false; return &v }(),
						ReadOnlyRootFilesystem:   func() *bool { v := false; return &v }(),
					},
					PodSecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: func() *bool { v := true; return &v }(),
						RunAsUser:    func() *int64 { v := int64(999); return &v }(),
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &platformv1alpha1.SingleDatabase{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should apply scheduling controls, security context and pod customization", func() {
			sts := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db-advanced-db", Namespace: "default"}, sts)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			podSpec := sts.Spec.Template.Spec
			container := podSpec.Containers[0]

			Expect(podSpec.NodeSelector).To(Equal(map[string]string{"kubernetes.io/os": "linux"}))
			Expect(podSpec.Tolerations).To(HaveLen(1))
			Expect(podSpec.Tolerations[0].Key).To(Equal("dedicated"))
			Expect(podSpec.PriorityClassName).To(BeEmpty())
			Expect(container.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			Expect(podSpec.TerminationGracePeriodSeconds).To(HaveValue(BeEquivalentTo(120)))

			Expect(sts.Spec.Template.Annotations).To(HaveKeyWithValue("prometheus.io/scrape", "true"))
			Expect(sts.Spec.Template.Labels).To(HaveKeyWithValue("tier", "database"))

			Expect(podSpec.SecurityContext).NotTo(BeNil())
			Expect(podSpec.SecurityContext.RunAsNonRoot).To(HaveValue(BeTrue()))
			Expect(podSpec.SecurityContext.RunAsUser).To(HaveValue(BeEquivalentTo(999)))

			Expect(container.SecurityContext).NotTo(BeNil())
			Expect(container.SecurityContext.AllowPrivilegeEscalation).To(HaveValue(BeFalse()))
		})

		It("should apply default probes when not specified", func() {
			sts := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db-advanced-db", Namespace: "default"}, sts)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			container := sts.Spec.Template.Spec.Containers[0]
			Expect(container.StartupProbe).NotTo(BeNil())
			Expect(container.ReadinessProbe).NotTo(BeNil())
			Expect(container.LivenessProbe).NotTo(BeNil())
			Expect(container.StartupProbe.Exec.Command).To(Equal([]string{"pg_isready", "-U", "postgres"}))
		})

		It("should populate enriched status", func() {
			singleDB := &platformv1alpha1.SingleDatabase{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, singleDB)).To(Succeed())
				g.Expect(singleDB.Status.Phase).To(Equal("Running"))
				g.Expect(singleDB.Status.Storage).To(Equal("1Gi"))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When managing password synchronization", func() {
		const resourceName = "test-single-db-pwsync"
		const timeout = 30 * time.Second
		const interval = 250 * time.Millisecond

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			singleDB := &platformv1alpha1.SingleDatabase{}
			err := k8sClient.Get(ctx, typeNamespacedName, singleDB)
			if err == nil {
				return
			}
			resource := &platformv1alpha1.SingleDatabase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: platformv1alpha1.SingleDatabaseSpec{
					Image: "supabase/postgres:17.6.1.084",
					Storage: platformv1alpha1.VolumeClaimTemplateSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &platformv1alpha1.SingleDatabase{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should include password-sync init container in the StatefulSet", func() {
			sts := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db-pwsync-db", Namespace: "default"}, sts)).To(Succeed())
				g.Expect(sts.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			}, timeout, interval).Should(Succeed())

			initContainer := sts.Spec.Template.Spec.InitContainers[0]
			Expect(initContainer.Name).To(Equal("password-sync"))
			Expect(initContainer.Image).To(Equal("supabase/postgres:17.6.1.084"))
			Expect(initContainer.Command).To(Equal([]string{"sh", "-c", passwordSyncScript}))
			Expect(initContainer.VolumeMounts).To(HaveLen(1))
			Expect(initContainer.VolumeMounts[0].MountPath).To(Equal("/var/lib/postgresql/data"))

			// Verify PGPASSWORD env references the credentials secret
			Expect(initContainer.Env).To(HaveLen(1))
			Expect(initContainer.Env[0].Name).To(Equal("PGPASSWORD"))
			Expect(initContainer.Env[0].ValueFrom.SecretKeyRef.Name).To(Equal("test-single-db-pwsync-credentials"))
			Expect(initContainer.Env[0].ValueFrom.SecretKeyRef.Key).To(Equal("password"))
		})

		It("should include secret-hash annotation in the pod template", func() {
			sts := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db-pwsync-db", Namespace: "default"}, sts)).To(Succeed())
				g.Expect(sts.Spec.Template.Annotations).To(HaveKey("supabase.io/secret-hash"))
				g.Expect(sts.Spec.Template.Annotations["supabase.io/secret-hash"]).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})

		It("should update secret-hash annotation when credentials secret is rotated", func() {
			sts := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db-pwsync-db", Namespace: "default"}, sts)).To(Succeed())
				g.Expect(sts.Spec.Template.Annotations).To(HaveKey("supabase.io/secret-hash"))
			}, timeout, interval).Should(Succeed())

			oldHash := sts.Spec.Template.Annotations["supabase.io/secret-hash"]

			// Delete the secret to trigger rotation
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db-pwsync-credentials", Namespace: "default"}, secret)).To(Succeed())
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

			// After reconciliation, the secret is recreated with a new password,
			// and the StatefulSet annotation should change
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db-pwsync-db", Namespace: "default"}, sts)).To(Succeed())
				g.Expect(sts.Spec.Template.Annotations["supabase.io/secret-hash"]).NotTo(Equal(oldHash))
			}, timeout, interval).Should(Succeed())
		})
	})
})
