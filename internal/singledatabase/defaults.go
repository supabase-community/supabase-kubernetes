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

package singledatabase

import (
	"fmt"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

const (
	// DefaultPostgresImage is the default Supabase Postgres image.
	DefaultPostgresImage = "supabase/postgres:17.6.1.084"

	// DefaultPostgresPort is the default Postgres port.
	DefaultPostgresPort int32 = 5432

	// DefaultPostgresUser is the default Postgres user.
	DefaultPostgresUser = "supabase_admin"

	// DefaultPostgresDatabase is the default Postgres database name.
	DefaultPostgresDatabase = "postgres"

	// DefaultSecretKeyPassword is the Secret data key that holds the Postgres password.
	DefaultSecretKeyPassword = "password"

	// PostgresDataMountPath is the path where Postgres data is stored.
	PostgresDataMountPath = "/var/lib/postgresql/data"

	// PostgresDataSubPath is the subPath used inside the PVC.
	PostgresDataSubPath = "postgres-data"

	// PostgresCustomMountPath is the path where Postgres custom config (incl. pgsodium_root.key) lives.
	PostgresCustomMountPath = "/etc/postgresql-custom"

	// PostgresCustomSubPath is the subPath used inside the PVC for the custom config.
	PostgresCustomSubPath = "postgres-custom"
)

// PostgresLabels returns the common labels for a SingleDatabase and its resources.
func PostgresLabels(db *supabasev1alpha1.SingleDatabase) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "postgres",
		"app.kubernetes.io/component":  "database",
		"app.kubernetes.io/instance":   db.Name,
		"app.kubernetes.io/managed-by": "supabase-operator",
	}
}

// PostgresSelectorLabels returns the selector labels for the SingleDatabase StatefulSet.
func PostgresSelectorLabels(db *supabasev1alpha1.SingleDatabase) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "postgres",
		"app.kubernetes.io/component": "database",
		"app.kubernetes.io/instance":  db.Name,
	}
}

// PostgresServiceHost returns the fully qualified DNS name of the SingleDatabase service.
func PostgresServiceHost(db *supabasev1alpha1.SingleDatabase) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", PostgresServiceName(db), db.Namespace)
}

// PostgresPVCDeletionPolicy returns the effective deletion policy for the SingleDatabase PVC.
func PostgresPVCDeletionPolicy(db *supabasev1alpha1.SingleDatabase) supabasev1alpha1.DeletionPolicy {
	return *db.Spec.Storage.DeletionPolicy
}
