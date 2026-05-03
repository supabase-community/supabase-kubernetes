package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

var _ = Describe("Function Controller", func() {
	const timeout = 30 * time.Second
	const interval = 250 * time.Millisecond

	It("should sync source files to an internal ConfigMap", func() {
		project := validProject("functions-project")
		Expect(k8sClient.Create(ctx, project)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

		function := &platformv1alpha1.Function{
			ObjectMeta: metav1.ObjectMeta{Name: "hello-fn", Namespace: "default"},
			Spec: platformv1alpha1.FunctionSpec{
				ProjectRef:   platformv1alpha1.FunctionProjectRef{Name: project.Name},
				FunctionName: "hello",
				Source: map[string]string{
					"index.ts":  "Deno.serve(() => new Response('ok'))",
					"helper.js": "export const x = 1",
				},
			},
		}
		Expect(k8sClient.Create(ctx, function)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, function) })

		cm := &corev1.ConfigMap{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "hello-fn-code", Namespace: "default"}, cm)).To(Succeed())
			g.Expect(cm.Data).To(HaveKey("index.ts"))
			g.Expect(cm.Data).To(HaveKey("helper.js"))
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			current := &platformv1alpha1.Function{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: function.Name, Namespace: function.Namespace}, current)).To(Succeed())
			g.Expect(meta.IsStatusConditionTrue(current.Status.Conditions, conditionTypeReady)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
	})

	It("should reject source without index.ts", func() {
		project := validProject("functions-invalid")
		Expect(k8sClient.Create(ctx, project)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

		function := &platformv1alpha1.Function{
			ObjectMeta: metav1.ObjectMeta{Name: "invalid-fn", Namespace: "default"},
			Spec: platformv1alpha1.FunctionSpec{
				ProjectRef:   platformv1alpha1.FunctionProjectRef{Name: project.Name},
				FunctionName: "invalid",
				Source: map[string]string{
					"helper.js": "export const x = 1",
				},
			},
		}
		Expect(k8sClient.Create(ctx, function)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, function) })

		Eventually(func(g Gomega) {
			current := &platformv1alpha1.Function{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: function.Name, Namespace: function.Namespace}, current)).To(Succeed())
			ready := meta.FindStatusCondition(current.Status.Conditions, conditionTypeReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal("InvalidSource"))
		}, timeout, interval).Should(Succeed())
	})

	It("should reject duplicate function names in the same project", func() {
		project := validProject("functions-duplicate")
		Expect(k8sClient.Create(ctx, project)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

		first := &platformv1alpha1.Function{
			ObjectMeta: metav1.ObjectMeta{Name: "dup-a", Namespace: "default"},
			Spec: platformv1alpha1.FunctionSpec{
				ProjectRef:   platformv1alpha1.FunctionProjectRef{Name: project.Name},
				FunctionName: "dup-fn",
				Source: map[string]string{
					"index.ts": "Deno.serve(() => new Response('a'))",
				},
			},
		}
		Expect(k8sClient.Create(ctx, first)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, first) })

		second := &platformv1alpha1.Function{
			ObjectMeta: metav1.ObjectMeta{Name: "dup-b", Namespace: "default"},
			Spec: platformv1alpha1.FunctionSpec{
				ProjectRef:   platformv1alpha1.FunctionProjectRef{Name: project.Name},
				FunctionName: "dup-fn",
				Source: map[string]string{
					"index.ts": "Deno.serve(() => new Response('b'))",
				},
			},
		}
		Expect(k8sClient.Create(ctx, second)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, second) })

		Eventually(func(g Gomega) {
			current := &platformv1alpha1.Function{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: second.Name, Namespace: second.Namespace}, current)).To(Succeed())
			ready := meta.FindStatusCondition(current.Status.Conditions, conditionTypeReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal("DuplicateFunctionName"))
		}, timeout, interval).Should(Succeed())
	})

	It("should reject reserved function name main", func() {
		project := validProject("functions-reserved")
		Expect(k8sClient.Create(ctx, project)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

		function := &platformv1alpha1.Function{
			ObjectMeta: metav1.ObjectMeta{Name: "reserved-main", Namespace: "default"},
			Spec: platformv1alpha1.FunctionSpec{
				ProjectRef:   platformv1alpha1.FunctionProjectRef{Name: project.Name},
				FunctionName: "main",
				Source: map[string]string{
					"index.ts": "Deno.serve(() => new Response('main'))",
				},
			},
		}
		Expect(k8sClient.Create(ctx, function)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, function) })

		Eventually(func(g Gomega) {
			current := &platformv1alpha1.Function{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: function.Name, Namespace: function.Namespace}, current)).To(Succeed())
			ready := meta.FindStatusCondition(current.Status.Conditions, conditionTypeReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal("ReservedFunctionName"))
		}, timeout, interval).Should(Succeed())
	})
})
