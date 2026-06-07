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
	"fmt"
	"strings"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapName returns the name of the ConfigMap that holds the migration SQL.
func ConfigMapName(migrationName string) string {
	return fmt.Sprintf("%s-sql", migrationName)
}

// BuildConfigMap constructs the ConfigMap containing the batched SQL migrations.
func BuildConfigMap(migration *supabasev1alpha1.Migration) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(migration.Name),
			Namespace: migration.Namespace,
			Labels:    DefaultLabels(migration.Name),
		},
		Data: map[string]string{
			DefaultConfigMapKey: BatchSQL(migration),
		},
	}
}

// BatchSQL concatenates all migration entries into a single SQL batch.
func BatchSQL(migration *supabasev1alpha1.Migration) string {
	var b strings.Builder
	for i, entry := range migration.Spec.Migrations {
		fmt.Fprintf(&b, "-- migration %d: %s\n", i, entry.Name)
		fmt.Fprint(&b, entry.SQL)
		fmt.Fprint(&b, "\n\n")
	}
	return b.String()
}
