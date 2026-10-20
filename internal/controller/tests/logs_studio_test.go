package tests

import (
	"testing"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	analyticsdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/analytics"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestStudioAnalyticsLifecycle(t *testing.T) {
	ctx, r, p, objects := componentFixture(t)
	var studio *core.Studio
	for _, obj := range objects {
		if _, ok := obj.(*core.Envoy); ok {
			runComponent(t, ctx, r, obj)
		}
		if s, ok := obj.(*core.Studio); ok {
			studio = s
		}
	}
	key := client.ObjectKey{Namespace: p.Namespace, Name: studio.Name + "-studio"}
	workload := &appsv1.StatefulSet{}
	read := func() {
		t.Helper()
		runComponent(t, ctx, r, studio)
		if err := r.Get(ctx, key, workload); err != nil {
			t.Fatal(err)
		}
	}
	read()
	expectEnvValue(t, workload, "ENABLED_FEATURES_LOGS_ALL", "false")
	a := &core.Analytics{ObjectMeta: metav1.ObjectMeta{Name: "custom-logs", Namespace: p.Namespace}, Spec: core.AnalyticsSpec{ProjectRef: corev1.LocalObjectReference{Name: p.Name}}}
	if err := r.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	state := defaults.NewContext(p)
	state.Names["Analytics"] = a.Name
	secret, err := analyticsdefaults.AnalyticsAuthSecret(state, logsDatabase().Status.ResolvedDatabase, "password")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Create(ctx, secret); err != nil {
		t.Fatal(err)
	}
	read()
	expectEnvValue(t, workload, "ENABLED_FEATURES_LOGS_ALL", "true")
	expectEnvValue(t, workload, "LOGFLARE_URL", "http://custom-logs-analytics.test.svc.cluster.local:4000")
	found := false
	for _, env := range workload.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "LOGFLARE_PRIVATE_ACCESS_TOKEN" {
			found = env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name == "custom-logs-analytics-auth" && env.ValueFrom.SecretKeyRef.Key == "private-access-token"
		}
	}
	if !found {
		t.Fatal("Studio must use the Analytics private token")
	}
	beforeTokenChange := workload.Spec.Template.Annotations["core.supabase.io/inputs-hash"]
	if err := r.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		t.Fatal(err)
	}
	secret.Data[analyticsdefaults.AuthSecretPrivateTokenKey] = []byte("updated-private-token")
	if err := r.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}
	read()
	if workload.Spec.Template.Annotations["core.supabase.io/inputs-hash"] == beforeTokenChange {
		t.Fatal("Analytics token change did not update Studio")
	}
	if err := r.Delete(ctx, a); err != nil {
		t.Fatal(err)
	}
	read()
	expectEnvValue(t, workload, "ENABLED_FEATURES_LOGS_ALL", "false")
	for _, env := range workload.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "LOGFLARE_PRIVATE_ACCESS_TOKEN" || env.Name == "LOGFLARE_URL" {
			t.Fatal("removed Analytics left generated settings in Studio")
		}
	}
	studio.Spec.Config = []corev1.EnvVar{{Name: "ENABLED_FEATURES_LOGS_ALL", Value: "true"}, {Name: "LOGFLARE_URL", Value: "http://external:4000"}}
	if err := r.Update(ctx, studio); err != nil {
		t.Fatal(err)
	}
	read()
	expectEnvValue(t, workload, "LOGFLARE_URL", "http://external:4000")
	expectEnvValue(t, workload, "ENABLED_FEATURES_LOGS_ALL", "true")
}
