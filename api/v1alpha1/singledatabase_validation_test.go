package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func minimalValidSingleDatabase(name string) *platformv1alpha1.SingleDatabase {
	return &platformv1alpha1.SingleDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: platformv1alpha1.SingleDatabaseSpec{
			Image: "supabase/postgres:17.6.1.084",
			Storage: platformv1alpha1.VolumeClaimTemplateSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
			},
		},
	}
}

var _ = Describe("SingleDatabase Validation", func() {
	Context("required fields", func() {
		It("should accept a minimal valid spec", func() {
			singleDB := minimalValidSingleDatabase("test-val-valid")
			Expect(k8sClient.Create(ctx, singleDB)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, singleDB) })
		})
	})

})
