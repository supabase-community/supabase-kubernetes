package tests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Logs APIs", func() {
	It("requires a project reference and makes it immutable", func() {
		analytics := &core.Analytics{ObjectMeta: metav1.ObjectMeta{Name: "logs-validation", Namespace: "default"}}
		Expect(k8sClient.Create(ctx, analytics)).NotTo(Succeed())
		analytics.Spec.ProjectRef = corev1.LocalObjectReference{Name: "project"}
		Expect(k8sClient.Create(ctx, analytics)).To(Succeed())
		Expect(*analytics.Spec.Replicas).To(Equal(int32(1)))
		analytics.Spec.ProjectRef.Name = otherProjectName
		Expect(k8sClient.Update(ctx, analytics)).NotTo(Succeed())
		Expect(k8sClient.Delete(ctx, analytics)).To(Succeed())
		vector := &core.Vector{ObjectMeta: metav1.ObjectMeta{Name: "logs-validation", Namespace: "default"}, Spec: core.VectorSpec{ProjectRef: corev1.LocalObjectReference{Name: "project"}}}
		Expect(k8sClient.Create(ctx, vector)).To(Succeed())
		vector.Spec.ProjectRef.Name = otherProjectName
		Expect(k8sClient.Update(ctx, vector)).NotTo(Succeed())
		Expect(k8sClient.Delete(ctx, vector)).To(Succeed())
	})
})
