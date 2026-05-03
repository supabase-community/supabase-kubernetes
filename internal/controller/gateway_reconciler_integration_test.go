package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

var _ = Describe("Gateway Reconciler", func() {
	const timeout = 20 * time.Second
	const interval = 250 * time.Millisecond

	It("should create a Gateway resource", func() {
		project := validProject("test-gw-create")
		Expect(k8sClient.Create(ctx, project)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

		gwKey := types.NamespacedName{Name: "test-gw-create-gateway", Namespace: "default"}
		Eventually(func(g Gomega) {
			gw := &gatewayv1.Gateway{}
			g.Expect(k8sClient.Get(ctx, gwKey, gw)).To(Succeed())
			g.Expect(gw.Spec.GatewayClassName).To(Equal(gatewayv1.ObjectName("test-class")))
			g.Expect(gw.Spec.Listeners).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())
	})

	It("should update the existing Gateway resource", func() {
		project := validProject("test-gw-update")
		Expect(k8sClient.Create(ctx, project)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

		gwKey := types.NamespacedName{Name: "test-gw-update-gateway", Namespace: "default"}
		Eventually(func() error {
			return k8sClient.Get(ctx, gwKey, &gatewayv1.Gateway{})
		}, timeout, interval).Should(Succeed())

		fetched := &platformv1alpha1.Project{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, fetched)).To(Succeed())
		fetched.Spec.Gateway.GatewayClassName = "istio"
		fetched.Spec.Gateway.Listeners = []platformv1alpha1.GatewayListenerSpec{{Name: "https", Protocol: "HTTPS", Port: 443}}
		fetched.Spec.Global = platformv1alpha1.GlobalSpec{SiteURL: "https://test.example.com"}
		Expect(k8sClient.Update(ctx, fetched)).To(Succeed())

		Eventually(func(g Gomega) {
			gw := &gatewayv1.Gateway{}
			g.Expect(k8sClient.Get(ctx, gwKey, gw)).To(Succeed())
			g.Expect(gw.Spec.GatewayClassName).To(Equal(gatewayv1.ObjectName("istio")))
			g.Expect(gw.Spec.Listeners).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())
	})
})
