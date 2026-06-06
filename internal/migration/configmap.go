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
	"crypto/sha256"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

// ConfigMapName returns the name of the ConfigMap that holds the migration SQL.
func ConfigMapName(migrationName string) string {
	return fmt.Sprintf("%s-sql", migrationName)
}

// BuildConfigMap constructs the ConfigMap containing the batched SQL migrations.
func BuildConfigMap(migration *supabasev1alpha1.Migration, name string, batchHash string) *corev1.ConfigMap {
	batchSQL := BuildBatchSQL(migration.Spec.Migrations, batchHash)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: migration.Namespace,
		},
		Data: map[string]string{
			"batch.sql": batchSQL,
		},
	}
}

// BuildBatchSQL concatenates all migration entries into a single SQL batch.
func BuildBatchSQL(entries []supabasev1alpha1.MigrationEntry, batchHash string) string {
	var b strings.Builder
	for i, entry := range entries {
		b.WriteString(fmt.Sprintf("-- migration %d: %s\n", i, entry.Name))
		b.WriteString(entry.SQL)
		b.WriteString("\n\n")
	}
	b.WriteString(fmt.Sprintf("INSERT INTO _migrations (hash) VALUES ('%s');\n", batchHash))
	return b.String()
}

// CalculateBatchHash computes a SHA-256 hash over the ordered migration SQLs.
func CalculateBatchHash(entries []supabasev1alpha1.MigrationEntry) string {
	h := sha256.New()
	for _, entry := range entries {
		// Delimiter ensures concatenation is unambiguous
		h.Write([]byte(entry.SQL))
		h.Write([]byte("\x00MIGRATION\x00"))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
