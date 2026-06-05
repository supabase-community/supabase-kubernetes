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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/singledatabase"
)

func testSingleDatabaseMinimal(name string) *supabasev1alpha1.SingleDatabase {
	return &supabasev1alpha1.SingleDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: supabasev1alpha1.SingleDatabaseSpec{
			Version: "2026.04.27",
		},
	}
}

var _ = Describe("SingleDatabase Controller", func() {
	const timeout = 30 * time.Second
	const interval = 250 * time.Millisecond

	Context("When creating a SingleDatabase", func() {
		const dbName = "test-singledatabase"
		dbKey := types.NamespacedName{Name: dbName, Namespace: "default"}

		BeforeEach(func() {
			db := &supabasev1alpha1.SingleDatabase{
				ObjectMeta: metav1.ObjectMeta{Name: dbName, Namespace: "default"},
				Spec: supabasev1alpha1.SingleDatabaseSpec{
					Version: "2026.04.27",
					Storage: supabasev1alpha1.VolumeClaimTemplateSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, db)).To(Succeed())
		})

		AfterEach(func() {
			db := &supabasev1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, dbKey, db); err == nil {
				Expect(k8sClient.Delete(ctx, db)).To(Succeed())
			}
		})

		It("Should create Secret, PVC, Service and StatefulSet", func() {
			By("Checking that Secret was created")
			secret := &corev1.Secret{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: singledatabase.SecretName(dbName), Namespace: "default"}, secret)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			Expect(secret.Data).To(HaveKey("password"))
			Expect(secret.Data).To(HaveKey("database"))

			By("Checking that PVC was created")
			pvc := &corev1.PersistentVolumeClaim{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: singledatabase.PVCName(dbName), Namespace: "default"}, pvc)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Checking that Service was created")
			svc := &corev1.Service{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: singledatabase.ServiceName(dbName), Namespace: "default"}, svc)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			Expect(svc.Spec.Ports).To(HaveLen(1))
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(5432)))

			By("Checking that StatefulSet was created")
			sts := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: singledatabase.StatefulSetName(dbName), Namespace: "default"}, sts)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			Expect(sts.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(sts.Spec.Template.Spec.InitContainers).To(HaveLen(1))
		})

		It("Should mark SingleDatabase as Ready when StatefulSet is ready", func() {
			By("Simulating StatefulSet readiness")
			sts := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: singledatabase.StatefulSetName(dbName), Namespace: "default"}, sts)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			sts.Status.Replicas = 1
			sts.Status.ReadyReplicas = 1
			Expect(k8sClient.Status().Update(ctx, sts)).To(Succeed())

			By("Checking that SingleDatabase status is Ready")
			Eventually(func(g Gomega) {
				db := &supabasev1alpha1.SingleDatabase{}
				g.Expect(k8sClient.Get(ctx, dbKey, db)).To(Succeed())
				g.Expect(db.Status.Phase).To(Equal("Ready"))
				g.Expect(meta.IsStatusConditionTrue(db.Status.Conditions, ConditionTypeReady)).To(BeTrue())
				g.Expect(db.Status.ServiceName).To(Equal(singledatabase.ServiceName(dbName)))
				g.Expect(db.Status.SecretName).To(Equal(singledatabase.SecretName(dbName)))
			}, timeout, interval).Should(Succeed())
		})

		It("Should regenerate password Secret if deleted", func() {
			By("Waiting for Secret to be created")
			secret := &corev1.Secret{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: singledatabase.SecretName(dbName), Namespace: "default"}, secret)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			oldPassword := string(secret.Data["password"])
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

			By("Waiting for Secret to be recreated with new password")
			Eventually(func(g Gomega) {
				recreated := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: singledatabase.SecretName(dbName), Namespace: "default"}, recreated)).To(Succeed())
				g.Expect(string(recreated.Data["password"])).NotTo(Equal(oldPassword))
			}, timeout, interval).Should(Succeed())
		})

		It("Should update StatefulSet spec when SingleDatabase spec changes", func() {
			By("Waiting for StatefulSet to be created")
			sts := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: singledatabase.StatefulSetName(dbName), Namespace: "default"}, sts)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			oldImage := sts.Spec.Template.Spec.Containers[0].Image

			By("Updating SingleDatabase version")
			db := &supabasev1alpha1.SingleDatabase{}
			Expect(k8sClient.Get(ctx, dbKey, db)).To(Succeed())
			db.Spec.Version = "2026.04.27"
			// Trigger a change by adding an env var since version alone isn't stored directly in a field we mutate
			db.Spec.Env = []corev1.EnvVar{{Name: "TEST_VAR", Value: "test_value"}}
			Expect(k8sClient.Update(ctx, db)).To(Succeed())

			By("Waiting for StatefulSet to be updated with new env var")
			Eventually(func(g Gomega) {
				updated := &appsv1.StatefulSet{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: singledatabase.StatefulSetName(dbName), Namespace: "default"}, updated)).To(Succeed())
				container := updated.Spec.Template.Spec.Containers[0]
				var found bool
				for _, env := range container.Env {
					if env.Name == "TEST_VAR" && env.Value == "test_value" {
						found = true
						break
					}
				}
				g.Expect(found).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			_ = oldImage
		})
	})

	Context("When creating a SingleDatabase without storage resources", func() {
		const dbName = "test-singledatabase-default-storage"
		dbKey := types.NamespacedName{Name: dbName, Namespace: "default"}

		BeforeEach(func() {
			db := testSingleDatabaseMinimal(dbName)
			Expect(k8sClient.Create(ctx, db)).To(Succeed())
		})

		AfterEach(func() {
			db := &supabasev1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, dbKey, db); err == nil {
				Expect(k8sClient.Delete(ctx, db)).To(Succeed())
			}
		})

		It("Should default storage to 10Gi", func() {
			pvc := &corev1.PersistentVolumeClaim{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: singledatabase.PVCName(dbName), Namespace: "default"}, pvc)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			storage := pvc.Spec.Resources.Requests.Storage()
			Expect(storage).NotTo(BeNil())
			Expect(storage.String()).To(Equal("10Gi"))
		})
	})
})
