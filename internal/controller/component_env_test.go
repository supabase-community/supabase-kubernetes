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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func findEnv(envs []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range envs {
		if envs[i].Name == name {
			return &envs[i]
		}
	}
	return nil
}

func int32P(i int32) *int32 { return &i }
func strP(s string) *string { return &s }
func boolP(b bool) *bool    { return &b }

func newTestEnvProject() *platformv1alpha1.Project {
	return &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec: platformv1alpha1.ProjectSpec{
			Global: platformv1alpha1.GlobalSpec{SiteURL: "https://app.example.com", JWTExpirySeconds: int32P(3600)},
			Gateway: platformv1alpha1.GatewaySpec{
				GatewayClassName: "envoy",
				Host:             "api.example.com",
				Listeners:        []platformv1alpha1.GatewayListenerSpec{{Name: "http", Protocol: "HTTP", Port: 80}},
			},
			Database: platformv1alpha1.DatabaseSpec{
				Host:        "db.example.com",
				Port:        int32P(5432),
				DBName:      strP("postgres"),
				PasswordRef: platformv1alpha1.SecretKeyRef{Name: "db-secret", Key: "password"},
			},
			Studio:    &platformv1alpha1.StudioSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/studio:latest"}},
			Auth:      &platformv1alpha1.AuthSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/gotrue:latest"}},
			Rest:      &platformv1alpha1.RestSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "postgrest/postgrest:latest"}},
			Realtime:  &platformv1alpha1.RealtimeSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/realtime:latest"}},
			Storage:   &platformv1alpha1.StorageSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/storage-api:latest"}},
			Meta:      &platformv1alpha1.MetaSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/postgres-meta:latest"}},
			Functions: &platformv1alpha1.FunctionsSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/edge-runtime:latest"}},
		},
	}
}

var _ = Describe("Component env builders", func() {
	Describe("StudioEnvVars", func() {
		It("should include base env vars and key secret refs", func() {
			envs := StudioEnvVars(newTestEnvProject())
			Expect(findEnv(envs, "HOSTNAME").Value).To(Equal("0.0.0.0"))
			Expect(findEnv(envs, "PG_META_CRYPTO_KEY").ValueFrom.SecretKeyRef.Name).To(Equal("main-keys"))
			Expect(findEnv(envs, "AUTH_JWT_SECRET").ValueFrom.SecretKeyRef.Name).To(Equal("main-jwt"))
		})
	})

	Describe("AuthEnvVars", func() {
		It("should include core GOTRUE env vars", func() {
			envs := AuthEnvVars(newTestEnvProject())
			Expect(findEnv(envs, "GOTRUE_API_HOST").Value).To(Equal("0.0.0.0"))
			Expect(findEnv(envs, "GOTRUE_DB_DATABASE_URL").Value).To(ContainSubstring("$(DB_PASSWORD)"))
			Expect(findEnv(envs, "GOTRUE_JWT_SECRET").ValueFrom.SecretKeyRef.Name).To(Equal("main-jwt"))
		})

		It("should include SMTP vars when configured", func() {
			project := newTestEnvProject()
			project.Spec.Auth.Email = &platformv1alpha1.AuthEmailSpec{SMTP: &platformv1alpha1.AuthSmtpSpec{AdminEmail: "admin@example.com", Host: "smtp.example.com", Port: 587, UserRef: platformv1alpha1.SecretKeyRef{Name: "smtp", Key: "user"}, PassRef: platformv1alpha1.SecretKeyRef{Name: "smtp", Key: "pass"}}}
			envs := AuthEnvVars(project)
			Expect(findEnv(envs, "GOTRUE_SMTP_HOST").Value).To(Equal("smtp.example.com"))
		})
	})

	Describe("Remaining env builders", func() {
		It("should include expected secret refs", func() {
			project := newTestEnvProject()
			Expect(findEnv(RestEnvVars(project), "PGRST_JWT_SECRET").ValueFrom.SecretKeyRef.Key).To(Equal("jwt-jwks"))
			Expect(findEnv(RealtimeEnvVars(project), "SECRET_KEY_BASE").ValueFrom.SecretKeyRef.Name).To(Equal("main-keys"))
			Expect(findEnv(StorageEnvVars(project), "S3_PROTOCOL_ACCESS_KEY_ID").ValueFrom.SecretKeyRef.Name).To(Equal("main-storage-s3-protocol"))
			Expect(findEnv(StorageEnvVars(project), "DATABASE_URL").Value).To(Equal("postgres://supabase_storage_admin:$(POSTGRES_PASSWORD)@db.example.com:5432/postgres"))
			Expect(findEnv(StorageEnvVars(project), "POSTGRES_HOST").Value).To(Equal("db.example.com"))
			Expect(findEnv(MetaEnvVars(project), "CRYPTO_KEY").ValueFrom.SecretKeyRef.Name).To(Equal("main-keys"))
			Expect(findEnv(FunctionsEnvVars(project), "SUPABASE_PUBLISHABLE_KEYS").ValueFrom.SecretKeyRef.Name).To(Equal("main-jwt"))
		})

		It("should trim .svc.cluster.local for node-based components", func() {
			project := newTestEnvProject()
			project.Spec.Database.Host = "postgres.db.svc.cluster.local"
			Expect(findEnv(StorageEnvVars(project), "POSTGRES_HOST").Value).To(Equal("postgres.db"))
			Expect(findEnv(FunctionsEnvVars(project), "SUPABASE_DB_URL").Value).To(ContainSubstring("@postgres.db:5432/"))
		})

		It("should honor functions verifyJwt", func() {
			project := newTestEnvProject()
			project.Spec.Functions.VerifyJWT = boolP(true)
			Expect(findEnv(FunctionsEnvVars(project), "VERIFY_JWT").Value).To(Equal("true"))
		})
	})
})
