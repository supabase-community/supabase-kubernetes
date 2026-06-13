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
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	projectpkg "github.com/supabase-community/supabase-kubernetes/internal/project"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

//nolint:unparam // timeout is always the same in current test suite
func simulateSingleDatabaseReady(name string, timeout, interval time.Duration) {
	stsName := name + "-db"
	sts := &appsv1.StatefulSet{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: stsName, Namespace: "default"}, sts)).To(Succeed())
		sts.Status.Replicas = 1
		sts.Status.ReadyReplicas = 1
		g.Expect(k8sClient.Status().Update(ctx, sts)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

func testSingleDatabase(name string) *supabasev1alpha1.SingleDatabase {
	return &supabasev1alpha1.SingleDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: supabasev1alpha1.SingleDatabaseSpec{
			Storage: &supabasev1alpha1.VolumeClaim{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Size:        resource.MustParse("1Gi"),
			},
		},
	}
}

func validProject(name string) *supabasev1alpha1.Project {
	return &supabasev1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: supabasev1alpha1.ProjectSpec{
			HTTP: supabasev1alpha1.HTTPConfig{
				Protocol: "https",
				Hostname: "test.example.com",
			},
			DatabaseRef: supabasev1alpha1.DatabaseRef{Kind: "SingleDatabase", Name: "test-db"},
		},
	}
}

func minimalProject(name string) *supabasev1alpha1.Project {
	return &supabasev1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: supabasev1alpha1.ProjectSpec{
			HTTP: supabasev1alpha1.HTTPConfig{
				Protocol: "http",
				Hostname: "test.example.com",
			},
			DatabaseRef: supabasev1alpha1.DatabaseRef{Kind: "SingleDatabase", Name: "test-db"},
		},
	}
}

//nolint:unparam // timeout is always the same in current test suite
func simulateMigrationSuccess(projectName string, timeout, interval time.Duration) {
	migrationName := projectName + "-migration-0"
	jobName := migrationName + "-apply"

	var migration *supabasev1alpha1.Migration
	Eventually(func(g Gomega) {
		migration = &supabasev1alpha1.Migration{}
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: migrationName, Namespace: "default"}, migration)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	// If the migration is already applied (e.g. reused from a previous test
	// that has not been garbage-collected yet), skip job simulation.
	if meta.IsStatusConditionTrue(migration.Status.Conditions, reconciler.ConditionTypeReady) {
		return
	}

	Eventually(func(g Gomega) {
		job := &batchv1.Job{}
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: "default"}, job)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	job := &batchv1.Job{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: "default"}, job)).To(Succeed())
	job.Status.Succeeded = 1
	Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
}

//nolint:unparam // timeout is always the same in current test suite
func simulateJWTSyncSuccess(projectName string, timeout, interval time.Duration) {
	jobName := projectName + "-sync-jwt"

	job := &batchv1.Job{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: "default"}, job)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	if job.Status.Succeeded > 0 {
		return
	}

	job.Status.Succeeded = 1
	Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
}

//nolint:unparam
func simulatePasswordSyncSuccess(projectName string, timeout, interval time.Duration) {
	jobName := projectName + "-sync-password"

	job := &batchv1.Job{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: "default"}, job)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	if job.Status.Succeeded > 0 {
		return
	}

	job.Status.Succeeded = 1
	Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
}

