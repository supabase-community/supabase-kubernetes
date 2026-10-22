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

package tests

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
	restdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/rest"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults/singledatabase"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

var _ = Describe("Project Controller", func() {
	const (
		defaultTimeout = 30 * time.Second
		defaultPolling = 200 * time.Millisecond
	)

	var ns string

	BeforeEach(func() {
		ns = "proj-test-" + rand.String(6)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		})).To(Succeed())
	})

	// driveSingleDatabaseReady creates a SingleDatabase CR and drives it to
	// Ready by marking its StatefulSet status as ready. The SingleDatabase
	// reconciler then writes Ready=True and populates ResolvedDatabase, which
	// is a stable steady state that the Project reconciler can rely on.
	driveSingleDatabaseReady := func(name string) *supabasev1alpha1.SingleDatabase {
		db := &supabasev1alpha1.SingleDatabase{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: supabasev1alpha1.SingleDatabaseSpec{},
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

	markJobSucceeded := func(name string) {
		key := types.NamespacedName{Name: name, Namespace: ns}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &batchv1.Job{})
		}, defaultTimeout, defaultPolling).Should(Succeed())

		Eventually(func(g Gomega) {
			job := &batchv1.Job{}
			g.Expect(k8sClient.Get(ctx, key, job)).To(Succeed())
			job.Status.Succeeded = 1
			g.Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
		}, defaultTimeout, defaultPolling).Should(Succeed())
	}

	markDeploymentReady := func(name string) {
		key := types.NamespacedName{Name: name, Namespace: ns}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &appsv1.Deployment{})
		}, defaultTimeout, defaultPolling).Should(Succeed())

		Eventually(func(g Gomega) {
			d := &appsv1.Deployment{}
			g.Expect(k8sClient.Get(ctx, key, d)).To(Succeed())
			replicas := int32(1)
			if d.Spec.Replicas != nil {
				replicas = *d.Spec.Replicas
			}
			d.Status.UpdatedReplicas = replicas
			d.Status.ReadyReplicas = replicas
			d.Status.Replicas = replicas
			d.Status.AvailableReplicas = replicas
			d.Status.ObservedGeneration = d.Generation
			g.Expect(k8sClient.Status().Update(ctx, d)).To(Succeed())
		}, defaultTimeout, defaultPolling).Should(Succeed())
	}

	// newProject builds shared infrastructure without components.
	newProject := func(name, dbName string) *supabasev1alpha1.Project {
		return &supabasev1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: supabasev1alpha1.ProjectSpec{
				PublicURL: "http://api.test.local",
				DatabaseRef: supabasev1alpha1.DatabaseRef{
					Kind: "SingleDatabase",
					Name: dbName,
				},
			},
		}
	}

	newMigration := func(name, dbName string) *supabasev1alpha1.Migration {
		return &supabasev1alpha1.Migration{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: supabasev1alpha1.MigrationSpec{
				DatabaseRef: supabasev1alpha1.DatabaseRef{Kind: "SingleDatabase", Name: dbName},
				Migrations: []supabasev1alpha1.MigrationEntry{
					{Name: "01-init", SQL: "CREATE TABLE project_migration_test (id integer);"},
				},
			},
		}
	}

	waitForMigrationPending := func(name string) {
		Eventually(func(g Gomega) {
			migration := &supabasev1alpha1.Migration{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, migration)).To(Succeed())
			g.Expect(apimeta.IsStatusConditionTrue(migration.Status.Conditions, reconciler.ConditionTypeReady)).To(BeFalse())
		}, defaultTimeout, defaultPolling).Should(Succeed())
	}

	markProjectMigrationSucceeded := func(proj *supabasev1alpha1.Project) {
		markJobSucceeded(project.ProjectMigration1Name(project.NewContext(proj)) + "-job")
	}

	// ---- Phase 1: gating ----

	Context("when the DatabaseRef is missing", func() {
		It("reports Ready=False with reason DatabaseNotReady", func() {
			proj := newProject("p", "does-not-exist")
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proj.Name, Namespace: ns}, got)).To(Succeed())
				cond := apimeta.FindStatusCondition(got.Status.Conditions, reconciler.ConditionTypeReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("DatabaseNotReady"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})
	})

	// ---- Phase 2: shared resources on the happy path ----

	Context("when the database is ready", func() {
		It("creates the initial Migration with the configured Pod overlay", func() {
			driveSingleDatabaseReady("pg")
			proj := newProject("demo", "pg")
			proj.Spec.Migrations = &supabasev1alpha1.ProjectMigrationsSpec{
				Pod: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{NodeSelector: map[string]string{"workload": "migration"}},
				},
			}
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())

			migrationKey := types.NamespacedName{Name: project.ProjectMigration1Name(project.NewContext(proj)), Namespace: ns}
			Eventually(func(g Gomega) {
				migration := &supabasev1alpha1.Migration{}
				g.Expect(k8sClient.Get(ctx, migrationKey, migration)).To(Succeed())
				g.Expect(migration.Spec.Migrations).To(HaveLen(5))
				g.Expect(migration.Spec.Pod).NotTo(BeNil())
				g.Expect(migration.Spec.Pod.Spec.NodeSelector).To(HaveKeyWithValue("workload", "migration"))
				g.Expect(migration.OwnerReferences).To(HaveLen(1))
				g.Expect(migration.OwnerReferences[0].Kind).To(Equal("Project"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("waits for matching Migrations before creating Project resources", func() {
			driveSingleDatabaseReady("pg")
			migration := newMigration("bootstrap", "pg")
			Expect(k8sClient.Create(ctx, migration)).To(Succeed())
			waitForMigrationPending(migration.Name)

			proj := newProject("demo", "pg")
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proj.Name, Namespace: ns}, got)).To(Succeed())
				cond := apimeta.FindStatusCondition(got.Status.Conditions, reconciler.ConditionTypeReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("MigrationsNotReady"))
			}, defaultTimeout, defaultPolling).Should(Succeed())

			Consistently(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: project.JWTSecretName(project.NewContext(proj)), Namespace: ns}, &corev1.Secret{})
			}, 2*time.Second, defaultPolling).ShouldNot(Succeed())

			markJobSucceeded(migration.Name + "-job")
			markProjectMigrationSucceeded(proj)
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: project.JWTSecretName(project.NewContext(proj)), Namespace: ns}, &corev1.Secret{})
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("creates the JWT and Keys Secrets after the initial Migration succeeds", func() {
			driveSingleDatabaseReady("pg")
			proj := newProject("demo", "pg")
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())
			markProjectMigrationSucceeded(proj)

			jwtKey := types.NamespacedName{Name: project.JWTSecretName(project.NewContext(proj)), Namespace: ns}
			keysKey := types.NamespacedName{Name: project.KeysSecretName(project.NewContext(proj)), Namespace: ns}

			Eventually(func(g Gomega) {
				jwt := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, jwtKey, jwt)).To(Succeed())
				g.Expect(jwt.Data).To(HaveKey(project.JWTSecretKey))
				g.Expect(jwt.Data).To(HaveKey(project.JWTSecretAnonKey))
				g.Expect(jwt.Data).To(HaveKey(project.JWTSecretServiceKey))

				keys := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, keysKey, keys)).To(Succeed())
				g.Expect(keys.Data).NotTo(BeEmpty())
				g.Expect(keys.Data).To(HaveLen(4))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("creates the sync-jwt and sync-password Jobs and records hashes on success", func() {
			driveSingleDatabaseReady("pg")
			proj := newProject("demo", "pg")
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())
			markProjectMigrationSucceeded(proj)

			markJobSucceeded(project.SyncJWTJobName(project.NewContext(proj)))
			Eventually(func(g Gomega) {
				job := &batchv1.Job{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: project.SyncPasswordJobName(project.NewContext(proj)), Namespace: ns}, job)).To(Succeed())
				container := job.Spec.Template.Spec.Containers[0]
				g.Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: "PGDATABASE", Value: "_supabase"}))
				for _, env := range container.Env {
					g.Expect(env.Name).NotTo(Equal("ANALYTICS_BACKEND_URL"))
					if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
						g.Expect(env.Name).To(Equal("PGPASSWORD"))
					}
				}
			}, defaultTimeout, defaultPolling).Should(Succeed())

			markJobSucceeded(project.SyncPasswordJobName(project.NewContext(proj)))

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proj.Name, Namespace: ns}, got)).To(Succeed())
				g.Expect(got.Status.JWTSyncHash).NotTo(BeEmpty())
				g.Expect(got.Status.PasswordSyncHash).NotTo(BeEmpty())
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("returns to NotReady when a matching Migration is created", func() {
			driveSingleDatabaseReady("pg")
			proj := newProject("demo", "pg")
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())
			markProjectMigrationSucceeded(proj)
			markJobSucceeded(project.SyncJWTJobName(project.NewContext(proj)))
			markJobSucceeded(project.SyncPasswordJobName(project.NewContext(proj)))

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proj.Name, Namespace: ns}, got)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
			}, defaultTimeout, defaultPolling).Should(Succeed())

			migration := newMigration("late", "pg")
			Expect(k8sClient.Create(ctx, migration)).To(Succeed())
			waitForMigrationPending(migration.Name)

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proj.Name, Namespace: ns}, got)).To(Succeed())
				cond := apimeta.FindStatusCondition(got.Status.Conditions, reconciler.ConditionTypeReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Reason).To(Equal("MigrationsNotReady"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("does not create the initial Migration when disabled but still waits for external Migrations", func() {
			driveSingleDatabaseReady("pg")
			externalMigration := newMigration("external", "pg")
			Expect(k8sClient.Create(ctx, externalMigration)).To(Succeed())
			waitForMigrationPending(externalMigration.Name)

			disabled := false
			proj := newProject("demo", "pg")
			proj.Spec.Migrations = &supabasev1alpha1.ProjectMigrationsSpec{Enable: &disabled}
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())

			initialMigrationKey := types.NamespacedName{Name: project.ProjectMigration1Name(project.NewContext(proj)), Namespace: ns}
			Consistently(func() error {
				return k8sClient.Get(ctx, initialMigrationKey, &supabasev1alpha1.Migration{})
			}, 2*time.Second, defaultPolling).ShouldNot(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proj.Name, Namespace: ns}, got)).To(Succeed())
				condition := apimeta.FindStatusCondition(got.Status.Conditions, reconciler.ConditionTypeReady)
				g.Expect(condition).NotTo(BeNil())
				g.Expect(condition.Reason).To(Equal("MigrationsNotReady"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})
	})

	Context("shared PostgREST settings", func() {
		It("defaults optional fields and rejects invalid values", func() {
			for _, spec := range []*supabasev1alpha1.ProjectPostgRESTSpec{
				{Schemas: ptr.To("")}, {MaxRows: ptr.To(int64(0))}, {MaxRows: ptr.To(int64(-1))},
			} {
				proj := newProject("invalid-"+rand.String(6), "missing-db")
				proj.Spec.PostgREST = spec
				Expect(apierrors.IsInvalid(k8sClient.Create(ctx, proj))).To(BeTrue())
			}
			for _, spec := range []*supabasev1alpha1.ProjectPostgRESTSpec{
				{}, {MaxRows: ptr.To(int64(2000))}, {ExtraSearchPath: ptr.To("")},
			} {
				proj := newProject("defaults-"+rand.String(6), "missing-db")
				proj.Spec.PostgREST = spec
				Expect(k8sClient.Create(ctx, proj)).To(Succeed())
				got := &supabasev1alpha1.Project{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(proj), got)).To(Succeed())
				Expect(got.Spec.PostgREST.Schemas).To(HaveValue(Equal("public,graphql_public")))
				Expect(got.Spec.PostgREST.MaxRows).To(HaveValue(Equal(ptr.Deref(spec.MaxRows, 1000))))
				Expect(got.Spec.PostgREST.ExtraSearchPath).To(HaveValue(Equal(ptr.Deref(spec.ExtraSearchPath, "public"))))
			}
		})

		It("updates Rest and Studio when the Project changes", func() {
			driveSingleDatabaseReady("pg")
			proj := newProject("shared", "pg")
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())
			markProjectMigrationSucceeded(proj)
			markJobSucceeded(project.SyncJWTJobName(project.NewContext(proj)))
			markJobSucceeded(project.SyncPasswordJobName(project.NewContext(proj)))
			metadata := metav1.ObjectMeta{Name: "shared", Namespace: ns}
			ref := corev1.LocalObjectReference{Name: proj.Name}
			for _, component := range []client.Object{
				&supabasev1alpha1.Rest{ObjectMeta: metadata, Spec: supabasev1alpha1.RestSpec{ProjectRef: ref, Replicas: ptr.To(int32(0))}},
				&supabasev1alpha1.Studio{ObjectMeta: metadata, Spec: supabasev1alpha1.StudioSpec{ProjectRef: ref, Replicas: ptr.To(int32(0))}},
				&supabasev1alpha1.Meta{ObjectMeta: metadata, Spec: supabasev1alpha1.MetaSpec{ProjectRef: ref, Replicas: ptr.To(int32(0))}},
				&supabasev1alpha1.Envoy{ObjectMeta: metadata, Spec: supabasev1alpha1.EnvoySpec{ProjectRef: ref, Replicas: ptr.To(int32(0))}},
			} {
				Expect(k8sClient.Create(ctx, component)).To(Succeed())
			}
			checkWorkloads := func(schemas, maxRows, searchPath string) {
				Eventually(func(g Gomega) {
					rest := &appsv1.Deployment{}
					studio := &appsv1.StatefulSet{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "shared-rest", Namespace: ns}, rest)).To(Succeed())
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "shared-studio", Namespace: ns}, studio)).To(Succeed())
					for _, pod := range []corev1.PodSpec{rest.Spec.Template.Spec, studio.Spec.Template.Spec} {
						g.Expect(pod.Containers).NotTo(BeEmpty())
						g.Expect(pod.Containers[0].Env).To(ContainElements(
							corev1.EnvVar{Name: "PGRST_DB_SCHEMAS", Value: schemas},
							corev1.EnvVar{Name: "PGRST_DB_MAX_ROWS", Value: maxRows},
							corev1.EnvVar{Name: "PGRST_DB_EXTRA_SEARCH_PATH", Value: searchPath},
						))
					}
				}, defaultTimeout, defaultPolling).Should(Succeed())
			}
			checkWorkloads("public,graphql_public", "1000", "public")
			// Update only the Project; the manager's watches must reconcile both components.
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(proj), proj)).To(Succeed())
				proj.Spec.PostgREST = &supabasev1alpha1.ProjectPostgRESTSpec{Schemas: ptr.To("public,custom"), MaxRows: ptr.To(int64(2000)), ExtraSearchPath: ptr.To("")}
				g.Expect(k8sClient.Update(ctx, proj)).To(Succeed())
			}, defaultTimeout, defaultPolling).Should(Succeed())
			checkWorkloads("public,custom", "2000", "")
		})
	})

	// ---- Phase 3: one enabled component end-to-end ----

	Context("when only the Rest component is enabled", func() {
		It("creates the Rest Service and Deployment, blocks on readiness, and reports Ready=True once the Deployment is ready", func() {
			driveSingleDatabaseReady("pg")
			proj := newProject("demo", "pg")
			rest := &supabasev1alpha1.Rest{ObjectMeta: metav1.ObjectMeta{Name: "custom-api", Namespace: ns}, Spec: supabasev1alpha1.RestSpec{ProjectRef: corev1.LocalObjectReference{Name: proj.Name}}}
			Expect(k8sClient.Create(ctx, rest)).To(Succeed())
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())
			markProjectMigrationSucceeded(proj)

			markJobSucceeded(project.SyncJWTJobName(project.NewContext(proj)))
			markJobSucceeded(project.SyncPasswordJobName(project.NewContext(proj)))

			restSvcKey := types.NamespacedName{Name: "custom-api-rest", Namespace: ns}
			restDeployKey := types.NamespacedName{Name: "custom-api-rest", Namespace: ns}

			Eventually(func(g Gomega) {
				svc := &corev1.Service{}
				g.Expect(k8sClient.Get(ctx, restSvcKey, svc)).To(Succeed())
				g.Expect(svc.Spec.Ports).NotTo(BeEmpty())
				g.Expect(svc.Spec.Ports[0].Port).To(Equal(restdefaults.DefaultRestPort))

				d := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, restDeployKey, d)).To(Succeed())
				g.Expect(d.Spec.Template.Spec.Containers).To(HaveLen(1))
				g.Expect(d.Spec.Template.Spec.Containers[0].Name).To(Equal("rest"))

				envMap := map[string]bool{}
				for _, e := range d.Spec.Template.Spec.Containers[0].Env {
					envMap[e.Name] = true
				}
				g.Expect(envMap).To(HaveKey("PGRST_JWT_SECRET"))
			}, defaultTimeout, defaultPolling).Should(Succeed())

			// Component readiness is independent of shared Project readiness.
			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proj.Name, Namespace: ns}, got)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, "Ready")).To(BeTrue())
				component := &supabasev1alpha1.Rest{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: rest.Name, Namespace: ns}, component)).To(Succeed())
				g.Expect(apimeta.FindStatusCondition(component.Status.Conditions, "Ready").Reason).To(Equal("WorkloadNotReady"))
			}, defaultTimeout, defaultPolling).Should(Succeed())

			markDeploymentReady("custom-api-rest")
			Eventually(func(g Gomega) {
				component := &supabasev1alpha1.Rest{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: rest.Name, Namespace: ns}, component)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(component.Status.Conditions, "Ready")).To(BeTrue())
			}, defaultTimeout, defaultPolling).Should(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proj.Name, Namespace: ns}, got)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
				cond := apimeta.FindStatusCondition(got.Status.Conditions, reconciler.ConditionTypeReady)
				g.Expect(cond.Reason).To(Equal("ReconcileSucceeded"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})
	})
})
