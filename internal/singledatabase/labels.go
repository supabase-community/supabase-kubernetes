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

const (
	// Component is the Kubernetes component label value for the database.
	Component = "singledatabase"
	// Port is the PostgreSQL port.
	Port = int32(5432)
	// ManagedBy is the value for the managed-by label.
	ManagedBy = "supabase-operator"
	// AppName is the value for the name label.
	AppName = "supabase"
	// Database is the default database to use on postgres.
	Database = "postgres"
)

// DefaultLabels returns the standard labels for SingleDatabase resources.
func DefaultLabels(instanceName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       AppName,
		"app.kubernetes.io/instance":   instanceName,
		"app.kubernetes.io/component":  Component,
		"app.kubernetes.io/managed-by": ManagedBy,
	}
}
