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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
)

// ProjectMigration1Name returns the name of the first migration for a Project.
func ProjectMigration1Name(project *ResourceContext) string {
	return fmt.Sprintf("%s-migration-1", project.Name)
}

// ProjectMigration1 constructs the first migration for a Project.
// Add a new numbered Migration instead of changing its immutable migration list.
func ProjectMigration1(project *ResourceContext) *supabasev1alpha1.Migration {
	if project.Spec.Migrations != nil && project.Spec.Migrations.Enable != nil && !*project.Spec.Migrations.Enable {
		return nil
	}

	var pod *corev1.PodTemplateSpec
	if project.Spec.Migrations != nil {
		pod = project.Spec.Migrations.Pod
	}

	return &supabasev1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ProjectMigration1Name(project),
			Namespace: project.Namespace,
			Labels:    ProjectLabels(project),
		},
		Spec: supabasev1alpha1.MigrationSpec{
			Pod:         pod,
			DatabaseRef: project.Spec.DatabaseRef,
			Migrations: []supabasev1alpha1.MigrationEntry{
				{Name: "supabase.sql", SQL: assets.SupabaseMigration},
				{Name: "realtime.sql", SQL: assets.RealtimeMigration},
				{Name: "logs.sql", SQL: assets.LogsMigration},
				{Name: "pooler.sql", SQL: assets.PoolerMigration},
				{Name: "webhooks.sql", SQL: assets.WebhooksMigration},
			},
		},
	}
}
