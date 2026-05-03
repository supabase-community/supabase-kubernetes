package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func validFunction(name string) *platformv1alpha1.Function {
	return &platformv1alpha1.Function{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: platformv1alpha1.FunctionSpec{
			ProjectRef:   platformv1alpha1.FunctionProjectRef{Name: "sample-project"},
			FunctionName: "main",
			Source: map[string]string{
				"index.ts": "Deno.serve(() => new Response('ok'))",
			},
		},
	}
}

var _ = Describe("Function Validation", func() {
	It("should reject missing projectRef.name", func() {
		fn := validFunction("sf-no-project")
		fn.Spec.ProjectRef.Name = ""

		err := k8sClient.Create(ctx, fn)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
	})

	It("should reject empty functionName", func() {
		fn := validFunction("sf-no-name")
		fn.Spec.FunctionName = ""

		err := k8sClient.Create(ctx, fn)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
	})

	It("should reject invalid functionName pattern", func() {
		fn := validFunction("sf-bad-name")
		fn.Spec.FunctionName = "Main_Function"

		err := k8sClient.Create(ctx, fn)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
	})

	It("should accept a valid resource", func() {
		fn := validFunction("sf-valid")
		Expect(k8sClient.Create(ctx, fn)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, fn) })
	})
})
