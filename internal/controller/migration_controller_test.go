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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

var _ = Describe("Migration Controller", func() {
	Context("When reconciling sequential migrations", func() {
		const singleDBName = "test-seq-migration-db"
		const migrationName = "test-seq-migration"
		const timeout = 30 * time.Second
		const interval = 250 * time.Millisecond

		ctx := context.Background()

		singleDBNamespacedName := types.NamespacedName{
			Name:      singleDBName,
			Namespace: "default",
		}
		migrationNamespacedName := types.NamespacedName{
			Name:      migrationName,
			Namespace: "default",
		}

		BeforeEach(func() {
			singleDB := &platformv1alpha1.SingleDatabase{}
			err := k8sClient.Get(ctx, singleDBNamespacedName, singleDB)
			if err == nil {
				return
			}
			dbResource := &platformv1alpha1.SingleDatabase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      singleDBName,
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
			Expect(k8sClient.Create(ctx, dbResource)).To(Succeed())
		})

		AfterEach(func() {
			// Clean up migration first (which owns the jobs)
			migration := &platformv1alpha1.Migration{}
			err := k8sClient.Get(ctx, migrationNamespacedName, migration)
			if err == nil {
				Expect(k8sClient.Delete(ctx, migration)).To(Succeed())
			}

			// Clean up any leftover jobs
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
			for i := range jobList.Items {
				propagation := metav1.DeletePropagationBackground
				Expect(k8sClient.Delete(ctx, &jobList.Items[i], &client.DeleteOptions{
					PropagationPolicy: &propagation,
				})).To(Succeed())
			}

			singleDB := &platformv1alpha1.SingleDatabase{}
			err = k8sClient.Get(ctx, singleDBNamespacedName, singleDB)
			if err == nil {
				Expect(k8sClient.Delete(ctx, singleDB)).To(Succeed())
			}
		})

		It("should execute migrations sequentially and track status per step", func() {
			By("Waiting for SingleDatabase to be ready")
			singleDB := &platformv1alpha1.SingleDatabase{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, singleDBNamespacedName, singleDB)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(singleDB.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Creating the Migration resource with multiple migrations")
			migration := &platformv1alpha1.Migration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      migrationName,
					Namespace: "default",
				},
				Spec: platformv1alpha1.MigrationSpec{
					DatabaseRef: platformv1alpha1.DatabaseRef{
						Kind: "SingleDatabase",
						Name: singleDBName,
					},
					Image: "supabase/postgres:17.6.1.084",
					Migrations: []platformv1alpha1.MigrationEntry{
						{
							Name: "001-create-users",
							SQL:  "CREATE TABLE IF NOT EXISTS users (id serial PRIMARY KEY);",
						},
						{
							Name: "002-add-email",
							SQL:  "ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT;",
						},
						{
							Name: "003-create-posts",
							SQL:  "CREATE TABLE IF NOT EXISTS posts (id serial PRIMARY KEY, user_id INTEGER REFERENCES users(id));",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, migration)).To(Succeed())

			By("Checking that the first migration job was created")
			firstJobName := migrationName + "-0"
			firstJob := &batchv1.Job{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: firstJobName, Namespace: "default"}, firstJob)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Verifying the first job has correct owner reference")
			Expect(firstJob.OwnerReferences).To(HaveLen(1))
			Expect(firstJob.OwnerReferences[0].Kind).To(Equal("Migration"))
			Expect(firstJob.OwnerReferences[0].Name).To(Equal(migrationName))

			By("Verifying the job uses supabase_admin user")
			container := firstJob.Spec.Template.Spec.Containers[0]
			var pgUserEnv *corev1.EnvVar
			for i := range container.Env {
				if container.Env[i].Name == "PGUSER" {
					pgUserEnv = &container.Env[i]
					break
				}
			}
			Expect(pgUserEnv).NotTo(BeNil())
			Expect(pgUserEnv.Value).To(Equal("supabase_admin"))

			By("Verifying the second job does NOT exist yet")
			secondJobName := migrationName + "-1"
			secondJob := &batchv1.Job{}
			Consistently(func(g Gomega) {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: secondJobName, Namespace: "default"}, secondJob)
				g.Expect(err).To(HaveOccurred())
			}, 2*time.Second, interval).Should(Succeed())

			By("Simulating first job completion")
			firstJob.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, firstJob)).To(Succeed())

			By("Checking that the second migration job is created after first completes")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: secondJobName, Namespace: "default"}, secondJob)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Simulating second job completion")
			secondJob.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, secondJob)).To(Succeed())

			By("Checking that the third migration job is created")
			thirdJobName := migrationName + "-2"
			thirdJob := &batchv1.Job{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: thirdJobName, Namespace: "default"}, thirdJob)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Simulating third job completion")
			thirdJob.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, thirdJob)).To(Succeed())

			By("Checking that all migrations are marked as applied in status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, migrationNamespacedName, migration)).To(Succeed())
				g.Expect(migration.Status.MigrationStatuses).To(HaveLen(3))
				g.Expect(migration.Status.MigrationStatuses[0].Applied).To(BeTrue())
				g.Expect(migration.Status.MigrationStatuses[0].Name).To(Equal("001-create-users"))
				g.Expect(migration.Status.MigrationStatuses[0].JobName).To(Equal(firstJobName))
				g.Expect(migration.Status.MigrationStatuses[1].Applied).To(BeTrue())
				g.Expect(migration.Status.MigrationStatuses[1].Name).To(Equal("002-add-email"))
				g.Expect(migration.Status.MigrationStatuses[1].JobName).To(Equal(secondJobName))
				g.Expect(migration.Status.MigrationStatuses[2].Applied).To(BeTrue())
				g.Expect(migration.Status.MigrationStatuses[2].Name).To(Equal("003-create-posts"))
				g.Expect(migration.Status.MigrationStatuses[2].JobName).To(Equal(thirdJobName))
				g.Expect(meta.IsStatusConditionTrue(migration.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

	})

	Context("When a migration job fails", func() {
		const singleDBName = "test-fail-migration-db"
		const migrationName = "test-fail-migration"
		const timeout = 30 * time.Second
		const interval = 250 * time.Millisecond

		ctx := context.Background()

		singleDBNamespacedName := types.NamespacedName{
			Name:      singleDBName,
			Namespace: "default",
		}
		migrationNamespacedName := types.NamespacedName{
			Name:      migrationName,
			Namespace: "default",
		}

		BeforeEach(func() {
			singleDB := &platformv1alpha1.SingleDatabase{}
			err := k8sClient.Get(ctx, singleDBNamespacedName, singleDB)
			if err == nil {
				return
			}
			dbResource := &platformv1alpha1.SingleDatabase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      singleDBName,
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
			Expect(k8sClient.Create(ctx, dbResource)).To(Succeed())
		})

		AfterEach(func() {
			migration := &platformv1alpha1.Migration{}
			err := k8sClient.Get(ctx, migrationNamespacedName, migration)
			if err == nil {
				Expect(k8sClient.Delete(ctx, migration)).To(Succeed())
			}

			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
			for i := range jobList.Items {
				propagation := metav1.DeletePropagationBackground
				_ = k8sClient.Delete(ctx, &jobList.Items[i], &client.DeleteOptions{
					PropagationPolicy: &propagation,
				})
			}

			singleDB := &platformv1alpha1.SingleDatabase{}
			err = k8sClient.Get(ctx, singleDBNamespacedName, singleDB)
			if err == nil {
				Expect(k8sClient.Delete(ctx, singleDB)).To(Succeed())
			}
		})

		It("should stop execution when a migration job fails", func() {
			By("Waiting for SingleDatabase to be ready")
			singleDB := &platformv1alpha1.SingleDatabase{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, singleDBNamespacedName, singleDB)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(singleDB.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Creating the Migration resource with two migrations")
			migration := &platformv1alpha1.Migration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      migrationName,
					Namespace: "default",
				},
				Spec: platformv1alpha1.MigrationSpec{
					DatabaseRef: platformv1alpha1.DatabaseRef{
						Kind: "SingleDatabase",
						Name: singleDBName,
					},
					Image: "supabase/postgres:17.6.1.084",
					Migrations: []platformv1alpha1.MigrationEntry{
						{
							Name: "001-will-fail",
							SQL:  "INVALID SQL;",
						},
						{
							Name: "002-should-not-run",
							SQL:  "CREATE TABLE IF NOT EXISTS should_not_exist (id serial PRIMARY KEY);",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, migration)).To(Succeed())

			By("Waiting for the first job to be created")
			firstJobName := migrationName + "-0"
			firstJob := &batchv1.Job{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: firstJobName, Namespace: "default"}, firstJob)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Simulating first job failure")
			firstJob.Status.Failed = 1
			Expect(k8sClient.Status().Update(ctx, firstJob)).To(Succeed())

			By("Verifying migration status shows failure condition")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, migrationNamespacedName, migration)).To(Succeed())
				cond := meta.FindStatusCondition(migration.Status.Conditions, ConditionTypeReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("MigrationFailed"))
			}, timeout, interval).Should(Succeed())

			By("Verifying the second job was never created")
			secondJobName := migrationName + "-1"
			secondJob := &batchv1.Job{}
			Consistently(func(g Gomega) {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: secondJobName, Namespace: "default"}, secondJob)
				g.Expect(err).To(HaveOccurred())
			}, 2*time.Second, interval).Should(Succeed())
		})
	})
})
