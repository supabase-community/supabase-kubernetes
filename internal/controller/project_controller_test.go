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
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func validProject(name string) *platformv1alpha1.Project {
	return &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: platformv1alpha1.ProjectSpec{
			Version: "2026.04.27",
			Global:  platformv1alpha1.GlobalSpec{SiteURL: "https://test.example.com"},
			HTTP: platformv1alpha1.HTTPSpec{
				API: platformv1alpha1.HTTPConfig{
					Protocol: "https",
					Hostname: "test.example.com",
				},
				Studio: platformv1alpha1.HTTPConfig{
					Protocol: "https",
					Hostname: "studio.example.com",
				},
			},
			Gateway: platformv1alpha1.GatewaySpec{
				API: platformv1alpha1.ExistingGatewayRef{
					Name:      "gw",
					Namespace: "envoy-gateway-system",
				},
				Studio: platformv1alpha1.ExistingGatewayRef{
					Name:      "gw",
					Namespace: "envoy-gateway-system",
				},
			},
			Database:  platformv1alpha1.DatabaseSpec{Host: "postgres.test.svc", PasswordRef: platformv1alpha1.SecretKeyRef{Name: "test-db-secret", Key: "password"}},
			Studio:    &platformv1alpha1.StudioSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/studio:test"}},
			Auth:      &platformv1alpha1.AuthSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/gotrue:test"}},
			Rest:      &platformv1alpha1.RestSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "postgrest/postgrest:test"}},
			Realtime:  &platformv1alpha1.RealtimeSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/realtime:test"}},
			Storage:   &platformv1alpha1.StorageSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/storage-api:test"}},
			Meta:      &platformv1alpha1.MetaSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/postgres-meta:test"}},
			Functions: &platformv1alpha1.FunctionsSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/edge-runtime:test"}},
		},
	}
}

func minimalProject(name string) *platformv1alpha1.Project {
	return &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: platformv1alpha1.ProjectSpec{
			Version: "2026.04.27",
			Global:  platformv1alpha1.GlobalSpec{SiteURL: "https://test.example.com"},
			HTTP: platformv1alpha1.HTTPSpec{
				API: platformv1alpha1.HTTPConfig{
					Protocol: "http",
					Hostname: "test.example.com",
				},
				Studio: platformv1alpha1.HTTPConfig{
					Protocol: "http",
					Hostname: "studio.example.com",
				},
			},
			Gateway: platformv1alpha1.GatewaySpec{
				API: platformv1alpha1.ExistingGatewayRef{
					Name:      "gw",
					Namespace: "envoy-gateway-system",
				},
				Studio: platformv1alpha1.ExistingGatewayRef{
					Name:      "gw",
					Namespace: "envoy-gateway-system",
				},
			},
			Database: platformv1alpha1.DatabaseSpec{Host: "postgres.test.svc", PasswordRef: platformv1alpha1.SecretKeyRef{Name: "test-db-secret", Key: "password"}},
		},
	}
}

