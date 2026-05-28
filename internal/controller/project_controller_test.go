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

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
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

func testSingleDatabase(name string) *platformv1alpha1.SingleDatabase {
	return &platformv1alpha1.SingleDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: platformv1alpha1.SingleDatabaseSpec{
			Version: "2026.04.27",
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
}

func validProject(name string) *platformv1alpha1.Project {
	return &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: platformv1alpha1.ProjectSpec{
			Version: "2026.04.27",
			HTTP: platformv1alpha1.HTTPConfig{
				Protocol: "https",
				Hostname: "test.example.com",
			},
			DatabaseRef: platformv1alpha1.DatabaseRef{Kind: "SingleDatabase", Name: "test-db"},
		},
	}
}

func minimalProject(name string) *platformv1alpha1.Project {
	return &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: platformv1alpha1.ProjectSpec{
			Version: "2026.04.27",
			HTTP: platformv1alpha1.HTTPConfig{
				Protocol: "http",
				Hostname: "test.example.com",
			},
			DatabaseRef: platformv1alpha1.DatabaseRef{Kind: "SingleDatabase", Name: "test-db"},
		},
	}
}

//nolint:unparam // timeout is always the same in current test suite
func simulateMigrationSuccess(projectName string, timeout, interval time.Duration) {
	migrationName := projectName + "-migration"
	jobName := migrationName + "-apply"

	var migration *platformv1alpha1.Migration
	Eventually(func(g Gomega) {
		migration = &platformv1alpha1.Migration{}
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: migrationName, Namespace: "default"}, migration)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	// If the migration is already applied (e.g. reused from a previous test
	// that has not been garbage-collected yet), skip job simulation.
	if meta.IsStatusConditionTrue(migration.Status.Conditions, ConditionTypeReady) {
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
			Eventually(func(g Gomega) {
				created := &platformv1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, ConditionTypeSecretsReady)).To(BeTrue())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			project := &platformv1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			extDB := &platformv1alpha1.SingleDatabase{}
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
			project := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(project.Status.Conditions, ConditionTypeSecretsReady)).To(BeTrue())
			Expect(meta.IsStatusConditionTrue(project.Status.Conditions, ConditionTypeReady)).To(BeTrue())
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
			Eventually(func(g Gomega) {
				created := &platformv1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			project := &platformv1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			extDB := &platformv1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-db", Namespace: "default"}, extDB); err == nil {
				Expect(k8sClient.Delete(ctx, extDB)).To(Succeed())
			}
		})

		It("should not create component workloads", func() {
			project := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(project.Status.Conditions, ConditionTypeReady)).To(BeTrue())
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
				created := &platformv1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, ConditionTypeSecretsReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			project := &platformv1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			extDB := &platformv1alpha1.SingleDatabase{}
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

			project := &platformv1alpha1.Project{}
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
			project.Spec.DatabaseRef = platformv1alpha1.DatabaseRef{Kind: "SingleDatabase", Name: "test-single-db"}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			simulateMigrationSuccess(projectName, timeout, interval)
			Eventually(func(g Gomega) {
				created := &platformv1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			project := &platformv1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			singleDB := &platformv1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-single-db", Namespace: "default"}, singleDB); err == nil {
				Expect(k8sClient.Delete(ctx, singleDB)).To(Succeed())
			}
		})

		It("should reconcile successfully with SingleDatabase reference", func() {
			project := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(project.Status.Conditions, ConditionTypeReady)).To(BeTrue())
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
			project := &platformv1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			extDB := &platformv1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-db", Namespace: "default"}, extDB); err == nil {
				Expect(k8sClient.Delete(ctx, extDB)).To(Succeed())
			}
		})

		It("should create built-in Migration and apply it before Ready", func() {
			migrationName := projectName + "-migration"

			Eventually(func(g Gomega) {
				m := &platformv1alpha1.Migration{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: migrationName, Namespace: "default"}, m)).To(Succeed())
				g.Expect(m.Spec.Migrations).To(HaveLen(6))
				g.Expect(m.Spec.DatabaseRef.Name).To(Equal("test-db"))
			}, timeout, interval).Should(Succeed())

			simulateMigrationSuccess(projectName, timeout, interval)

			Eventually(func(g Gomega) {
				project := &platformv1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(project.Status.Conditions, ConditionTypeReady)).To(BeTrue())
				g.Expect(project.Status.AppliedMigrationHash).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})

		It("should not recreate Migration if AppliedMigrationHash is set", func() {
			simulateMigrationSuccess(projectName, timeout, interval)

			Eventually(func(g Gomega) {
				project := &platformv1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
				g.Expect(project.Status.AppliedMigrationHash).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())

			project := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
			if project.Annotations == nil {
				project.Annotations = map[string]string{}
			}
			project.Annotations["reconcile-trigger"] = time.Now().String()
			Expect(k8sClient.Update(ctx, project)).To(Succeed())

			// The existing Migration CR should not be replaced by a new one.
			Consistently(func(g Gomega) {
				m := &platformv1alpha1.Migration{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: projectName + "-migration", Namespace: "default"}, m)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(m.OwnerReferences).To(HaveLen(1))
				g.Expect(m.OwnerReferences[0].Kind).To(Equal("Project"))
			}, 2*time.Second, interval).Should(Succeed())
		})
	})

	Context("When creating a Project with component refs", func() {
		const projectName = "component-project"
		projectKey := types.NamespacedName{Name: projectName, Namespace: "default"}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, testSingleDatabase("test-db"))).To(Succeed())
			simulateSingleDatabaseReady("test-db", timeout, interval)

			rest := &platformv1alpha1.Rest{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rest", Namespace: "default"},
				Spec:       platformv1alpha1.RestSpec{},
			}
			Expect(k8sClient.Create(ctx, rest)).To(Succeed())

			auth := &platformv1alpha1.Auth{
				ObjectMeta: metav1.ObjectMeta{Name: "test-auth", Namespace: "default"},
				Spec: platformv1alpha1.AuthSpec{
					SiteURL:                "http://localhost:3000",
					DisableSignup:          false,
					EnableEmailSignup:      true,
					EnableAnonymousUsers:   false,
					EnableEmailAutoconfirm: false,
					EnablePhoneSignup:      false,
					EnablePhoneAutoconfirm: false,
				},
			}
			Expect(k8sClient.Create(ctx, auth)).To(Succeed())

			project := validProject(projectName)
			project.Spec.RestRef = &platformv1alpha1.RestRef{Kind: "Rest", Name: "test-rest"}
			project.Spec.AuthRef = &platformv1alpha1.AuthRef{Kind: "Auth", Name: "test-auth"}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())

			simulateMigrationSuccess(projectName, timeout, interval)
			Eventually(func(g Gomega) {
				created := &platformv1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			project := &platformv1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			extDB := &platformv1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-db", Namespace: "default"}, extDB); err == nil {
				Expect(k8sClient.Delete(ctx, extDB)).To(Succeed())
			}
			rest := &platformv1alpha1.Rest{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-rest", Namespace: "default"}, rest); err == nil {
				Expect(k8sClient.Delete(ctx, rest)).To(Succeed())
			}
			auth := &platformv1alpha1.Auth{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-auth", Namespace: "default"}, auth); err == nil {
				Expect(k8sClient.Delete(ctx, auth)).To(Succeed())
			}
		})

		It("should create Rest Deployment and Service", func() {
			Eventually(func(g Gomega) {
				dep := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-rest-rest", Namespace: "default"}, dep)).To(Succeed())
				g.Expect(dep.Spec.Template.Spec.Containers).To(HaveLen(1))
				g.Expect(dep.Spec.Template.Spec.Containers[0].Name).To(Equal("rest"))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				svc := &corev1.Service{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-rest-rest", Namespace: "default"}, svc)).To(Succeed())
				g.Expect(svc.Spec.Ports).To(HaveLen(1))
				g.Expect(svc.Spec.Ports[0].Port).To(Equal(int32(3000)))
			}, timeout, interval).Should(Succeed())
		})

		It("should create Auth Deployment and Service", func() {
			Eventually(func(g Gomega) {
				dep := &appsv1.Deployment{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-auth-auth", Namespace: "default"}, dep)).To(Succeed())
				g.Expect(dep.Spec.Template.Spec.Containers).To(HaveLen(1))
				g.Expect(dep.Spec.Template.Spec.Containers[0].Name).To(Equal("auth"))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				svc := &corev1.Service{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-auth-auth", Namespace: "default"}, svc)).To(Succeed())
				g.Expect(svc.Spec.Ports).To(HaveLen(1))
				g.Expect(svc.Spec.Ports[0].Port).To(Equal(int32(9999)))
			}, timeout, interval).Should(Succeed())
		})

		It("should set component ready conditions on Project", func() {
			project := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(project.Status.Conditions, ConditionTypeRestReady)).To(BeTrue())
			Expect(meta.IsStatusConditionTrue(project.Status.Conditions, ConditionTypeAuthReady)).To(BeTrue())
		})
	})

	Context("When creating a Project with a missing component ref", func() {
		const projectName = "missing-component-project"
		projectKey := types.NamespacedName{Name: projectName, Namespace: "default"}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, testSingleDatabase("test-db"))).To(Succeed())
			simulateSingleDatabaseReady("test-db", timeout, interval)

			project := validProject(projectName)
			project.Spec.RestRef = &platformv1alpha1.RestRef{Kind: "Rest", Name: "missing-rest"}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			simulateMigrationSuccess(projectName, timeout, interval)
		})

		AfterEach(func() {
			project := &platformv1alpha1.Project{}
			if err := k8sClient.Get(ctx, projectKey, project); err == nil {
				Expect(k8sClient.Delete(ctx, project)).To(Succeed())
			}
			extDB := &platformv1alpha1.SingleDatabase{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-db", Namespace: "default"}, extDB); err == nil {
				Expect(k8sClient.Delete(ctx, extDB)).To(Succeed())
			}
		})

		It("should set RestReady condition to False", func() {
			Eventually(func(g Gomega) {
				project := &platformv1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, project)).To(Succeed())
				cond := meta.FindStatusCondition(project.Status.Conditions, ConditionTypeRestReady)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal("ComponentNotFound"))
			}, timeout, interval).Should(Succeed())
		})
	})
})
