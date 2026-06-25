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

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

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

// MigrationHash computes a SHA-256 hash over the ordered migration SQLs.
func MigrationHash(migration *supabasev1alpha1.Migration) string {
	h := sha256.New()
	for _, entry := range migration.Spec.Migrations {
		// Delimiter ensures concatenation is unambiguous
		h.Write([]byte(entry.SQL))
		h.Write([]byte("\x00MIGRATION\x00"))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
