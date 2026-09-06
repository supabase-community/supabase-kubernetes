package meta

import (
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	projectdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
)

type ResourceContext = defaults.Context

const DefaultMetaPort int32 = 8080

func ComponentName(name, suffix string) string { return defaults.ComponentName(name, suffix) }
func MetaLabels(ctx *ResourceContext) map[string]string {
	return defaults.ComponentLabels(ctx, "Meta", "meta", "meta", "meta")
}
func MetaSelectorLabels(ctx *ResourceContext) map[string]string {
	return defaults.SelectorLabels(ctx, "Meta", "meta", "meta", "meta")
}

var KeysSecretName = projectdefaults.KeysSecretName

const KeysSecretCryptoKey = projectdefaults.KeysSecretCryptoKey
