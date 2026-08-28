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
	"github.com/supabase-community/supabase-kubernetes/internal/project"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
	"github.com/supabase-community/supabase-kubernetes/internal/singledatabase"
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
	driveSingleDatabaseReady := func(name string, image ...string) *supabasev1alpha1.SingleDatabase {
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
		if len(image) > 0 && image[0] != "" {
			db.Spec.Image = &image[0]
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

	// driveChildMigrationReady waits for the Project's child Migration to be
	// created, then marks its Job as succeeded so that the Migration controller
	// itself flips it to Ready=True. This is a stable steady state.
	driveChildMigrationReady := func(migrationName string) {
		migKey := types.NamespacedName{Name: migrationName, Namespace: ns}
		Eventually(func() error {
			return k8sClient.Get(ctx, migKey, &supabasev1alpha1.Migration{})
		}, defaultTimeout, defaultPolling).Should(Succeed())

		// The Migration controller creates a Job named "<migration>-job".
		jobName := migrationName + "-job"
		markJobSucceeded(jobName)

		Eventually(func(g Gomega) {
			got := &supabasev1alpha1.Migration{}
			g.Expect(k8sClient.Get(ctx, migKey, got)).To(Succeed())
			g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
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
			d.Status.ReadyReplicas = replicas
			d.Status.Replicas = replicas
			d.Status.AvailableReplicas = replicas
			d.Status.ObservedGeneration = d.Generation
			g.Expect(k8sClient.Status().Update(ctx, d)).To(Succeed())
		}, defaultTimeout, defaultPolling).Should(Succeed())
	}

	// newProject builds a minimal Project. Every component is explicitly
	// disabled so the test can opt in to only what it needs.
	newProject := func(name, dbName string) *supabasev1alpha1.Project {
		return &supabasev1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: supabasev1alpha1.ProjectSpec{
				HTTP: supabasev1alpha1.HTTPConfig{
					Protocol: "http",
					Hostname: "api.test.local",
				},
				DatabaseRef: supabasev1alpha1.DatabaseRef{
					Kind: "SingleDatabase",
					Name: dbName,
				},
				Auth: &supabasev1alpha1.AuthSpec{
					Enable:  ptr.To(false),
					SiteURL: "https://example.com",
				},
				Rest:     &supabasev1alpha1.RestSpec{Enable: ptr.To(false)},
				Meta:     &supabasev1alpha1.MetaSpec{Enable: ptr.To(false)},
				Realtime: &supabasev1alpha1.RealtimeSpec{Enable: ptr.To(false)},
				Envoy:    &supabasev1alpha1.EnvoySpec{Enable: ptr.To(false)},
				// Storage and Studio require their own VolumeClaim even when
				// disabled, so omit them entirely.
				Functions: &supabasev1alpha1.FunctionsSpec{
					Enable:    ptr.To(false),
					VerifyJWT: false,
				},
			},
		}
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

	// ---- Phase 2: child resources on the happy path ----

	Context("when the database is ready", func() {
		It("creates the child Migration CR with the expected entries", func() {
			driveSingleDatabaseReady("pg")
			proj := newProject("demo", "pg")
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())

			migKey := types.NamespacedName{
				Name:      project.ProjectMigration1Name(proj),
				Namespace: ns,
			}
			Eventually(func(g Gomega) {
				mig := &supabasev1alpha1.Migration{}
				g.Expect(k8sClient.Get(ctx, migKey, mig)).To(Succeed())
				g.Expect(mig.Spec.DatabaseRef.Kind).To(Equal("SingleDatabase"))
				g.Expect(mig.Spec.DatabaseRef.Name).To(Equal("pg"))
				g.Expect(mig.Spec.Migrations).To(HaveLen(5))
				names := make([]string, 0, len(mig.Spec.Migrations))
				for _, m := range mig.Spec.Migrations {
					names = append(names, m.Name)
				}
				g.Expect(names).To(ConsistOf("supabase.sql", "realtime.sql", "logs.sql", "pooler.sql", "webhooks.sql"))
				g.Expect(mig.OwnerReferences).To(HaveLen(1))
				g.Expect(mig.OwnerReferences[0].Kind).To(Equal("Project"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("reports Ready=False with reason MigrationNotReady while the child Migration is pending", func() {
			driveSingleDatabaseReady("pg")
			proj := newProject("demo", "pg")
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proj.Name, Namespace: ns}, got)).To(Succeed())
				cond := apimeta.FindStatusCondition(got.Status.Conditions, reconciler.ConditionTypeReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("MigrationNotReady"))
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("creates the JWT and Keys Secrets after the child Migration becomes ready", func() {
			driveSingleDatabaseReady("pg")
			proj := newProject("demo", "pg")
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())
			driveChildMigrationReady(project.ProjectMigration1Name(proj))

			jwtKey := types.NamespacedName{Name: project.JWTSecretName(proj), Namespace: ns}
			keysKey := types.NamespacedName{Name: project.KeysSecretName(proj), Namespace: ns}

			Eventually(func(g Gomega) {
				jwt := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, jwtKey, jwt)).To(Succeed())
				g.Expect(jwt.Data).To(HaveKey(project.JWTSecretKey))
				g.Expect(jwt.Data).To(HaveKey(project.JWTSecretAnonKey))
				g.Expect(jwt.Data).To(HaveKey(project.JWTSecretServiceKey))

				keys := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, keysKey, keys)).To(Succeed())
				g.Expect(keys.Data).NotTo(BeEmpty())
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("creates the sync-jwt and sync-password Jobs and records hashes on success", func() {
			db := driveSingleDatabaseReady("pg")
			proj := newProject("demo", "pg")
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())
			driveChildMigrationReady(project.ProjectMigration1Name(proj))

			// With no custom database image, each Job inherits the resolved
			// database image (here the default Postgres image). sync-password
			// is only created after sync-jwt succeeds, so assert and advance
			// them in order.
			expectJobImage := func(name, image string) {
				Eventually(func(g Gomega) {
					job := &batchv1.Job{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, job)).To(Succeed())
					g.Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal(image))
				}, defaultTimeout, defaultPolling).Should(Succeed())
			}

			expectJobImage(project.SyncJWTJobName(proj), db.Status.ResolvedDatabase.Image)
			markJobSucceeded(project.SyncJWTJobName(proj))
			expectJobImage(project.SyncPasswordJobName(proj), db.Status.ResolvedDatabase.Image)
			markJobSucceeded(project.SyncPasswordJobName(proj))

			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proj.Name, Namespace: ns}, got)).To(Succeed())
				g.Expect(got.Status.JwtSyncHash).NotTo(BeEmpty())
				g.Expect(got.Status.PasswordSyncHash).NotTo(BeEmpty())
			}, defaultTimeout, defaultPolling).Should(Succeed())
		})

		It("sync Jobs inherit a custom database image", func() {
			const dbImage = "example.com/supabase/postgres:custom"
			driveSingleDatabaseReady("pg", dbImage)
			proj := newProject("demo", "pg")
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())
			driveChildMigrationReady(project.ProjectMigration1Name(proj))

			expectJobImage := func(name string) {
				Eventually(func(g Gomega) {
					job := &batchv1.Job{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, job)).To(Succeed())
					g.Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal(dbImage))
				}, defaultTimeout, defaultPolling).Should(Succeed())
			}

			// sync-password is created only after sync-jwt succeeds.
			expectJobImage(project.SyncJWTJobName(proj))
			markJobSucceeded(project.SyncJWTJobName(proj))
			expectJobImage(project.SyncPasswordJobName(proj))
		})
	})

	// ---- Phase 3: one enabled component end-to-end ----

	Context("when only the Rest component is enabled", func() {
		It("creates the Rest Service and Deployment, blocks on readiness, and reports Ready=True once the Deployment is ready", func() {
			driveSingleDatabaseReady("pg")
			proj := newProject("demo", "pg")
			proj.Spec.Rest = &supabasev1alpha1.RestSpec{Enable: ptr.To(true)}
			Expect(k8sClient.Create(ctx, proj)).To(Succeed())

			driveChildMigrationReady(project.ProjectMigration1Name(proj))
			markJobSucceeded(project.SyncJWTJobName(proj))
			markJobSucceeded(project.SyncPasswordJobName(proj))

			restSvcKey := types.NamespacedName{Name: project.RestServiceName(proj), Namespace: ns}
			restDeployKey := types.NamespacedName{Name: project.RestDeploymentName(proj), Namespace: ns}

			Eventually(func(g Gomega) {
				svc := &corev1.Service{}
				g.Expect(k8sClient.Get(ctx, restSvcKey, svc)).To(Succeed())
				g.Expect(svc.Spec.Ports).NotTo(BeEmpty())
				g.Expect(svc.Spec.Ports[0].Port).To(Equal(project.DefaultRestPort))

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

			// While the deployment has no ready replicas the project should be
			// blocked with reason DeploymentsNotReady.
			Eventually(func(g Gomega) {
				got := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proj.Name, Namespace: ns}, got)).To(Succeed())
				cond := apimeta.FindStatusCondition(got.Status.Conditions, reconciler.ConditionTypeReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("DeploymentsNotReady"))
			}, defaultTimeout, defaultPolling).Should(Succeed())

			markDeploymentReady(project.RestDeploymentName(proj))

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
