package tests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Supavisor API", func() {
	It("requires a local immutable Project reference", func() {
		ns := "schema-" + rand.String(6)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())
		c := &core.Supavisor{ObjectMeta: metav1.ObjectMeta{Name: "arbitrary-name", Namespace: ns}}
		Expect(apierrors.IsInvalid(k8sClient.Create(ctx, c))).To(BeTrue())
		c.Spec.ProjectRef = corev1.LocalObjectReference{Name: "missing"}
		Expect(k8sClient.Create(ctx, c)).To(Succeed())

		Expect(apierrors.IsInvalid(k8sClient.Patch(ctx, c, client.RawPatch(types.MergePatchType, []byte(`{"spec":{"projectRef":{"name":"another-project"}}}`))))).To(BeTrue())
	})
})
