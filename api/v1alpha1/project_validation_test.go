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

		It("should reject CR missing spec.http.api.protocol", func() {
			project := minimalValidProject("test-val-no-http-api-protocol")
			project.Spec.HTTP.API.Protocol = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.http.api.hostname", func() {
			project := minimalValidProject("test-val-no-http-api-hostname")
			project.Spec.HTTP.API.Hostname = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR with invalid spec.http.api.protocol", func() {
			project := minimalValidProject("test-val-invalid-http-api-protocol")
			project.Spec.HTTP.API.Protocol = "tcp"
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.http.studio.protocol", func() {
			project := minimalValidProject("test-val-no-http-studio-protocol")
			project.Spec.HTTP.Studio.Protocol = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.http.studio.hostname", func() {
			project := minimalValidProject("test-val-no-http-studio-hostname")
			project.Spec.HTTP.Studio.Hostname = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.databaseRef.kind", func() {
			project := minimalValidProject("test-val-no-dbref-kind")
			project.Spec.DatabaseRef.Kind = ""
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR with invalid spec.databaseRef.kind", func() {
			project := minimalValidProject("test-val-invalid-dbref-kind")
			project.Spec.DatabaseRef.Kind = "UnknownDatabase"
			err := k8sClient.Create(ctx, project)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.databaseRef.name", func() {
			project := minimalValidProject("test-val-no-dbref-name")
			project.Spec.DatabaseRef.Name = ""
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
