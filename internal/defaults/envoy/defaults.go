package envoy

import (
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults/auth"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults/edgeruntime"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults/meta"
	projectdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults/realtime"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults/rest"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults/storage"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults/studio"
)

type ResourceContext = defaults.Context

const (
	DefaultEnvoyPort              int32 = 8000
	DefaultEnvoyAdminPort         int32 = 9901
	DefaultEnvoySecretKeyUsername       = "username"
	DefaultEnvoySecretKeyPassword       = "password"
	EnvoyConfigMountPath                = "/etc/envoy"
	EnvoyConfigSourcePath               = "/etc/envoy-config"
)

func ComponentName(name, suffix string) string { return defaults.ComponentName(name, suffix) }
func EnvoyLabels(ctx *ResourceContext) map[string]string {
	return defaults.ComponentLabels(ctx, "Envoy", "envoy", "envoy", "gateway")
}
func EnvoySelectorLabels(ctx *ResourceContext) map[string]string {
	return defaults.SelectorLabels(ctx, "Envoy", "envoy", "envoy", "gateway")
}

var AuthServiceName = auth.AuthServiceName
var RestServiceName = rest.RestServiceName
var RealtimeServiceName = realtime.RealtimeServiceName
var FunctionsServiceName = edgeruntime.FunctionsServiceName
var MetaServiceName = meta.MetaServiceName
var StorageServiceName = storage.StorageServiceName
var StudioServiceName = studio.StudioServiceName
var JWTSecretName = projectdefaults.JWTSecretName

const (
	DefaultAuthPort         = auth.DefaultAuthPort
	DefaultRestPort         = rest.DefaultRestPort
	DefaultRealtimePort     = realtime.DefaultRealtimePort
	DefaultFunctionsPort    = edgeruntime.DefaultFunctionsPort
	DefaultMetaPort         = meta.DefaultMetaPort
	DefaultStoragePort      = storage.DefaultStoragePort
	DefaultStudioPort       = studio.DefaultStudioPort
	JWTSecretAnonKey        = projectdefaults.JWTSecretAnonKey
	JWTSecretServiceKey     = projectdefaults.JWTSecretServiceKey
	JWTSecretPublishableKey = projectdefaults.JWTSecretPublishableKey
	JWTSecretOpaqueKey      = projectdefaults.JWTSecretOpaqueKey
	JWTSecretAnonKeyAsym    = projectdefaults.JWTSecretAnonKeyAsym
	JWTSecretServiceKeyAsym = projectdefaults.JWTSecretServiceKeyAsym
)