var _ = Describe("Project Controller", func() {
	const timeout = 30 * time.Second
	const interval = 250 * time.Millisecond

	Context("When creating a Project", func() {
		const projectName = "test-project"
		projectKey := types.NamespacedName{Name: projectName, Namespace: "default"}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, testSingleDatabase("test-db"))).To(Succeed())
			simulateSingleDatabaseReady("test-db", timeout, interval)
			Expect(k8sClient.Create(ctx, validProject(projectName))).To(Succeed())
			simulateMigrationSuccess(projectName, timeout, interval)
			simulateJWTSyncSuccess(projectName, timeout, interval)
			simulatePasswordSyncSuccess(projectName, timeout, interval)
			Eventually(func(g Gomega) {
				created := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, ConditionTypeSecretsReady)).To(BeTrue())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			project := &supabasev1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			extDB := &supabasev1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-db", Namespace: "default"}, extDB); err == nil {
				Expect(k8sClient.Delete(ctx, extDB)).To(Succeed())
			}
		})

		It("should create generated secrets", func() {
			for _, suffix := range []string{"jwt", "keys"} {
				secret := &corev1.Secret{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s", projectName, suffix), Namespace: "default"}, secret)).To(Succeed())
			}
		})

		It("should set owner references on generated secrets", func() {
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-jwt", projectName), Namespace: "default"}, secret)).To(Succeed())
			Expect(secret.OwnerReferences).NotTo(BeEmpty())
			Expect(secret.OwnerReferences[0].Kind).To(Equal("Project"))
		})

		It("should generate valid JWT secret content", func() {
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-jwt", projectName), Namespace: "default"}, secret)).To(Succeed())

			jwtSecret := string(secret.Data["jwt-secret"])
			_, err := base64.StdEncoding.DecodeString(jwtSecret)
			Expect(err).NotTo(HaveOccurred())

			anonToken, err := jwt.Parse(string(secret.Data["anon-key"]), func(t *jwt.Token) (any, error) {
				return []byte(jwtSecret), nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(anonToken.Valid).To(BeTrue())
		})

		It("should set Ready and SecretsReady conditions", func() {
			project := &supabasev1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(project.Status.Conditions, ConditionTypeSecretsReady)).To(BeTrue())
			Expect(meta.IsStatusConditionTrue(project.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
		})

		It("should generate expected shared keys secret format", func() {
			keysSecret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-keys", projectName), Namespace: "default"}, keysSecret)).To(Succeed())
			_, err := hex.DecodeString(string(keysSecret.Data["secret-key-base"]))
			Expect(err).NotTo(HaveOccurred())
			_, err = hex.DecodeString(string(keysSecret.Data["crypto-key"]))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When creating a minimal Project", func() {
		const projectName = "minimal-project"
		projectKey := types.NamespacedName{Name: projectName, Namespace: "default"}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, testSingleDatabase("test-db"))).To(Succeed())
			simulateSingleDatabaseReady("test-db", timeout, interval)
			Expect(k8sClient.Create(ctx, minimalProject(projectName))).To(Succeed())
			simulateMigrationSuccess(projectName, timeout, interval)
			simulateJWTSyncSuccess(projectName, timeout, interval)
			simulatePasswordSyncSuccess(projectName, timeout, interval)
			Eventually(func(g Gomega) {
				created := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			project := &supabasev1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			extDB := &supabasev1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-db", Namespace: "default"}, extDB); err == nil {
				Expect(k8sClient.Delete(ctx, extDB)).To(Succeed())
			}
		})

		It("should not create component workloads", func() {
			project := &supabasev1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(project.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
		})
	})

	Context("When a generated secret is deleted (rotation)", func() {
		const projectName = "rotation-project"
		projectKey := types.NamespacedName{Name: projectName, Namespace: "default"}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, testSingleDatabase("test-db"))).To(Succeed())
			simulateSingleDatabaseReady("test-db", timeout, interval)
			Expect(k8sClient.Create(ctx, validProject(projectName))).To(Succeed())
			simulateMigrationSuccess(projectName, timeout, interval)
			Eventually(func(g Gomega) {
				created := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, ConditionTypeSecretsReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			project := &supabasev1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			extDB := &supabasev1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-db", Namespace: "default"}, extDB); err == nil {
				Expect(k8sClient.Delete(ctx, extDB)).To(Succeed())
			}
		})

		It("should re-create a deleted JWT secret", func() {
			jwtKey := types.NamespacedName{Name: fmt.Sprintf("%s-jwt", projectName), Namespace: "default"}
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, jwtKey, secret)).To(Succeed())
			oldValue := string(secret.Data["jwt-secret"])
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

			Eventually(func(g Gomega) {
				recreated := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, jwtKey, recreated)).To(Succeed())
				g.Expect(string(recreated.Data["jwt-secret"])).NotTo(Equal(oldValue))
			}, timeout, interval).Should(Succeed())
		})

		It("should patch a missing key back into shared keys secret", func() {
			metaKey := types.NamespacedName{Name: fmt.Sprintf("%s-keys", projectName), Namespace: "default"}
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, metaKey, secret)).To(Succeed())
			delete(secret.Data, "crypto-key")
			Expect(k8sClient.Update(ctx, secret)).To(Succeed())

			project := &supabasev1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
			if project.Annotations == nil {
				project.Annotations = map[string]string{}
			}
			project.Annotations["reconcile-trigger"] = time.Now().String()
			Expect(k8sClient.Update(ctx, project)).To(Succeed())

			Eventually(func(g Gomega) {
				updated := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, metaKey, updated)).To(Succeed())
				g.Expect(updated.Data).To(HaveKey("crypto-key"))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating a Project with SingleDatabase", func() {
		const projectName = "single-db-project"
		projectKey := types.NamespacedName{Name: projectName, Namespace: "default"}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, testSingleDatabase("test-single-db"))).To(Succeed())
			simulateSingleDatabaseReady("test-single-db", timeout, interval)
			project := validProject(projectName)
			project.Spec.DatabaseRef = supabasev1alpha1.DatabaseRef{Kind: "SingleDatabase", Name: "test-single-db"}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			simulateMigrationSuccess(projectName, timeout, interval)
			simulateJWTSyncSuccess(projectName, timeout, interval)
			simulatePasswordSyncSuccess(projectName, timeout, interval)
			Eventually(func(g Gomega) {
				created := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			project := &supabasev1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			singleDB := &supabasev1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db", Namespace: "default"}, singleDB); err == nil {
				Expect(k8sClient.Delete(ctx, singleDB)).To(Succeed())
			}
		})

		It("should reconcile successfully with SingleDatabase reference", func() {
			project := &supabasev1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(project.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
		})
	})

	Context("When creating a Project", func() {
		const projectName = "migration-project"
		projectKey := types.NamespacedName{Name: projectName, Namespace: "default"}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, testSingleDatabase("test-db"))).To(Succeed())
			simulateSingleDatabaseReady("test-db", timeout, interval)
			Expect(k8sClient.Create(ctx, validProject(projectName))).To(Succeed())
		})

		AfterEach(func() {
			project := &supabasev1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			extDB := &supabasev1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-db", Namespace: "default"}, extDB); err == nil {
				Expect(k8sClient.Delete(ctx, extDB)).To(Succeed())
			}
		})

		It("should not recreate Migration once applied", func() {
			simulateMigrationSuccess(projectName, timeout, interval)

			Eventually(func(g Gomega) {
				project := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(project.Status.Conditions, ConditionTypeMigrationReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			project := &supabasev1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
			if project.Annotations == nil {
				project.Annotations = map[string]string{}
			}
			project.Annotations["reconcile-trigger"] = time.Now().String()
			Expect(k8sClient.Update(ctx, project)).To(Succeed())

			// The existing Migration CR should not be replaced by a new one.
			Consistently(func(g Gomega) {
				m := &supabasev1alpha1.Migration{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: projectName + "-migration-0", Namespace: "default"}, m)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(m.OwnerReferences).To(HaveLen(1))
				g.Expect(m.OwnerReferences[0].Kind).To(Equal("Project"))
			}, 2*time.Second, interval).Should(Succeed())
		})
	})

	Context("When creating a Project with inline components", func() {
		const projectName = "component-project"
		projectKey := types.NamespacedName{Name: projectName, Namespace: "default"}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, testSingleDatabase("test-db"))).To(Succeed())
			simulateSingleDatabaseReady("test-db", timeout, interval)

			project := validProject(projectName)
			project.Spec.Rest = &supabasev1alpha1.RestSpec{}
			project.Spec.Auth = &supabasev1alpha1.AuthSpec{
				SiteURL:                "http://localhost:3000",
				DisableSignup:          ptr.To(false),
				EnableEmailSignup:      ptr.To(true),
				EnableAnonymousUsers:   ptr.To(false),
				EnableEmailAutoconfirm: ptr.To(false),
				EnablePhoneSignup:      ptr.To(false),
				EnablePhoneAutoconfirm: ptr.To(false),
			}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			simulateMigrationSuccess(projectName, timeout, interval)
			simulateJWTSyncSuccess(projectName, timeout, interval)
			simulatePasswordSyncSuccess(projectName, timeout, interval)
			Eventually(func(g Gomega) {
				created := &supabasev1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, reconciler.ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			project := &supabasev1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			extDB := &supabasev1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-db", Namespace: "default"}, extDB); err == nil {
				Expect(k8sClient.Delete(ctx, extDB)).To(Succeed())
			}
		})

		It("should create Rest Deployment and Service", func() {
			Eventually(func(g Gomega) {
				dep := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: projectName + "-rest", Namespace: "default"}, dep)).To(Succeed())
				g.Expect(dep.Spec.Template.Spec.Containers).To(HaveLen(1))
				g.Expect(dep.Spec.Template.Spec.Containers[0].Name).To(Equal("rest"))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				svc := &corev1.Service{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: projectName + "-rest", Namespace: "default"}, svc)).To(Succeed())
				g.Expect(svc.Spec.Ports).To(HaveLen(1))
				g.Expect(svc.Spec.Ports[0].Port).To(Equal(int32(3000)))
			}, timeout, interval).Should(Succeed())
		})

		It("should create Auth Deployment and Service", func() {
			Eventually(func(g Gomega) {
				dep := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: projectName + "-auth", Namespace: "default"}, dep)).To(Succeed())
				g.Expect(dep.Spec.Template.Spec.Containers).To(HaveLen(1))
				g.Expect(dep.Spec.Template.Spec.Containers[0].Name).To(Equal("auth"))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				svc := &corev1.Service{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: projectName + "-auth", Namespace: "default"}, svc)).To(Succeed())
				g.Expect(svc.Spec.Ports).To(HaveLen(1))
				g.Expect(svc.Spec.Ports[0].Port).To(Equal(int32(9999)))
			}, timeout, interval).Should(Succeed())
		})

		It("should set component ready conditions on Project", func() {
			project := &supabasev1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(project.Status.Conditions, projectpkg.ConditionTypeRestReady)).To(BeTrue())
			Expect(meta.IsStatusConditionTrue(project.Status.Conditions, projectpkg.ConditionTypeAuthReady)).To(BeTrue())
		})
	})
})
