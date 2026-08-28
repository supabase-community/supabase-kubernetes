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
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// SyncPasswordJobName returns the name of the Job that syncs passwords for a Project.
func SyncPasswordJobName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-sync-password", project.Name)
}

// SyncPasswordJob constructs the Job that syncs passwords for a Project.
func SyncPasswordJob(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) (*batchv1.Job, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SyncPasswordJobName(project),
			Namespace: project.Namespace,
			Labels:    ProjectLabels(project),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr.To(DefaultBackoffLimit),
			TTLSecondsAfterFinished: ptr.To(DefaultTTLSecondsAfterFinished),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ProjectLabels(project),
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						buildSyncPasswordContainer(db),
					},
				},
			},
		},
	}

	return job, nil
}

// buildSyncPasswordContainer returns the password sync container specification.
func buildSyncPasswordContainer(db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	return corev1.Container{
		Name:            "sync-password",
		Image:           postgresImageOrDefault(db),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/bin/sh", "-c"},
		Args:            []string{assets.ProjectSyncPasswordScript},
		Env:             buildSyncPasswordEnvVars(db),
	}
}

// buildSyncPasswordEnvVars returns the environment variables for the password sync container.
func buildSyncPasswordEnvVars(db *supabasev1alpha1.ResolvedDatabase) []corev1.EnvVar {
	return []corev1.EnvVar{
		helper.EnvVar("PGHOST", db.Host),
		helper.EnvVar("PGPORT", strconv.Itoa(int(db.Port))),
		helper.EnvVar("PGUSER", db.User),
		helper.EnvVarFromSecret("PGPASSWORD", db.PasswordRef.Name, db.PasswordRef.Key),
		helper.EnvVar("PGDATABASE", db.DBName),
		helper.EnvVar("DB_ADMIN_USER", db.User),
		helper.EnvVar("DB_SRV_NAME", db.Host),
		helper.EnvVar("DB_SRV_PORT", strconv.Itoa(int(db.Port))),
	}
}
