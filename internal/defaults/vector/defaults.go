package vector

import "github.com/supabase-community/supabase-kubernetes/internal/defaults"

type ResourceContext = defaults.Context

const DefaultVectorPort int32 = 9001

// VectorName returns the name shared by the Vector workload and its configuration.
func VectorName(project *ResourceContext) string {
	return defaults.ComponentName(project.Names["Vector"], "vector")
}

// VectorLabels returns the stable labels for Vector resources.
func VectorLabels(project *ResourceContext) map[string]string {
	return defaults.SelectorLabels(project, "Vector", "vector", "vector", "vector")
}
