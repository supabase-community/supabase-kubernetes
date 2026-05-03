package controller

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Component workloads", func() {
	const timeout = 30 * time.Second
	const interval = 250 * time.Millisecond

	It("should create services and workloads for enabled components", func() {
		project := validProject("components-project")
		Expect(k8sClient.Create(ctx, project)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

		for _, component := range []string{"auth", "rest", "realtime", "meta", "functions", "studio", "storage"} {
			service := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s", project.Name, component), Namespace: project.Namespace}, service)
			}, timeout, interval).Should(Succeed())
		}

		authDeployment := &appsv1.Deployment{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: "components-project-auth", Namespace: "default"}, authDeployment)
		}, timeout, interval).Should(Succeed())

		studioStatefulSet := &appsv1.StatefulSet{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: "components-project-studio", Namespace: "default"}, studioStatefulSet)
		}, timeout, interval).Should(Succeed())
	})

	It("should not create deployment for disabled auth component", func() {
		project := validProject("disabled-project")
		f := false
		project.Spec.Auth.Enabled = &f
		Expect(k8sClient.Create(ctx, project)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

		Consistently(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "disabled-project-auth", Namespace: "default"}, &appsv1.Deployment{})
			return err != nil
		}, 5*time.Second, interval).Should(BeTrue())
	})

	It("should mount Function sources into studio and functions pods", func() {
		project := validProject("functions-mounts")
		Expect(k8sClient.Create(ctx, project)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, project) })

		functionsDeployment := &appsv1.Deployment{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: "functions-mounts-functions", Namespace: "default"}, functionsDeployment)
		}, timeout, interval).Should(Succeed())

		studioStatefulSet := &appsv1.StatefulSet{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: "functions-mounts-studio", Namespace: "default"}, studioStatefulSet)
		}, timeout, interval).Should(Succeed())

		functionMounts := functionsDeployment.Spec.Template.Spec.Containers[0].VolumeMounts
		studioMounts := studioStatefulSet.Spec.Template.Spec.Containers[0].VolumeMounts

		Expect(functionMounts).To(ContainElement(HaveField("MountPath", "/home/deno/functions/main/index.ts")))
		Expect(studioMounts).To(ContainElement(HaveField("MountPath", "/var/lib/studio/functions/main/index.ts")))
	})
})
