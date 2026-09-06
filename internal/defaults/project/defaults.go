package project

import (
	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceContext = defaults.Context

const (
	DefaultBackoffLimit            int32 = 3
	DefaultTTLSecondsAfterFinished int32 = 30
)

func NewContext(project *core.Project) *ResourceContext { return defaults.NewContext(project) }
func ProjectLabels(project metav1.Object) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "project",
		"app.kubernetes.io/component":  "project",
		"app.kubernetes.io/instance":   project.GetName(),
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}
func ProjectSelectorLabels(project metav1.Object) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "project",
		"app.kubernetes.io/component": "project",
		"app.kubernetes.io/instance":  project.GetName(),
	}
}
