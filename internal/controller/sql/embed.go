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

package sql

import (
	"embed"
	"io/fs"
	"path"
	"sort"
	"strings"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

//go:embed scripts/migrate.sh
var MigrationScript string

// DefaultMigrationEntries returns the built-in project migration entries
// loaded from embedded SQL files.
func DefaultMigrationEntries() ([]platformv1alpha1.MigrationEntry, error) {
	entries := []platformv1alpha1.MigrationEntry{}

	files, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(files))
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}
		names = append(names, f.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		data, err := migrationFiles.ReadFile(path.Join("migrations", name))
		if err != nil {
			return nil, err
		}
		entryName := strings.TrimSuffix(name, ".sql")
		entries = append(entries, platformv1alpha1.MigrationEntry{
			Name: entryName,
			SQL:  string(data),
		})
	}

	return entries, nil
}
