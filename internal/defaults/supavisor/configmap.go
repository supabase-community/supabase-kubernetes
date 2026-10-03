package supavisor

import (
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SupavisorConfigMapName(ctx *ResourceContext) string {
	return ComponentName(ctx.Names["Supavisor"], "supavisor-config")
}

func SupavisorConfigMap(ctx *ResourceContext) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: SupavisorConfigMapName(ctx), Namespace: ctx.Namespace, Labels: SupavisorLabels(ctx)},
		Data:       map[string]string{"pooler.exs": assets.SupavisorPoolerScript},
	}
}
