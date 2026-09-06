package rest

import (
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	projectdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
)

type ResourceContext = defaults.Context

const (
	DefaultRestPort      int32 = 3000
	DefaultRestAdminPort int32 = 3001
)

func ComponentName(name, suffix string) string { return defaults.ComponentName(name, suffix) }
func RestLabels(ctx *ResourceContext) map[string]string {
	return defaults.ComponentLabels(ctx, "Rest", "rest", "rest", "rest")
}
func RestSelectorLabels(ctx *ResourceContext) map[string]string {
	return defaults.SelectorLabels(ctx, "Rest", "rest", "rest", "rest")
}

var JWTSecretName = projectdefaults.JWTSecretName

const (
	JWTSecretJWKS = projectdefaults.JWTSecretJWKS
	JWTSecretKey  = projectdefaults.JWTSecretKey
)

func SchemasOrDefault() string         { return restSchemasOrDefault() }
func MaxRowsOrDefault() string         { return restMaxRowsOrDefault() }
func ExtraSearchPathOrDefault() string { return restExtraSearchPathOrDefault() }
