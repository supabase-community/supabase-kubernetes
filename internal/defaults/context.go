package defaults

import (
	"github.com/supabase-community/supabase-kubernetes/internal/helper"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Context struct {
	*core.Project
	Spec  Configuration
	Names map[string]string
	Owner client.Object
}
type Configuration struct {
	core.ProjectSpec
	Auth        *core.AuthSpec
	Rest        *core.RestSpec
	Meta        *core.MetaSpec
	Realtime    *core.RealtimeSpec
	Storage     *core.StorageSpec
	Studio      *core.StudioSpec
	Envoy       *core.EnvoySpec
	EdgeRuntime *core.EdgeRuntimeSpec
}

func NewContext(p *core.Project) *Context {
	return &Context{Project: p, Spec: Configuration{ProjectSpec: p.Spec}, Names: map[string]string{}}
}

// ComponentName derives child names from the discovered component CR.
func ComponentName(name, suffix string) string { return helper.ResourceName(name, suffix) }

// ComponentLabels returns the common labels for a component's resources.
func ComponentLabels(ctx *Context, key, suffix, name, component string) map[string]string {
	labels := SelectorLabels(ctx, key, suffix, name, component)
	labels["app.kubernetes.io/managed-by"] = "supabase-operator"
	return labels
}

// SelectorLabels returns the stable selector labels for a component.
func SelectorLabels(ctx *Context, key, suffix, name, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      name,
		"app.kubernetes.io/component": component,
		"app.kubernetes.io/instance":  ComponentName(ctx.Names[key], suffix),
	}
}
