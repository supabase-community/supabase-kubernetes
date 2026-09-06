package edgeruntime

import (
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	projectdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
)

type ResourceContext = defaults.Context

const DefaultFunctionsPort int32 = 9000

func ComponentName(name, suffix string) string { return defaults.ComponentName(name, suffix) }
func FunctionsLabels(ctx *ResourceContext) map[string]string {
	return defaults.ComponentLabels(ctx, "EdgeRuntime", "edge-runtime", "functions", "functions")
}
func FunctionsSelectorLabels(ctx *ResourceContext) map[string]string {
	return defaults.SelectorLabels(ctx, "EdgeRuntime", "edge-runtime", "functions", "functions")
}

func ProjectLabels(ctx *ResourceContext) map[string]string {
	return FunctionsLabels(ctx)
}

var APIExternalURL = projectdefaults.APIExternalURL
var JWTSecretName = projectdefaults.JWTSecretName

func EnvoyServiceName(ctx *ResourceContext) string {
	return defaults.ComponentName(ctx.Names["Envoy"], "envoy")
}

const (
	DefaultEnvoyPort        int32 = 8000
	JWTSecretKey                  = projectdefaults.JWTSecretKey
	JWTSecretAnonKey              = projectdefaults.JWTSecretAnonKey
	JWTSecretServiceKey           = projectdefaults.JWTSecretServiceKey
	JWTSecretPublishableKey       = projectdefaults.JWTSecretPublishableKey
	JWTSecretOpaqueKey            = projectdefaults.JWTSecretOpaqueKey
)
