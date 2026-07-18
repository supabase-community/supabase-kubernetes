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

package project

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
)

// ProjectMigration1Name returns the name of the first migration for a Project.
func ProjectMigration1Name(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-migration-1", project.Name)
}

// ProjectMigration2Name returns the name of the second migration for a Project.
// Add a new function like this instead of updating the existing migration.
// func ProjectMigration2Name(project *supabasev1alpha1.Project) string {
// 	return fmt.Sprintf("%s-migration-2", project.Name)
// }

// ProjectMigration1 constructs the first migration for a Project.
func ProjectMigration1(project *supabasev1alpha1.Project) (*supabasev1alpha1.Migration, error) {
	return &supabasev1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ProjectMigration1Name(project),
			Namespace: project.Namespace,
			Labels:    ProjectLabels(project),
		},
		Spec: supabasev1alpha1.MigrationSpec{
			DatabaseRef: project.Spec.DatabaseRef,
			Migrations: []supabasev1alpha1.MigrationEntry{
				{Name: "supabase.sql", SQL: assets.SupabaseMigration},
				{Name: "realtime.sql", SQL: assets.RealtimeMigration},
				{Name: "logs.sql", SQL: assets.LogsMigration},
				{Name: "pooler.sql", SQL: assets.PoolerMigration},
				{Name: "webhooks.sql", SQL: assets.WebhooksMigration},
			},
		},
	}, nil
}
