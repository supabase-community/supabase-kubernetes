package auth

import (
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	projectdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
)

type ResourceContext = defaults.Context

const DefaultAuthPort int32 = 9999

func ComponentName(name, suffix string) string { return defaults.ComponentName(name, suffix) }
func AuthLabels(ctx *ResourceContext) map[string]string {
	return defaults.ComponentLabels(ctx, "Auth", "auth", "auth", "auth")
}
func AuthSelectorLabels(ctx *ResourceContext) map[string]string {
	return defaults.SelectorLabels(ctx, "Auth", "auth", "auth", "auth")
}

var JWTSecretName = projectdefaults.JWTSecretName

const (
	JWTSecretKey  = projectdefaults.JWTSecretKey
	JWTSecretKeys = projectdefaults.JWTSecretKeys
)
