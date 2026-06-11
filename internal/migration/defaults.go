/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package migration

import (
	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	// DefaultAppName is the value for the name label.
	DefaultAppName = "supabase"
	// DefaultComponent is the Kubernetes component label value for migrations.
	DefaultComponent = "migration"
	// DefaultManagedBy is the value for the managed-by label.
	DefaultManagedBy = "supabase-operator"
	// DefaultContainerName is the name of the migration container.
	DefaultContainerName = "migration"
	// DefaultVolumeName is the name of the migration SQL volume.
	DefaultVolumeName = "migration-sql"
	// DefaultConfigMapKey is the ConfigMap data key for the batched SQL.
	DefaultConfigMapKey = "batch.sql"
	// DefaultBackoffLimit is the Job backoff limit.
	DefaultBackoffLimit = int32(0)
	// DefaultTTLSecondsAfterFinished is the Job TTL after finished.
	DefaultTTLSecondsAfterFinished = int32(30)
	// DefaultRestartPolicy is the Job pod restart policy.
	DefaultRestartPolicy = corev1.RestartPolicyNever
	// DefaultMigrationTable is the default name of the migrations tracking table.
	DefaultMigrationTable = "_supabase_operator_migrations"
	// DefaultMigrationMountPath is the mount path for the migration SQL volume.
	DefaultMigrationMountPath = "/migrations"
	// DefaultMigrationImage is the default container image for Migration jobs.
	DefaultMigrationImage = "supabase/postgres:17.6.1.084"
)

// DefaultLabels returns the standard labels for Migration resources.
func DefaultLabels(instanceName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       DefaultAppName,
		"app.kubernetes.io/instance":   instanceName,
		"app.kubernetes.io/component":  DefaultComponent,
		"app.kubernetes.io/managed-by": DefaultManagedBy,
	}
}

// ResolveImage returns the container image for a Migration.
// If migration.Spec.Image is set, it returns that value directly.
// Otherwise, it resolves the image from the version/component registry.
func ResolveImage(migration *supabasev1alpha1.Migration) string {
	if migration.Spec.Image != nil && *migration.Spec.Image != "" {
		return *migration.Spec.Image
	}
	return DefaultMigrationImage
}
