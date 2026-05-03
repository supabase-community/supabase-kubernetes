package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("Project Validation", func() {
	Context("required fields", func() {
		It("should reject CR missing spec.version", func() {
			project := minimalValidProject("test-val-no-version")
			project.Spec.Version = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.global.siteUrl", func() {
			project := minimalValidProject("test-val-no-siteurl")
			project.Spec.Global.SiteURL = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.http.protocol", func() {
			project := minimalValidProject("test-val-no-http-protocol")
			project.Spec.HTTP.Protocol = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.http.hostname", func() {
			project := minimalValidProject("test-val-no-http-hostname")
			project.Spec.HTTP.Hostname = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR with invalid spec.http.protocol", func() {
			project := minimalValidProject("test-val-invalid-http-protocol")
			project.Spec.HTTP.Protocol = "tcp"
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.http.gatewayRef.name", func() {
			project := minimalValidProject("test-val-no-http-gateway-name")
			project.Spec.HTTP.GatewayRef.Name = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.http.gatewayRef.namespace", func() {
			project := minimalValidProject("test-val-no-http-gateway-namespace")
			project.Spec.HTTP.GatewayRef.Namespace = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.database.host", func() {
			project := minimalValidProject("test-val-no-dbhost")
			project.Spec.Database.Host = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.database.passwordRef.name", func() {
			project := minimalValidProject("test-val-no-pwref-name")
			project.Spec.Database.PasswordRef.Name = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should accept a minimal valid spec", func() {
			project := minimalValidProject("test-val-valid")
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })
		})

		It("should accept spec without component image overrides", func() {
			project := minimalValidProject("test-val-no-image-overrides")
			project.Spec.Studio.Image = ""
			project.Spec.Auth.Image = ""
			project.Spec.Rest.Image = ""
			project.Spec.Realtime.Image = ""
			project.Spec.Storage.Image = ""
			project.Spec.Meta.Image = ""
			project.Spec.Functions.Image = ""
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })
		})
	})

	Context("component constraints", func() {
		It("should reject negative replicas on a component", func() {
			project := minimalValidProject("test-val-neg-replicas")
			project.Spec.Studio.Replicas = int32Ptr(-1)
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should allow zero replicas", func() {
			project := minimalValidProject("test-val-zero-replicas")
			project.Spec.Studio.Replicas = int32Ptr(0)
			Expect(k8sClient.Create(ctx, project)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })
		})

		It("should reject rest.dbMaxRows less than 1", func() {
			project := minimalValidProject("test-val-neg-maxrows")
			project.Spec.Rest.DBMaxRows = int32Ptr(0)
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})
	})
})
