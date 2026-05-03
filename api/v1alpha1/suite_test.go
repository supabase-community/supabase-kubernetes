package v1alpha1_test

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API v1alpha1 Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = platformv1alpha1.AddToScheme(clientgoscheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: clientgoscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	ns := &corev1.Namespace{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "default"}, ns)
	if client.IgnoreNotFound(err) != nil {
		Expect(err).NotTo(HaveOccurred())
	}
	if err != nil {
		ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	}
})

var _ = AfterSuite(func() {
	cancel()
	Expect(testEnv.Stop()).To(Succeed())
})
