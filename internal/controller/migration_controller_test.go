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
	Context("When applying a migration batch", func() {
		const singleDBName = "test-batch-migration-db"
		const migrationName = "test-batch-migration"
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

		It("should execute migrations in a single job and track applied hash", func() {
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

			By("Checking that exactly one migration job was created")
			jobName := migrationName + "-apply"
			job := &batchv1.Job{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: "default"}, job)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Verifying the job has correct owner reference")
			Expect(job.OwnerReferences).To(HaveLen(1))
			Expect(job.OwnerReferences[0].Kind).To(Equal("Migration"))
			Expect(job.OwnerReferences[0].Name).To(Equal(migrationName))

			By("Verifying the job uses supabase_admin user")
			container := job.Spec.Template.Spec.Containers[0]
			var pgUserEnv *corev1.EnvVar
			for i := range container.Env {
				if container.Env[i].Name == "PGUSER" {
					pgUserEnv = &container.Env[i]
					break
				}
			}
			Expect(pgUserEnv).NotTo(BeNil())
			Expect(pgUserEnv.Value).To(Equal("supabase_admin"))

			By("Verifying the job receives the MIGRATION_HASH env var")
			var hashEnv *corev1.EnvVar
			for i := range container.Env {
				if container.Env[i].Name == "MIGRATION_HASH" {
					hashEnv = &container.Env[i]
					break
				}
			}
			Expect(hashEnv).NotTo(BeNil())
			Expect(hashEnv.Value).NotTo(BeEmpty())
			expectedHash := calculateBatchHash(migration.Spec.Migrations)
			Expect(hashEnv.Value).To(Equal(expectedHash))

			By("Checking that exactly one ConfigMap was created")
			cmName := migrationName + "-sql"
			cm := &corev1.ConfigMap{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: cmName, Namespace: "default"}, cm)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			Expect(cm.Data).To(HaveKey("batch.sql"))

			By("Simulating job completion")
			job.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

			By("Checking that status shows applied hash and Ready=True")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, migrationNamespacedName, migration)).To(Succeed())
				g.Expect(migration.Status.AppliedHash).To(Equal(expectedHash))
				g.Expect(migration.Status.AppliedAt).NotTo(BeNil())
				g.Expect(meta.IsStatusConditionTrue(migration.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Verifying that job and configmap are cleaned up after success")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: "default"}, &batchv1.Job{})
				g.Expect(err).To(HaveOccurred())
				err = k8sClient.Get(ctx, types.NamespacedName{Name: cmName, Namespace: "default"}, &corev1.ConfigMap{})
				g.Expect(err).To(HaveOccurred())
			}, timeout, interval).Should(Succeed())
		})

		It("should not reapply a batch with the same hash", func() {
			By("Waiting for SingleDatabase to be ready")
			singleDB := &platformv1alpha1.SingleDatabase{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, singleDBNamespacedName, singleDB)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(singleDB.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Creating the first Migration resource")
			firstMigration := &platformv1alpha1.Migration{
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
					},
				},
			}
			Expect(k8sClient.Create(ctx, firstMigration)).To(Succeed())

			By("Simulating first job success")
			firstJobName := migrationName + "-apply"
			firstJob := &batchv1.Job{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: firstJobName, Namespace: "default"}, firstJob)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			firstJob.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, firstJob)).To(Succeed())

			expectedHash := calculateBatchHash(firstMigration.Spec.Migrations)

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, migrationNamespacedName, firstMigration)).To(Succeed())
				g.Expect(firstMigration.Status.AppliedHash).To(Equal(expectedHash))
			}, timeout, interval).Should(Succeed())

			By("Creating a second Migration with the same SQL but different name")
			secondMigrationName := migrationName + "-duplicate"
			secondMigration := &platformv1alpha1.Migration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secondMigrationName,
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
							Name: "different-name",
							SQL:  "CREATE TABLE IF NOT EXISTS users (id serial PRIMARY KEY);",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, secondMigration)).To(Succeed())

			By("Simulating second job success (hash already exists in DB)")
			secondJobName := secondMigrationName + "-apply"
			secondJob := &batchv1.Job{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: secondJobName, Namespace: "default"}, secondJob)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			secondJob.Status.Succeeded = 1
			Expect(k8sClient.Status().Update(ctx, secondJob)).To(Succeed())

			By("Checking that second migration also reports success with the same hash")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: secondMigrationName, Namespace: "default"}, secondMigration)).To(Succeed())
				g.Expect(secondMigration.Status.AppliedHash).To(Equal(expectedHash))
				g.Expect(meta.IsStatusConditionTrue(secondMigration.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When a migration batch job fails", func() {
		const singleDBName = "test-fail-batch-db"
		const migrationName = "test-fail-batch"
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

		It("should stop execution and leave applied hash empty when the batch fails", func() {
			By("Waiting for SingleDatabase to be ready")
			singleDB := &platformv1alpha1.SingleDatabase{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, singleDBNamespacedName, singleDB)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(singleDB.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Creating the Migration resource")
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
					},
				},
			}
			Expect(k8sClient.Create(ctx, migration)).To(Succeed())

			By("Waiting for the job to be created")
			jobName := migrationName + "-apply"
			job := &batchv1.Job{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: "default"}, job)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Simulating job failure")
			job.Status.Failed = 1
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

			By("Verifying migration status shows failure condition")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, migrationNamespacedName, migration)).To(Succeed())
				cond := meta.FindStatusCondition(migration.Status.Conditions, ConditionTypeReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("MigrationFailed"))
				g.Expect(migration.Status.AppliedHash).To(BeEmpty())
			}, timeout, interval).Should(Succeed())

			By("Verifying the failed job is NOT deleted")
			Consistently(func(g Gomega) {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: "default"}, &batchv1.Job{})
				g.Expect(err).NotTo(HaveOccurred())
			}, 2*time.Second, interval).Should(Succeed())
		})
	})

	Context("When attempting to modify migrations", func() {
		const singleDBName = "test-immutable-db"
		const migrationName = "test-immutable"
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

			singleDB := &platformv1alpha1.SingleDatabase{}
			err = k8sClient.Get(ctx, singleDBNamespacedName, singleDB)
			if err == nil {
				Expect(k8sClient.Delete(ctx, singleDB)).To(Succeed())
			}
		})

		It("should reject updates that modify the migrations array", func() {
			By("Waiting for SingleDatabase to be ready")
			singleDB := &platformv1alpha1.SingleDatabase{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, singleDBNamespacedName, singleDB)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(singleDB.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Creating the Migration resource")
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
					},
				},
			}
			Expect(k8sClient.Create(ctx, migration)).To(Succeed())

			By("Attempting to append a new migration entry")
			migration.Spec.Migrations = append(migration.Spec.Migrations, platformv1alpha1.MigrationEntry{
				Name: "002-add-email",
				SQL:  "ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT;",
			})
			err := k8sClient.Update(ctx, migration)
			Expect(err).To(HaveOccurred())
		})
	})
})
