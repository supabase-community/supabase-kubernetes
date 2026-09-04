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
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// MigrationJobName returns the name of the Job that applies migrations.
func MigrationJobName(migration *supabasev1alpha1.Migration) string {
	return fmt.Sprintf("%s-job", migration.Name)
}

// MigrationJob constructs the migration Job.
func MigrationJob(migration *supabasev1alpha1.Migration, db *supabasev1alpha1.ResolvedDatabase) (*batchv1.Job, error) {
	template, err := helper.Overlay(corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: MigrationLabels(migration)},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Containers:    []corev1.Container{buildMigrationContainer(migration, db)},
			Volumes:       []corev1.Volume{buildMigrationVolume(migration)},
		},
	}, migration.Spec.Pod)
	if err != nil {
		return nil, err
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MigrationJobName(migration),
			Namespace: migration.Namespace,
			Labels:    MigrationLabels(migration),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr.To(DefaultBackoffLimit),
			TTLSecondsAfterFinished: ptr.To(DefaultTTLSecondsAfterFinished),
			Template:                template,
		},
	}

	return job, nil
}

// buildMigrationContainer returns the migration container specification.
func buildMigrationContainer(migration *supabasev1alpha1.Migration, db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	return corev1.Container{
		Name:            "migration",
		Image:           getImageOrDefault(migration),
		ImagePullPolicy: getImagePullPolicyOrDefault(migration),
		Command:         []string{"/bin/sh", "-c"},
		Args:            []string{assets.MigrationApplyScript},
		Env:             buildMigrationEnvVars(migration, db),
		Resources:       corev1.ResourceRequirements{},
		VolumeMounts:    migrationVolumeMounts(),
	}
}

// migrationVolumeMounts returns the volume mounts for the migration container.
func migrationVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "migrations",
			MountPath: MigrationsMountPath,
		},
	}
}

// buildMigrationVolume returns the migration ConfigMap volume specification.
func buildMigrationVolume(migration *supabasev1alpha1.Migration) corev1.Volume {
	return corev1.Volume{
		Name: "migrations",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: MigrationConfigMapName(migration),
				},
			},
		},
	}
}

// buildMigrationEnvVars returns the environment variables for the migration container.
func buildMigrationEnvVars(migration *supabasev1alpha1.Migration, db *supabasev1alpha1.ResolvedDatabase) []corev1.EnvVar {
	return []corev1.EnvVar{
		helper.EnvVar("PGHOST", db.Host),
		helper.EnvVar("PGPORT", strconv.Itoa(int(db.Port))),
		helper.EnvVar("PGUSER", db.User),
		helper.EnvVarFromSecret("PGPASSWORD", db.PasswordRef.Name, db.PasswordRef.Key),
		helper.EnvVar("PGDATABASE", db.DBName),
		helper.EnvVar("MIGRATION_TABLE", DefaultMigrationTable),
		helper.EnvVar("MIGRATION_HASH", MigrationHash(migration)),
		helper.EnvVar("MIGRATION_BATCH_PATH", fmt.Sprintf("%s/%s", MigrationsMountPath, MigrationsBatchFile)),
	}
}
