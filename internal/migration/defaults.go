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
	corev1 "k8s.io/api/core/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

const (
	// DefaultPostgresImage is the default Postgres image used by migration Jobs.
	DefaultPostgresImage = "supabase/postgres:17.6.1.084"

	// DefaultMigrationTable is the table used to track applied migrations.
	DefaultMigrationTable = "supabase_operator._operator_migrations"

	// DefaultBackoffLimit is the number of retries before marking a Job as failed.
	DefaultBackoffLimit int32 = 3

	// DefaultTTLSecondsAfterFinished is the TTL for cleaning up finished Jobs.
	DefaultTTLSecondsAfterFinished int32 = 30

	// MigrationsMountPath is the path where the migration ConfigMap is mounted.
	MigrationsMountPath = "/etc/supabase/migrations"

	// MigrationsBatchFile is the file name of the batched SQL inside the ConfigMap.
	MigrationsBatchFile = "batch.sql"
)

// MigrationLabels returns the common labels for a Migration and its resources.
func MigrationLabels(migration *supabasev1alpha1.Migration) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "migration",
		"app.kubernetes.io/component":  "migration",
		"app.kubernetes.io/instance":   migration.Name,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}

// MigrationSelectorLabels returns the selector labels for the Migration Job.
func MigrationSelectorLabels(migration *supabasev1alpha1.Migration) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "migration",
		"app.kubernetes.io/component": "migration",
		"app.kubernetes.io/instance":  migration.Name,
	}
}

// getImageOrDefault returns the migration image, preferring an explicit spec
// override, then the resolved database image, then the default image.
func getImageOrDefault(migration *supabasev1alpha1.Migration, db *supabasev1alpha1.ResolvedDatabase) string {
	if migration.Spec.Image != nil && *migration.Spec.Image != "" {
		return *migration.Spec.Image
	}
	if db != nil && db.Image != "" {
		return db.Image
	}
	return DefaultPostgresImage
}

// getImagePullPolicyOrDefault returns the image pull policy from the spec or the default.
func getImagePullPolicyOrDefault(migration *supabasev1alpha1.Migration) corev1.PullPolicy {
	if migration.Spec.ImagePullPolicy != nil {
		return *migration.Spec.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}
