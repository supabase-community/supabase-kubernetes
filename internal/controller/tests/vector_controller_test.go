package tests

import (
	"context"
	"strings"
	"testing"

	vectorcontroller "github.com/supabase-community/supabase-kubernetes/internal/controller/vector"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	analyticsdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/analytics"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestVectorIsolationLifecycleAndRBAC(t *testing.T) {
	ctx := context.Background()
	p := logsProject()
	db := logsDatabase()
	a := &core.Analytics{ObjectMeta: metav1.ObjectMeta{Name: "logs", Namespace: p.Namespace}, Spec: core.AnalyticsSpec{ProjectRef: corev1.LocalObjectReference{Name: p.Name}}}
	v := &core.Vector{ObjectMeta: metav1.ObjectMeta{Name: "collector", Namespace: p.Namespace, UID: "vector-uid"}, Spec: core.VectorSpec{ProjectRef: corev1.LocalObjectReference{Name: p.Name}}}
	auth := &core.Auth{ObjectMeta: metav1.ObjectMeta{Name: "custom-auth", Namespace: p.Namespace}, Spec: core.AuthSpec{ProjectRef: corev1.LocalObjectReference{Name: p.Name}}}
	other := auth.DeepCopy()
	other.Name = "other-auth"
	other.Spec.ProjectRef.Name = otherProjectName
	envoy := &core.Envoy{ObjectMeta: metav1.ObjectMeta{Name: "gateway", Namespace: p.Namespace}, Spec: core.EnvoySpec{ProjectRef: corev1.LocalObjectReference{Name: p.Name}}}
	state := defaults.NewContext(p)
	state.Names["Analytics"] = a.Name
	keys, err := analyticsdefaults.AnalyticsAuthSecret(state, db.Status.ResolvedDatabase, "password")
	if err != nil {
		t.Fatal(err)
	}
	c := logsClient(t, p, db, a, v, auth, other, envoy, keys)
	r := &vectorcontroller.Reconciler{Client: c, Scheme: c.Scheme()}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(v)}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	cm := &corev1.ConfigMap{}
	key := client.ObjectKey{Namespace: p.Namespace, Name: "collector-vector"}
	if err := c.Get(ctx, key, cm); err != nil {
		t.Fatal(err)
	}
	cfg := cm.Data["vector.yml"]
	if !strings.Contains(cfg, "custom-auth-auth") || strings.Contains(cfg, "other-auth") || !strings.Contains(cfg, "app.kubernetes.io/component=gateway") || !strings.Contains(cfg, "metadata.namespace=default") {
		t.Fatal("incorrect workload selection")
	}
	ds := &appsv1.DaemonSet{}
	if err := c.Get(ctx, key, ds); err != nil {
		t.Fatal(err)
	}
	assertSecretEnv(t, ds.Spec.Template.Spec.Containers[0], "LOGFLARE_PUBLIC_ACCESS_TOKEN", "logs-analytics-auth", "public-access-token")
	version := ds.ResourceVersion
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, key, ds); err != nil {
		t.Fatal(err)
	}
	if ds.ResourceVersion != version {
		t.Fatal("unchanged Vector rolled out")
	}
	beforeTokenChange := ds.Spec.Template.Annotations["core.supabase.io/inputs-hash"]
	if err := c.Get(ctx, client.ObjectKeyFromObject(keys), keys); err != nil {
		t.Fatal(err)
	}
	keys.Data[analyticsdefaults.AuthSecretPublicTokenKey] = []byte("updated-public-token")
	if err := c.Update(ctx, keys); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, key, ds); err != nil {
		t.Fatal(err)
	}
	if ds.Spec.Template.Annotations["core.supabase.io/inputs-hash"] == beforeTokenChange {
		t.Fatal("Analytics token change did not update Vector")
	}
	binding := &rbacv1.ClusterRoleBinding{}
	if err := c.Get(ctx, client.ObjectKey{Name: "supabase-vector-" + string(v.UID)}, binding); err != nil {
		t.Fatal(err)
	}
	if binding.Subjects[0].Namespace != p.Namespace || binding.Subjects[0].Name != "collector-vector" {
		t.Fatal("incorrect binding")
	}
	if err := c.Delete(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, key, ds); err == nil {
		t.Fatal("collector continued after Analytics deletion")
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(v), v); err != nil {
		t.Fatal(err)
	}
	if err := c.Delete(ctx, v); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, client.ObjectKey{Name: "supabase-vector-" + string(v.UID)}, binding); err == nil {
		t.Fatal("binding leaked")
	}
	role := &rbacv1.ClusterRole{}
	if err := c.Get(ctx, client.ObjectKey{Name: "supabase-vector-" + string(v.UID)}, role); err == nil {
		t.Fatal("role leaked")
	}
}
