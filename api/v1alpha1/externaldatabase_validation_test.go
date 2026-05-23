package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func minimalValidExternalDatabase(name string) *platformv1alpha1.ExternalDatabase {
	return &platformv1alpha1.ExternalDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: platformv1alpha1.ExternalDatabaseSpec{
			Host: "postgres.db.svc",
			PasswordRef: platformv1alpha1.SecretKeyRef{
				Name: "db-secret",
				Key:  "password",
			},
		},
	}
}

var _ = Describe("ExternalDatabase Validation", func() {
	Context("required fields", func() {
		It("should reject CR missing spec.host", func() {
			extDB := minimalValidExternalDatabase("test-val-no-host")
			extDB.Spec.Host = ""
			err := k8sClient.Create(ctx, extDB)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.passwordRef.name", func() {
			extDB := minimalValidExternalDatabase("test-val-no-pwref-name")
			extDB.Spec.PasswordRef.Name = ""
			err := k8sClient.Create(ctx, extDB)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject CR missing spec.passwordRef.key", func() {
			extDB := minimalValidExternalDatabase("test-val-no-pwref-key")
			extDB.Spec.PasswordRef.Key = ""
			err := k8sClient.Create(ctx, extDB)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err) || apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should accept a minimal valid spec", func() {
			extDB := minimalValidExternalDatabase("test-val-valid")
			Expect(k8sClient.Create(ctx, extDB)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, extDB) })
		})
	})

	Context("defaults", func() {
		It("should default port to 5432 and dbName to postgres", func() {
			extDB := minimalValidExternalDatabase("test-def-db")
			extDB.Spec.Port = nil
			extDB.Spec.DBName = nil
			Expect(k8sClient.Create(ctx, extDB)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, extDB) })

			fetched := &platformv1alpha1.ExternalDatabase{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: extDB.Name, Namespace: extDB.Namespace}, fetched)).To(Succeed())

			Expect(fetched.Spec.Port).NotTo(BeNil())
			Expect(*fetched.Spec.Port).To(Equal(int32(5432)))
			Expect(fetched.Spec.DBName).NotTo(BeNil())
			Expect(*fetched.Spec.DBName).To(Equal("postgres"))
		})
	})
})
