package studio

import (
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	metadefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/meta"
	projectdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
	restdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/rest"
)

type ResourceContext = defaults.Context

const (
	DefaultStudioPort        int32 = 3000
	StudioSnippetsMountPath        = "/app/snippets"
	StudioSnippetsSubPath          = "snippets"
	StudioFunctionsMountPath       = "/app/edge-functions"
)

func ComponentName(name, suffix string) string { return defaults.ComponentName(name, suffix) }
func StudioLabels(ctx *ResourceContext) map[string]string {
	return defaults.ComponentLabels(ctx, "Studio", "studio", "studio", "studio")
}
func StudioSelectorLabels(ctx *ResourceContext) map[string]string {
	return defaults.SelectorLabels(ctx, "Studio", "studio", "studio", "studio")
}

var JWTSecretName = projectdefaults.JWTSecretName
var KeysSecretName = projectdefaults.KeysSecretName
var APIExternalURL = projectdefaults.APIExternalURL
var MetaServiceName = metadefaults.MetaServiceName

func EnvoyServiceName(ctx *ResourceContext) string {
	return defaults.ComponentName(ctx.Names["Envoy"], "envoy")
}

const (
	DefaultMetaPort               = metadefaults.DefaultMetaPort
	DefaultEnvoyPort        int32 = 8000
	KeysSecretCryptoKey           = projectdefaults.KeysSecretCryptoKey
	JWTSecretAnonKey              = projectdefaults.JWTSecretAnonKey
	JWTSecretServiceKey           = projectdefaults.JWTSecretServiceKey
	JWTSecretKey                  = projectdefaults.JWTSecretKey
	JWTSecretPublishableKey       = projectdefaults.JWTSecretPublishableKey
	JWTSecretOpaqueKey            = projectdefaults.JWTSecretOpaqueKey
)

func restSchemasOrDefault() string         { return restdefaults.SchemasOrDefault() }
func restMaxRowsOrDefault() string         { return restdefaults.MaxRowsOrDefault() }
func restExtraSearchPathOrDefault() string { return restdefaults.ExtraSearchPathOrDefault() }
