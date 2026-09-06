package realtime

import (
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	projectdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
)

type ResourceContext = defaults.Context

const DefaultRealtimePort int32 = 4000

func ComponentName(name, suffix string) string { return defaults.ComponentName(name, suffix) }
func RealtimeLabels(ctx *ResourceContext) map[string]string {
	return defaults.ComponentLabels(ctx, "Realtime", "realtime", "realtime", "realtime")
}
func RealtimeSelectorLabels(ctx *ResourceContext) map[string]string {
	return defaults.SelectorLabels(ctx, "Realtime", "realtime", "realtime", "realtime")
}

var KeysSecretName = projectdefaults.KeysSecretName
var JWTSecretName = projectdefaults.JWTSecretName

const (
	KeysSecretRealtimeDBEncKey = projectdefaults.KeysSecretRealtimeDBEncKey
	KeysSecretSecretKeyBase    = projectdefaults.KeysSecretSecretKeyBase
	JWTSecretKey               = projectdefaults.JWTSecretKey
	JWTSecretJWKS              = projectdefaults.JWTSecretJWKS
	JWTSecretAnonKey           = projectdefaults.JWTSecretAnonKey
)
