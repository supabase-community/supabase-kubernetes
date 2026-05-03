package controller

const (
	componentStudio    = "studio"
	componentAuth      = "auth"
	componentRest      = "rest"
	componentRealtime  = "realtime"
	componentStorage   = "storage"
	componentMeta      = "meta"
	componentFunctions = "functions"
)

var projectVersionCatalog = map[string]map[string]string{
	"2026.04.27": {
		componentStudio:    "supabase/studio:2026.04.27-sha-5f60601",
		componentAuth:      "supabase/gotrue:v2.186.0",
		componentRest:      "postgrest/postgrest:v14.8",
		componentRealtime:  "supabase/realtime:v2.76.5",
		componentStorage:   "supabase/storage-api:v1.48.26",
		componentMeta:      "supabase/postgres-meta:v0.96.3",
		componentFunctions: "supabase/edge-runtime:v1.71.2",
	},
}
