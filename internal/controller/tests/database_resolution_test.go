package tests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/database"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

var _ = Describe("Database reference resolution", func() {
	DescribeTable("does not resolve unavailable or outdated external configuration", func(state string, wantReady bool) {
		db := &core.ExternalDatabase{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default", Generation: 2}}
		db.Status.ResolvedDatabase = &core.ResolvedDatabase{Host: "external.example.com", Port: 5432, DBName: "postgres", User: "postgres", PasswordRef: core.SecretKeyRef{Name: "credentials", Key: "password"}}
		reconciler.SetReady(db, "ReconcileSucceeded", "Resolved")
		ref := core.DatabaseRef{Kind: "ExternalDatabase", Name: db.Name}
		switch state {
		case "missing":
			ref.Name = "absent"
		case "namespace":
			db.Namespace = "elsewhere"
		case "not ready":
			reconciler.SetNotReady(db, "SecretNotFound", "Missing Secret")
		case "no condition":
			db.Status.Conditions = nil
		case "no parameters":
			db.Status.ResolvedDatabase = nil
		case "stale":
			db.Status.Conditions[0].ObservedGeneration = 1
		case "deleting":
			now := metav1.Now()
			db.DeletionTimestamp = &now
			db.Finalizers = []string{"test.example.com/hold"}
		}
		c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(db).Build()
		resolved, ready, err := database.ResolveRef(ctx, c, ref, "default")
		Expect(err).NotTo(HaveOccurred())
		Expect(ready).To(Equal(wantReady))
		if wantReady {
			Expect(resolved).To(Equal(db.Status.ResolvedDatabase))
		} else {
			Expect(resolved).To(BeNil())
		}
	},
		Entry("ready", "ready", true),
		Entry("missing resource", "missing", false),
		Entry("another namespace", "namespace", false),
		Entry("not ready", "not ready", false),
		Entry("missing condition", "no condition", false),
		Entry("missing parameters", "no parameters", false),
		Entry("old generation", "stale", false),
		Entry("deleting resource", "deleting", false),
	)
	It("preserves SingleDatabase resolution and rejects unknown kinds", func() {
		db := &core.SingleDatabase{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"}}
		db.Status.ResolvedDatabase = &core.ResolvedDatabase{Host: "managed-postgres", Port: 5432}
		reconciler.SetReady(db, "ReconcileSucceeded", "Ready")
		c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(db).Build()
		resolved, ready, err := database.ResolveRef(ctx, c, core.DatabaseRef{Kind: "SingleDatabase", Name: db.Name}, db.Namespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(ready).To(BeTrue())
		Expect(resolved).To(Equal(db.Status.ResolvedDatabase))
		_, _, err = database.ResolveRef(ctx, c, core.DatabaseRef{Kind: "Unknown", Name: db.Name}, db.Namespace)
		Expect(err).To(MatchError(ContainSubstring("unsupported database kind")))
	})
})
