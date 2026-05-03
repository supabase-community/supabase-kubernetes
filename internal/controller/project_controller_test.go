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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func validProject(name string) *platformv1alpha1.Project {
	return &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: platformv1alpha1.ProjectSpec{
			Global: platformv1alpha1.GlobalSpec{SiteURL: "https://test.example.com"},
			HTTP: platformv1alpha1.HTTPSpec{
				Protocol: "https",
				Hostname: "test.example.com",
				GatewayRef: platformv1alpha1.ExistingGatewayRef{
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
			for _, suffix := range []string{"jwt", "dashboard", "keys", "storage-s3-protocol"} {
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

		It("should create HTTPRoute for the configured gatewayRef", func() {
			route := &gatewayv1.HTTPRoute{}
			routeKey := types.NamespacedName{Name: projectName + "-gateway", Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, routeKey, route)
			}, timeout, interval).Should(Succeed())
			Expect(route.Spec.ParentRefs).To(HaveLen(1))
			Expect(route.Spec.ParentRefs[0].Name).To(Equal(gatewayv1.ObjectName("gw")))
			Expect(route.Spec.ParentRefs[0].Namespace).NotTo(BeNil())
			Expect(*route.Spec.ParentRefs[0].Namespace).To(Equal(gatewayv1.Namespace("envoy-gateway-system")))
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
})
