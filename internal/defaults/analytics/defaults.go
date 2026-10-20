package analytics

import "github.com/supabase-community/supabase-kubernetes/internal/defaults"

type ResourceContext = defaults.Context

const DefaultAnalyticsPort int32 = 4000

var ComponentName = defaults.ComponentName

func AnalyticsLabels(c *ResourceContext) map[string]string {
	return defaults.ComponentLabels(c, "Analytics", "analytics", "analytics", "analytics")
}
func AnalyticsSelectorLabels(c *ResourceContext) map[string]string {
	return defaults.SelectorLabels(c, "Analytics", "analytics", "analytics", "analytics")
}
