package supavisor

import (
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	projectdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
)

type ResourceContext = defaults.Context

const (
	DefaultSupavisorPort   int32 = 4000
	DefaultSessionPort     int32 = 5432
	DefaultTransactionPort int32 = 6543
	DatabaseURLKey               = "database-url"
)

func ComponentName(name, suffix string) string { return defaults.ComponentName(name, suffix) }
func SupavisorLabels(ctx *ResourceContext) map[string]string {
	return defaults.ComponentLabels(ctx, "Supavisor", "supavisor", "supavisor", "supavisor")
}
func SupavisorSelectorLabels(ctx *ResourceContext) map[string]string {
	return defaults.SelectorLabels(ctx, "Supavisor", "supavisor", "supavisor", "supavisor")
}

var KeysSecretName = projectdefaults.KeysSecretName
var JWTSecretName = projectdefaults.JWTSecretName

const (
	KeysSecretVaultEncKey   = projectdefaults.KeysSecretVaultEncKey
	KeysSecretSecretKeyBase = projectdefaults.KeysSecretSecretKeyBase
	JWTSecretKey            = projectdefaults.JWTSecretKey
)