var _ = Describe("Project Controller", func() {
	const timeout = 30 * time.Second
	const interval = 250 * time.Millisecond

	Context("When creating a Project", func() {
		const projectName = "test-project"
		projectKey := types.NamespacedName{Name: projectName, Namespace: "default"}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, validProject(projectName))).To(Succeed())
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
		})

		It("should create generated secrets", func() {
			for _, suffix := range []string{"jwt", "studio", "keys", "storage-s3-protocol"} {
				secret := &corev1.Secret{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s", projectName, suffix), Namespace: "default"}, secret)).To(Succeed())
			}
		})

		It("should include .htpasswd in the studio secret", func() {
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: projectName + "-studio", Namespace: "default"}, secret)).To(Succeed())
			Expect(secret.Data).To(HaveKey(".htpasswd"))

			username := string(secret.Data["username"])
			password := string(secret.Data["password"])
			hash := sha1.Sum([]byte(password))
			expected := fmt.Sprintf("%s:{SHA}%s", username, base64.StdEncoding.EncodeToString(hash[:]))
			Expect(string(secret.Data[".htpasswd"])).To(Equal(expected))
			Expect(strings.HasPrefix(string(secret.Data[".htpasswd"]), "supabase:{SHA}")).To(BeTrue())
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

		It("should create HTTPRoutes for the configured gateways", func() {
			apiRoute := &gatewayv1.HTTPRoute{}
			apiRouteKey := types.NamespacedName{Name: projectName + "-gateway-api", Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, apiRouteKey, apiRoute)
			}, timeout, interval).Should(Succeed())
			Expect(apiRoute.Spec.ParentRefs).To(HaveLen(1))
			Expect(apiRoute.Spec.ParentRefs[0].Name).To(Equal(gatewayv1.ObjectName("gw")))
			Expect(apiRoute.Spec.ParentRefs[0].Namespace).NotTo(BeNil())
			Expect(*apiRoute.Spec.ParentRefs[0].Namespace).To(Equal(gatewayv1.Namespace("envoy-gateway-system")))

			studioRoute := &gatewayv1.HTTPRoute{}
			studioRouteKey := types.NamespacedName{Name: projectName + "-gateway-studio", Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, studioRouteKey, studioRoute)
			}, timeout, interval).Should(Succeed())
			Expect(studioRoute.Spec.ParentRefs).To(HaveLen(1))
			Expect(studioRoute.Spec.ParentRefs[0].Name).To(Equal(gatewayv1.ObjectName("gw")))
			Expect(studioRoute.Spec.ParentRefs[0].Namespace).NotTo(BeNil())
			Expect(*studioRoute.Spec.ParentRefs[0].Namespace).To(Equal(gatewayv1.Namespace("envoy-gateway-system")))

			sp := &unstructured.Unstructured{}
			sp.SetAPIVersion("gateway.envoyproxy.io/v1alpha1")
			sp.SetKind("SecurityPolicy")
			spKey := types.NamespacedName{Name: projectName + "-studio-basic-auth", Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, spKey, sp)
			}, timeout, interval).Should(Succeed())

			spec, ok := sp.Object["spec"].(map[string]any)
			Expect(ok).To(BeTrue())
			targetRefs, ok := spec["targetRefs"].([]any)
			Expect(ok).To(BeTrue())
			Expect(targetRefs).To(HaveLen(1))
			tr, ok := targetRefs[0].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(tr["name"]).To(Equal(projectName + "-gateway-studio"))

			basicAuth, ok := spec["basicAuth"].(map[string]any)
			Expect(ok).To(BeTrue())
			users, ok := basicAuth["users"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(users["name"]).To(Equal(projectName + "-studio"))
		})

		It("should create a SAML secret when SAML is enabled", func() {
			samlProject := validProject("saml-project")
			samlProject.Spec.Auth.SAML = &platformv1alpha1.AuthSamlSpec{Enabled: boolP(true)}
			Expect(k8sClient.Create(ctx, samlProject)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, samlProject) })

			samlSecretKey := types.NamespacedName{Name: "saml-project-saml", Namespace: "default"}
			Eventually(func(g Gomega) {
				secret := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, samlSecretKey, secret)).To(Succeed())
				g.Expect(secret.Data).To(HaveKey("private-key"))
			}, timeout, interval).Should(Succeed())
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
			Expect(k8sClient.Create(ctx, minimalProject(projectName))).To(Succeed())
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
		})

		It("should create default component workloads", func() {
			for _, name := range []string{"auth", "rest", "realtime", "meta", "functions"} {
				deployment := &appsv1.Deployment{}
				key := types.NamespacedName{Name: fmt.Sprintf("%s-%s", projectName, name), Namespace: "default"}
				Eventually(func() error {
					return k8sClient.Get(ctx, key, deployment)
				}, timeout, interval).Should(Succeed())
			}

			for _, name := range []string{"studio", "storage"} {
				statefulSet := &appsv1.StatefulSet{}
				key := types.NamespacedName{Name: fmt.Sprintf("%s-%s", projectName, name), Namespace: "default"}
				Eventually(func() error {
					return k8sClient.Get(ctx, key, statefulSet)
				}, timeout, interval).Should(Succeed())
			}
		})
	})

	Context("When a generated secret is deleted (rotation)", func() {
		const projectName = "rotation-project"
		projectKey := types.NamespacedName{Name: projectName, Namespace: "default"}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, validProject(projectName))).To(Succeed())
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

	Context("When creating a Project with disabled components", func() {
		const projectName = "disabled-components-project"
		projectKey := types.NamespacedName{Name: projectName, Namespace: "default"}

		It("should not create studio HTTPRoute when studio is disabled", func() {
			project := validProject(projectName)
			f := false
			project.Spec.Studio.Enabled = &f
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			Eventually(func(g Gomega) {
				created := &platformv1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			studioRoute := &gatewayv1.HTTPRoute{}
			studioRouteKey := types.NamespacedName{Name: projectName + "-gateway-studio", Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, studioRouteKey, studioRoute)
			}, timeout, interval).ShouldNot(Succeed())

			apiRoute := &gatewayv1.HTTPRoute{}
			apiRouteKey := types.NamespacedName{Name: projectName + "-gateway-api", Namespace: "default"}
			Expect(k8sClient.Get(ctx, apiRouteKey, apiRoute)).To(Succeed())

			sp := &unstructured.Unstructured{}
			sp.SetAPIVersion("gateway.envoyproxy.io/v1alpha1")
			sp.SetKind("SecurityPolicy")
			spKey := types.NamespacedName{Name: projectName + "-studio-basic-auth", Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, spKey, sp)
			}, timeout, interval).ShouldNot(Succeed())
		})

		It("should not create api HTTPRoute when all API components are disabled", func() {
			project := minimalProject(projectName)
			f := false
			project.Spec.Auth = &platformv1alpha1.AuthSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}}
			project.Spec.Rest = &platformv1alpha1.RestSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}}
			project.Spec.Realtime = &platformv1alpha1.RealtimeSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}}
			project.Spec.Storage = &platformv1alpha1.StorageSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}}
			project.Spec.Meta = &platformv1alpha1.MetaSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}}
			project.Spec.Functions = &platformv1alpha1.FunctionsSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}}
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			Eventually(func(g Gomega) {
				created := &platformv1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			apiRoute := &gatewayv1.HTTPRoute{}
			apiRouteKey := types.NamespacedName{Name: projectName + "-gateway-api", Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, apiRouteKey, apiRoute)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

	Context("When disabling previously enabled components", func() {
		const projectName = "disable-components-project"
		projectKey := types.NamespacedName{Name: projectName, Namespace: "default"}

		It("should delete component workloads when disabled", func() {
			project := validProject(projectName)
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			// Wait for project to be ready and components created
			Eventually(func(g Gomega) {
				created := &platformv1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			// Verify studio StatefulSet and auth Deployment exist
			studioSts := &appsv1.StatefulSet{}
			studioStsKey := types.NamespacedName{Name: projectName + "-studio", Namespace: "default"}
			Expect(k8sClient.Get(ctx, studioStsKey, studioSts)).To(Succeed())

			authDeploy := &appsv1.Deployment{}
			authDeployKey := types.NamespacedName{Name: projectName + "-auth", Namespace: "default"}
			Expect(k8sClient.Get(ctx, authDeployKey, authDeploy)).To(Succeed())

			// Verify services exist
			studioSvc := &corev1.Service{}
			studioSvcKey := types.NamespacedName{Name: projectName + "-studio", Namespace: "default"}
			Expect(k8sClient.Get(ctx, studioSvcKey, studioSvc)).To(Succeed())

			authSvc := &corev1.Service{}
			authSvcKey := types.NamespacedName{Name: projectName + "-auth", Namespace: "default"}
			Expect(k8sClient.Get(ctx, authSvcKey, authSvc)).To(Succeed())

			// Disable studio and auth
			updated := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, updated)).To(Succeed())
			f := false
			updated.Spec.Studio.Enabled = &f
			updated.Spec.Auth.Enabled = &f
			Expect(k8sClient.Update(ctx, updated)).To(Succeed())

			// Wait for studio StatefulSet to be deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, studioStsKey, studioSts)
			}, timeout, interval).ShouldNot(Succeed())

			// Wait for auth Deployment to be deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, authDeployKey, authDeploy)
			}, timeout, interval).ShouldNot(Succeed())

			// Wait for studio Service to be deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, studioSvcKey, studioSvc)
			}, timeout, interval).ShouldNot(Succeed())

			// Wait for auth Service to be deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, authSvcKey, authSvc)
			}, timeout, interval).ShouldNot(Succeed())
		})

		It("should delete studio secret when studio is disabled", func() {
			project := validProject(projectName)
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			// Wait for project to be ready
			Eventually(func(g Gomega) {
				created := &platformv1alpha1.Project{}
				g.Expect(k8sClient.Get(ctx, projectKey, created)).To(Succeed())
				g.Expect(meta.IsStatusConditionTrue(created.Status.Conditions, ConditionTypeReady)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			// Verify studio secret exists
			studioSecret := &corev1.Secret{}
			studioSecretKey := types.NamespacedName{Name: projectName + "-studio", Namespace: "default"}
			Expect(k8sClient.Get(ctx, studioSecretKey, studioSecret)).To(Succeed())

			// Disable studio
			updated := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, projectKey, updated)).To(Succeed())
			f := false
			updated.Spec.Studio.Enabled = &f
			Expect(k8sClient.Update(ctx, updated)).To(Succeed())

			// Wait for studio secret to be deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, studioSecretKey, studioSecret)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})
})
