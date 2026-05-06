package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

var _ = Describe("buildStudioBasicAuthSecurityPolicy", func() {
	It("should return nil when studio is disabled", func() {
		f := false
		project := &platformv1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec: platformv1alpha1.ProjectSpec{
				Studio: &platformv1alpha1.StudioSpec{
					ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f},
				},
			},
		}
		Expect(buildStudioBasicAuthSecurityPolicy(project)).To(BeNil())
	})

	It("should return a correctly structured SecurityPolicy when studio is enabled", func() {
		project := &platformv1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "my-project", Namespace: "default"},
			Spec: platformv1alpha1.ProjectSpec{
				Studio: &platformv1alpha1.StudioSpec{},
			},
		}

		sp := buildStudioBasicAuthSecurityPolicy(project)
		Expect(sp).NotTo(BeNil())
		Expect(sp.GetAPIVersion()).To(Equal("gateway.envoyproxy.io/v1alpha1"))
		Expect(sp.GetKind()).To(Equal("SecurityPolicy"))
		Expect(sp.GetName()).To(Equal("my-project-studio-basic-auth"))
		Expect(sp.GetNamespace()).To(Equal("default"))

		spec, ok := sp.Object["spec"].(map[string]any)
		Expect(ok).To(BeTrue())

		targetRefs, ok := spec["targetRefs"].([]map[string]any)
		Expect(ok).To(BeTrue())
		Expect(targetRefs).To(HaveLen(1))
		Expect(targetRefs[0]["group"]).To(Equal("gateway.networking.k8s.io"))
		Expect(targetRefs[0]["kind"]).To(Equal("HTTPRoute"))
		Expect(targetRefs[0]["name"]).To(Equal("my-project-gateway-studio"))

		basicAuth, ok := spec["basicAuth"].(map[string]any)
		Expect(ok).To(BeTrue())
		users, ok := basicAuth["users"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(users["name"]).To(Equal("my-project-studio"))
	})
})
