package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

var _ = Describe("Project Defaults", func() {
	Context("global defaults", func() {
		It("should default jwtExpirySeconds to 3600", func() {
			project := minimalValidProject("test-def-jwt")
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			fetched := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, fetched)).To(Succeed())

			Expect(fetched.Spec.Global.JWTExpirySeconds).NotTo(BeNil())
			Expect(*fetched.Spec.Global.JWTExpirySeconds).To(Equal(int32(3600)))
		})
	})

	Context("database defaults", func() {
		It("should default port to 5432 and dbName to postgres", func() {
			project := minimalValidProject("test-def-db")
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			fetched := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, fetched)).To(Succeed())

			Expect(fetched.Spec.Database.Port).NotTo(BeNil())
			Expect(*fetched.Spec.Database.Port).To(Equal(int32(5432)))
			Expect(fetched.Spec.Database.DBName).NotTo(BeNil())
			Expect(*fetched.Spec.Database.DBName).To(Equal("postgres"))
		})
	})

	Context("component defaults", func() {
		It("should default enabled=true and replicas=1 for all components", func() {
			project := minimalValidProject("test-def-components")
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			fetched := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, fetched)).To(Succeed())

			for _, c := range []struct {
				name string
				spec platformv1alpha1.ComponentSpec
			}{
				{"Studio", fetched.Spec.Studio.ComponentSpec},
				{"Auth", fetched.Spec.Auth.ComponentSpec},
				{"Rest", fetched.Spec.Rest.ComponentSpec},
				{"Realtime", fetched.Spec.Realtime.ComponentSpec},
				{"Storage", fetched.Spec.Storage.ComponentSpec},
				{"Meta", fetched.Spec.Meta.ComponentSpec},
				{"Functions", fetched.Spec.Functions.ComponentSpec},
			} {
				Expect(c.spec.Enabled).NotTo(BeNil(), "%s.enabled should not be nil", c.name)
				Expect(*c.spec.Enabled).To(BeTrue(), "%s.enabled should default to true", c.name)
				Expect(c.spec.Replicas).NotTo(BeNil(), "%s.replicas should not be nil", c.name)
				Expect(*c.spec.Replicas).To(Equal(int32(1)), "%s.replicas should default to 1", c.name)
			}
		})

		It("should preserve explicit overrides", func() {
			project := minimalValidProject("test-def-override")
			project.Spec.Studio.Replicas = int32Ptr(3)
			project.Spec.Auth.Enabled = boolPtr(false)
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			fetched := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, fetched)).To(Succeed())

			Expect(*fetched.Spec.Studio.Replicas).To(Equal(int32(3)))
			Expect(*fetched.Spec.Auth.Enabled).To(BeFalse())
		})
	})

	Context("auth defaults", func() {
		It("should default auth boolean fields", func() {
			project := minimalValidProject("test-def-auth-bools")
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			fetched := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, fetched)).To(Succeed())

			Expect(*fetched.Spec.Auth.DisableSignup).To(BeFalse())
			Expect(*fetched.Spec.Auth.EnableAnonymousUsers).To(BeFalse())
			Expect(*fetched.Spec.Auth.ExternalSkipNonceCheck).To(BeFalse())
		})
	})

	Context("rest defaults", func() {
		It("should default rest fields", func() {
			project := minimalValidProject("test-def-rest")
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			fetched := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, fetched)).To(Succeed())

			Expect(fetched.Spec.Rest.DBSchemas).To(Equal([]string{"public", "storage", "graphql_public"}))
			Expect(*fetched.Spec.Rest.DBMaxRows).To(Equal(int32(1000)))
			Expect(*fetched.Spec.Rest.DBExtraSearchPath).To(Equal("public"))
		})
	})

	Context("storage defaults", func() {
		It("should default storage fields", func() {
			project := minimalValidProject("test-def-storage")
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			fetched := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, fetched)).To(Succeed())

			Expect(*fetched.Spec.Storage.Backend).To(Equal("file"))
			Expect(*fetched.Spec.Storage.Bucket).To(Equal("stub"))
			Expect(*fetched.Spec.Storage.TenantID).To(Equal("stub"))
			Expect(*fetched.Spec.Storage.Region).To(Equal("local"))
			Expect(*fetched.Spec.Storage.FileSizeLimit).To(Equal(int32(52428800)))
		})
	})

	Context("functions defaults", func() {
		It("should default verifyJwt to false", func() {
			project := minimalValidProject("test-def-functions")
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			fetched := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, fetched)).To(Succeed())

			Expect(*fetched.Spec.Functions.VerifyJWT).To(BeFalse())
		})
	})

	Context("studio defaults", func() {
		It("should default studio organization and project", func() {
			project := minimalValidProject("test-def-studio")
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

			fetched := &platformv1alpha1.Project{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, fetched)).To(Succeed())

			Expect(*fetched.Spec.Studio.Organization).To(Equal("Default Organization"))
			Expect(*fetched.Spec.Studio.Project).To(Equal("Default Project"))
		})
	})
})
