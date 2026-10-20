package vector

import (
	"context"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	singledatabase "github.com/supabase-community/supabase-kubernetes/internal/defaults/singledatabase"
	vectordefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/vector"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) targets(ctx context.Context, p *core.Project) ([]vectordefaults.Target, error) {
	definitions := []struct {
		list                                client.ObjectList
		kind, suffix, container, app, label string
	}{
		{&core.AuthList{}, "Auth", "auth", "auth", "supabase-auth", "auth"},
		{&core.RestList{}, "Rest", "rest", "rest", "supabase-rest", "rest"},
		{&core.RealtimeList{}, "Realtime", "realtime", "realtime", "realtime-dev.supabase-realtime", "realtime"},
		{&core.StorageList{}, "Storage", "storage", "storage", "supabase-storage", "storage"},
		{&core.EdgeRuntimeList{}, "EdgeRuntime", "edge-runtime", "edge-runtime", "supabase-edge-functions", "functions"},
		{&core.EnvoyList{}, "Envoy", "envoy", "envoy", "supabase-envoy", "envoy"},
	}
	targets := make([]vectordefaults.Target, 0, len(definitions)+1)
	for _, d := range definitions {
		obj, err := component.ListOptional(ctx, r.Client, p.Namespace, p.Name, d.list, d.kind)
		if err != nil {
			return nil, err
		}
		if obj == nil || !obj.GetDeletionTimestamp().IsZero() {
			continue
		}
		state := defaults.NewContext(p)
		state.Names[d.kind] = obj.GetName()
		labels := defaults.SelectorLabels(state, d.kind, d.suffix, d.label, d.label)
		if d.kind == "Envoy" {
			labels["app.kubernetes.io/component"] = "gateway"
		}
		labels["app.kubernetes.io/managed-by"] = "supabase-operator"
		targets = append(targets, vectordefaults.Target{Name: d.suffix, Container: d.container, AppName: d.app, Labels: labels})
	}
	db := &core.SingleDatabase{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: p.Spec.DatabaseRef.Name}, db); err != nil {
		return nil, err
	}
	targets = append(targets, vectordefaults.Target{Name: "db", Container: "postgres", AppName: "supabase-db", Labels: singledatabase.PostgresLabels(db)})
	return targets, nil
}
