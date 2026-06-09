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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/migration"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	"github.com/supabase-community/supabase-kubernetes/internal/singledatabase"
)

var _ = Describe("Migration Controller", func() {
	const timeout = 30 * time.Second
	const interval = 250 * time.Millisecond

	Context("When creating a Migration with a ready SingleDatabase", func() {
		const dbName = "test-migration-db"
		const migrationName = "test-migration"
		dbKey := types.NamespacedName{Name: dbName, Namespace: "default"}
		migrationKey := types.NamespacedName{Name: migrationName, Namespace: "default"}

		BeforeEach(func() {
			By("Creating a SingleDatabase")
			db := &supabasev1alpha1.SingleDatabase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dbName,
					Namespace: "default",
				},
				Spec: supabasev1alpha1.SingleDatabaseSpec{

					Storage: supabasev1alpha1.VolumeClaim{
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

			By("Simulating StatefulSet readiness")
			sts := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      singledatabase.StatefulSetName(dbName),
					Namespace: "default",
				}, sts)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			sts.Status.Replicas = 1
			sts.Status.ReadyReplicas = 1
			Expect(k8sClient.Status().Update(ctx, sts)).To(Succeed())

			By("Waiting for SingleDatabase to be ready")
			Eventually(func(g Gomega) {
				db := &supabasev1alpha1.SingleDatabase{}
				g.Expect(k8sClient.Get(ctx, dbKey, db)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(db.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Creating the Migration")
			m := &supabasev1alpha1.Migration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      migrationName,
					Namespace: "default",
				},
				Spec: supabasev1alpha1.MigrationSpec{

					DatabaseRef: supabasev1alpha1.DatabaseRef{
						Kind: "SingleDatabase",
						Name: dbName,
					},
					Migrations: []supabasev1alpha1.MigrationEntry{
						{
							Name: "init",
							SQL:  "CREATE TABLE tests (id INT);",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, m)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up Job")
			job := &batchv1.Job{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: migration.JobName(migrationName), Namespace: "default"}, job); err == nil {
				background := metav1.DeletePropagationBackground
				Expect(k8sClient.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &background})).To(Succeed())
			}

			By("Cleaning up ConfigMap")
			cm := &corev1.ConfigMap{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: migration.ConfigMapName(migrationName), Namespace: "default"}, cm); err == nil {
				Expect(k8sClient.Delete(ctx, cm)).To(Succeed())
			}

			By("Cleaning up Migration")
			m := &supabasev1alpha1.Migration{}
			if err := k8sClient.Get(ctx, migrationKey, m); err == nil {
				Expect(k8sClient.Delete(ctx, m)).To(Succeed())
			}

			By("Cleaning up SingleDatabase")
			db := &supabasev1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, dbKey, db); err == nil {
				Expect(k8sClient.Delete(ctx, db)).To(Succeed())
			}
		})

		It("Should create ConfigMap and Job", func() {
			By("Checking that ConfigMap was created")
			cm := &corev1.ConfigMap{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      migration.ConfigMapName(migrationName),
					Namespace: "default",
				}, cm)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			Expect(cm.Data).To(HaveKey("batch.sql"))

			By("Checking that Job was created")
			job := &batchv1.Job{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      migration.JobName(migrationName),
					Namespace: "default",
				}, job)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
		})

		It("Should mark Migration as Ready when Job succeeds", func() {
			By("Simulating Job completion")
			job := &batchv1.Job{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      migration.JobName(migrationName),
					Namespace: "default",
				}, job)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			job.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

			By("Checking that Migration status is Ready")
			Eventually(func(g Gomega) {
				m := &supabasev1alpha1.Migration{}
				g.Expect(k8sClient.Get(ctx, migrationKey, m)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(m.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
				g.Expect(m.Status.AppliedHash).NotTo(BeEmpty())
				g.Expect(m.Status.AppliedAt).NotTo(BeNil())
			}, timeout, interval).Should(Succeed())
		})

		It("Should regenerate ConfigMap if deleted", func() {
			By("Waiting for ConfigMap to be created")
			cm := &corev1.ConfigMap{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      migration.ConfigMapName(migrationName),
					Namespace: "default",
				}, cm)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			oldData := cm.Data["batch.sql"]
			Expect(k8sClient.Delete(ctx, cm)).To(Succeed())

			By("Waiting for ConfigMap to be recreated")
			Eventually(func(g Gomega) {
				recreated := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      migration.ConfigMapName(migrationName),
					Namespace: "default",
				}, recreated)).To(Succeed())
				g.Expect(recreated.Data["batch.sql"]).To(Equal(oldData))
			}, timeout, interval).Should(Succeed())
		})

		It("Should not recreate Job when migration is already applied", func() {
			By("Waiting for Job to be created")
			job := &batchv1.Job{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      migration.JobName(migrationName),
					Namespace: "default",
				}, job)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Simulating Job completion")
			job.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

			By("Waiting for Migration to be marked as applied")
			Eventually(func(g Gomega) {
				m := &supabasev1alpha1.Migration{}
				g.Expect(k8sClient.Get(ctx, migrationKey, m)).To(Succeed())
				g.Expect(m.Status.AppliedHash).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())

			By("Deleting the Job")
			background := metav1.DeletePropagationBackground
			Expect(k8sClient.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &background})).To(Succeed())

			By("Waiting for Job to be deleted")
			Eventually(func(g Gomega) {
				j := &batchv1.Job{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      migration.JobName(migrationName),
					Namespace: "default",
				}, j)
				g.Expect(err).To(HaveOccurred())
			}, timeout, interval).Should(Succeed())

			By("Ensuring Job is not recreated")
			Consistently(func(g Gomega) {
				j := &batchv1.Job{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      migration.JobName(migrationName),
					Namespace: "default",
				}, j)
				g.Expect(err).To(HaveOccurred())
			}, 3*time.Second, 500*time.Millisecond).Should(Succeed())
		})
	})
})
