package edgeruntime

import (
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RuntimeConfigPath contains the router, shared import map, and user functions.
const RuntimeConfigPath = "/home/deno/functions"

func RuntimeConfigMapName(project *ResourceContext) string {
	return ComponentName(project.Names["EdgeRuntime"], "config")
}

// RuntimeConfigMap supplies the Docker router and its shared import map.
func RuntimeConfigMap(project *ResourceContext) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RuntimeConfigMapName(project),
			Namespace: project.Namespace,
			Labels:    FunctionsLabels(project),
		},
		Data: map[string]string{
			"index.ts":   assets.MainFunction,
			"deno.jsonc": assets.FunctionsDenoConfig,
		},
	}
}
