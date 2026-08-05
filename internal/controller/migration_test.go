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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	migrationpkg "github.com/supabase-community/supabase-kubernetes/internal/migration"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	"github.com/supabase-community/supabase-kubernetes/internal/singledatabase"
)

var _ = Describe("Migration Controller", func() {
	const (
		defaultTimeout = 15 * time.Second
		defaultPolling = 150 * time.Millisecond
	)

	var ns string

	BeforeEach(func() {
		ns = "mig-test-" + rand.String(6)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		})).To(Succeed())
	})

	// driveSingleDatabaseReady creates a SingleDatabase CR and drives it to
	// Ready by repeatedly marking its StatefulSet status as ready. The
	// SingleDatabase reconciler then writes ResolvedDatabase and Ready=True
	// itself, which is a stable steady state that database.ResolveRef can read.
	driveSingleDatabaseReady := func(name, image string) *supabasev1alpha1.SingleDatabase {
		db := &supabasev1alpha1.SingleDatabase{
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
		if image != "" {
			db.Spec.Image = &image
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		stsKey := types.NamespacedName{Name: singledatabase.PostgresStatefulSetName(db), Namespace: ns}
		Eventually(func() error {
			return k8sClient.Get(ctx, stsKey, &appsv1.StatefulSet{})
		}, defaultTimeout, defaultPolling).Should(Succeed())

		Eventually(func(g Gomega) {
			sts := &appsv1.StatefulSet{}
			g.Expect(k8sClient.Get(ctx, stsKey, sts)).To(Succeed())
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

		Eventually(func(g Gomega) {
			got := &supabasev1alpha1.SingleDatabase{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, got)).To(Succeed())
			g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
			g.Expect(got.Status.ResolvedDatabase).NotTo(BeNil())
		}, defaultTimeout, defaultPolling).Should(Succeed())

		got := &supabasev1alpha1.SingleDatabase{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, got)).To(Succeed())
		return got
	}

	newMigration := func(name, dbName string) *supabasev1alpha1.Migration {
		return &supabasev1alpha1.Migration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: supabasev1alpha1.MigrationSpec{
				DatabaseRef: supabasev1alpha1.DatabaseRef{
					Kind: "SingleDatabase",
					Name: dbName,
				},
				Migrations: []supabasev1alpha1.MigrationEntry{
					{Name: "01-init", SQL: "CREATE TABLE t (id int);"},
				},
			},
		}
	}

	Context("when a Migration is created against a ready database", func() {
		It("creates a ConfigMap and a Job wired to the resolved database", func() {
			db := driveSingleDatabaseReady("pg", "")
			m := newMigration("mig", "pg")
			Expect(k8sClient.Create(ctx, m)).To(Succeed())

			cmKey := types.NamespacedName{Name: migrationpkg.MigrationConfigMapName(m), Namespace: ns}
			jobKey := types.NamespacedName{Name: migrationpkg.MigrationJobName(m), Namespace: ns}

			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, cmKey, cm)).To(Succeed())
				g.Expect(cm.Data).To(HaveKey(migrationpkg.MigrationsBatchFile))
				g.Expect(cm.OwnerReferences).To(HaveLen(1))
				g.Expect(cm.OwnerReferences[0].Kind).To(Equal("Migration"))

				job := &batchv1.Job{}
				g.Expect(k8sClient.Get(ctx, jobKey, job)).To(Succeed())
				g.Expect(job.OwnerReferences).To(HaveLen(1))
				g.Expect(job.OwnerReferences[0].Kind).To(Equal("Migration"))
				g.Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))

				// With no explicit override, the Job inherits the resolved
				// database image (here the default Postgres image).
				g.Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal(db.Status.ResolvedDatabase.Image))

				envMap := map[string]string{}
				for _, e := range job.Spec.Template.Spec.Containers[0].Env {
					envMap[e.Name] = e.Value
				}
				g.Expect(envMap["PGHOST"]).To(Equal(singledatabase.PostgresServiceHost(db)))
				g.Expect(envMap["PGPORT"]).To(Equal("5432"))
				g.Expect(envMap["PGUSER"]).To(Equal(singledatabase.DefaultPostgresUser))
				g.Expect(envMap["PGDATABASE"]).To(Equal(singledatabase.DefaultPostgresDatabase))

				// PGPASSWORD comes from a secret reference.
				var foundPGPassword bool
				for _, e := range job.Spec.Template.Spec.Containers[0].Env {
					if e.Name == "PGPASSWORD" {
						g.Expect(e.ValueFrom).NotTo(BeNil())
						g.Expect(e.ValueFrom.SecretKeyRef).NotTo(BeNil())
						g.Expect(e.ValueFrom.SecretKeyRef.Name).To(Equal(singledatabase.PostgresSecretName(db)))
						g.Expect(e.ValueFrom.SecretKeyRef.Key).To(Equal(singledatabase.DefaultSecretKeyPassword))
						foundPGPassword = true
					}
				}
				g.Expect(foundPGPassword).To(BeTrue(), "PGPASSWORD env var should be present")
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("inherits a custom database image, and an explicit spec image wins", func() {
			const dbImage = "example.com/supabase/postgres:custom"
			const overrideImage = "example.com/supabase/postgres:override"

			driveSingleDatabaseReady("pg", dbImage)

			By("inheriting the resolved database image when the Migration has none")
			m := newMigration("mig", "pg")
			Expect(k8sClient.Create(ctx, m)).To(Succeed())
			jobKey := types.NamespacedName{Name: migrationpkg.MigrationJobName(m), Namespace: ns}
			Eventually(func(g Gomega) {
				job := &batchv1.Job{}
				g.Expect(k8sClient.Get(ctx, jobKey, job)).To(Succeed())
				g.Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal(dbImage))
			}, defaultTimeout, defaultPolling).Should(Succeed())

			By("preferring an explicit spec image over the inherited one")
			mo := newMigration("mig-override", "pg")
			mo.Spec.Image = ptr.To(overrideImage)
			Expect(k8sClient.Create(ctx, mo)).To(Succeed())
			jobKeyO := types.NamespacedName{Name: migrationpkg.MigrationJobName(mo), Namespace: ns}
			Eventually(func(g Gomega) {
				job := &batchv1.Job{}
				g.Expect(k8sClient.Get(ctx, jobKeyO, job)).To(Succeed())
				g.Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal(overrideImage))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("stays Ready=False with reason JobInProgress until the Job succeeds", func() {
			driveSingleDatabaseReady("pg", "")
			m := newMigration("mig", "pg")
			Expect(k8sClient.Create(ctx, m)).To(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Migration{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: m.Name, Namespace: ns}, got)).To(Succeed())
				cond := apimeta.FindStatusCondition(got.Status.Conditions, reconciler.ConditionTypeReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("JobInProgress"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("becomes Ready=True with AppliedHash set when the Job succeeds", func() {
			driveSingleDatabaseReady("pg", "")
			m := newMigration("mig", "pg")
			Expect(k8sClient.Create(ctx, m)).To(Succeed())

			jobKey := types.NamespacedName{Name: migrationpkg.MigrationJobName(m), Namespace: ns}
			Eventually(func() error {
				return k8sClient.Get(ctx, jobKey, &batchv1.Job{})
			}, defaultTimeout, defaultPolling).Should(Succeed())

			Eventually(func(g Gomega) {
				job := &batchv1.Job{}
				g.Expect(k8sClient.Get(ctx, jobKey, job)).To(Succeed())
				job.Status.Succeeded = 1
				g.Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
			}, defaultTimeout, defaultPolling).Should(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Migration{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: m.Name, Namespace: ns}, got)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
				g.Expect(got.Status.AppliedHash).NotTo(BeEmpty())
				g.Expect(got.Status.AppliedAt).NotTo(BeNil())
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("reports Ready=False with reason JobFailed when the Job fails", func() {
			driveSingleDatabaseReady("pg", "")
			m := newMigration("mig", "pg")
			Expect(k8sClient.Create(ctx, m)).To(Succeed())

			jobKey := types.NamespacedName{Name: migrationpkg.MigrationJobName(m), Namespace: ns}
			Eventually(func() error {
				return k8sClient.Get(ctx, jobKey, &batchv1.Job{})
			}, defaultTimeout, defaultPolling).Should(Succeed())

			Eventually(func(g Gomega) {
				job := &batchv1.Job{}
				g.Expect(k8sClient.Get(ctx, jobKey, job)).To(Succeed())
				job.Status.Failed = 1
				g.Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
			}, defaultTimeout, defaultPolling).Should(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Migration{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: m.Name, Namespace: ns}, got)).To(Succeed())
				cond := apimeta.FindStatusCondition(got.Status.Conditions, reconciler.ConditionTypeReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("JobFailed"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})
	})

	Context("when the referenced database is not ready", func() {
		It("reports Ready=False with reason DatabaseNotReady", func() {
			// Note: SingleDatabase does not exist at all - ResolveRef returns ready=false.
			m := newMigration("mig", "missing-db")
			Expect(k8sClient.Create(ctx, m)).To(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Migration{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: m.Name, Namespace: ns}, got)).To(Succeed())
				cond := apimeta.FindStatusCondition(got.Status.Conditions, reconciler.ConditionTypeReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("DatabaseNotReady"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})
	})
})
