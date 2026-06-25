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
	"k8s.io/apimachinery/pkg/util/rand"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	"github.com/supabase-community/supabase-kubernetes/internal/singledatabase"
)

var _ = Describe("SingleDatabase Controller", func() {
	const (
		defaultTimeout = 15 * time.Second
		defaultPolling = 150 * time.Millisecond
	)

	var ns string

	BeforeEach(func() {
		ns = "sdb-test-" + rand.String(6)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		})).To(Succeed())
	})

	newSingleDatabase := func(name string) *supabasev1alpha1.SingleDatabase {
		return &supabasev1alpha1.SingleDatabase{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: supabasev1alpha1.SingleDatabaseSpec{
				Storage: supabasev1alpha1.VolumeClaim{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Size:        resource.MustParse("1Gi"),
				},
			},
		}
	}

	markStatefulSetReady := func(name string) {
		Eventually(func(g Gomega) {
			sts := &appsv1.StatefulSet{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, sts)).To(Succeed())
			replicas := int32(1)
			if sts.Spec.Replicas != nil {
				replicas = *sts.Spec.Replicas
			}
			sts.Status.ReadyReplicas = replicas
			sts.Status.Replicas = replicas
			sts.Status.AvailableReplicas = replicas
			sts.Status.ObservedGeneration = sts.Generation
			g.Expect(k8sClient.Status().Update(ctx, sts)).To(Succeed())
		}, defaultTimeout, defaultPolling).Should(Succeed())
	}

	Context("when a SingleDatabase is created", func() {
		It("creates Secret, PVC, Service and StatefulSet with controller ownerRefs", func() {
			db := newSingleDatabase("pg")
			Expect(k8sClient.Create(ctx, db)).To(Succeed())

			secretKey := types.NamespacedName{Name: singledatabase.PostgresSecretName(db), Namespace: ns}
			pvcKey := types.NamespacedName{Name: singledatabase.PostgresPVCName(db), Namespace: ns}
			svcKey := types.NamespacedName{Name: singledatabase.PostgresServiceName(db), Namespace: ns}
			stsKey := types.NamespacedName{Name: singledatabase.PostgresStatefulSetName(db), Namespace: ns}

			Eventually(func(g Gomega) {
				secret := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, secretKey, secret)).To(Succeed())
				g.Expect(secret.Data).To(HaveKey(singledatabase.DefaultSecretKeyPassword))
				g.Expect(secret.OwnerReferences).To(HaveLen(1))
				g.Expect(secret.OwnerReferences[0].Kind).To(Equal("SingleDatabase"))

				pvc := &corev1.PersistentVolumeClaim{}
				g.Expect(k8sClient.Get(ctx, pvcKey, pvc)).To(Succeed())
				g.Expect(pvc.Spec.AccessModes).To(ContainElement(corev1.ReadWriteOnce))

				svc := &corev1.Service{}
				g.Expect(k8sClient.Get(ctx, svcKey, svc)).To(Succeed())
				g.Expect(svc.OwnerReferences).To(HaveLen(1))
				g.Expect(svc.OwnerReferences[0].Kind).To(Equal("SingleDatabase"))

				sts := &appsv1.StatefulSet{}
				g.Expect(k8sClient.Get(ctx, stsKey, sts)).To(Succeed())
				g.Expect(sts.OwnerReferences).To(HaveLen(1))
				g.Expect(sts.OwnerReferences[0].Kind).To(Equal("SingleDatabase"))
				g.Expect(sts.Spec.Template.Spec.Containers).To(HaveLen(1))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("reports Ready=False with reason StatefulSetNotReady before the StatefulSet pods come up", func() {
			db := newSingleDatabase("pg")
			Expect(k8sClient.Create(ctx, db)).To(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.SingleDatabase{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: ns}, got)).To(Succeed())
				cond := meta.FindStatusCondition(got.Status.Conditions, reconciler.ConditionTypeReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("StatefulSetNotReady"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("reports Ready=True and populates ResolvedDatabase once the StatefulSet is ready", func() {
			db := newSingleDatabase("pg")
			Expect(k8sClient.Create(ctx, db)).To(Succeed())

			markStatefulSetReady(singledatabase.PostgresStatefulSetName(db))

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.SingleDatabase{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: ns}, got)).To(Succeed())

				g.Expect(meta.IsStatusConditionTrue(got.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
				g.Expect(got.Status.ResolvedDatabase).NotTo(BeNil())
				g.Expect(got.Status.ResolvedDatabase.Host).To(Equal(singledatabase.PostgresServiceHost(db)))
				g.Expect(got.Status.ResolvedDatabase.Port).To(Equal(singledatabase.DefaultPostgresPort))
				g.Expect(got.Status.ResolvedDatabase.DBName).To(Equal(singledatabase.DefaultPostgresDatabase))
				g.Expect(got.Status.ResolvedDatabase.User).To(Equal(singledatabase.DefaultPostgresUser))
				g.Expect(got.Status.ResolvedDatabase.PasswordRef.Name).To(Equal(singledatabase.PostgresSecretName(db)))
				g.Expect(got.Status.ResolvedDatabase.PasswordRef.Key).To(Equal(singledatabase.DefaultSecretKeyPassword))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})
	})

	Context("when the SingleDatabase spec is updated", func() {
		It("propagates pod label changes to the StatefulSet template", func() {
			db := newSingleDatabase("pg")
			Expect(k8sClient.Create(ctx, db)).To(Succeed())

			stsKey := types.NamespacedName{Name: singledatabase.PostgresStatefulSetName(db), Namespace: ns}
			Eventually(func() error {
				return k8sClient.Get(ctx, stsKey, &appsv1.StatefulSet{})
			}, defaultTimeout, defaultPolling).Should(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.SingleDatabase{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: ns}, got)).To(Succeed())
				got.Spec.PodLabels = map[string]string{"custom-label": "applied"}
				g.Expect(k8sClient.Update(ctx, got)).To(Succeed())
			}, defaultTimeout, defaultPolling).Should(Succeed())

			Eventually(func(g Gomega) {
				sts := &appsv1.StatefulSet{}
				g.Expect(k8sClient.Get(ctx, stsKey, sts)).To(Succeed())
				g.Expect(sts.Spec.Template.Labels).To(HaveKeyWithValue("custom-label", "applied"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})
	})
})
