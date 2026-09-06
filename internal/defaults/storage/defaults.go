package storage

import (
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	projectdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
	restdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/rest"
)

type ResourceContext = defaults.Context

const (
	DefaultStoragePort           int32 = 5000
	StorageDataMountPath               = "/var/lib/storage"
	StorageDataSubPath                 = "storage-data"
	StorageSecretAccessKeyID           = "accessKeyId"
	StorageSecretAccessKeySecret       = "accessKeySecret"
)

func ComponentName(name, suffix string) string { return defaults.ComponentName(name, suffix) }
func StorageLabels(ctx *ResourceContext) map[string]string {
	return defaults.ComponentLabels(ctx, "Storage", "storage", "storage", "storage")
}
func StorageSelectorLabels(ctx *ResourceContext) map[string]string {
	return defaults.SelectorLabels(ctx, "Storage", "storage", "storage", "storage")
}

var APIExternalURL = projectdefaults.APIExternalURL
var JWTSecretName = projectdefaults.JWTSecretName
var RestServiceName = restdefaults.RestServiceName

const (
	DefaultRestPort     = restdefaults.DefaultRestPort
	JWTSecretAnonKey    = projectdefaults.JWTSecretAnonKey
	JWTSecretServiceKey = projectdefaults.JWTSecretServiceKey
	JWTSecretKey        = projectdefaults.JWTSecretKey
	JWTSecretJWKS       = projectdefaults.JWTSecretJWKS
)
