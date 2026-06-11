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
	"maps"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobName returns the name of the Job that applies migrations.
func JobName(migrationName string) string {
	return fmt.Sprintf("%s-apply", migrationName)
}

// BuildJob constructs the migration Job.
func BuildJob(migration *supabasev1alpha1.Migration, db *supabasev1alpha1.ResolvedDatabase) *batchv1.Job {
	backoffLimit := DefaultBackoffLimit
	ttlSecondsAfterFinished := DefaultTTLSecondsAfterFinished

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(migration.Name),
			Namespace: migration.Namespace,
			Labels:    DefaultLabels(migration.Name),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      PodLabels(migration),
					Annotations: PodAnnotations(migration),
				},
				Spec: PodSpec(migration, db),
			},
		},
	}
}

// PodLabels returns the labels for the Migration pod, merging defaults with user-provided pod labels.
func PodLabels(migration *supabasev1alpha1.Migration) map[string]string {
	labels := DefaultLabels(migration.Name)
	maps.Copy(labels, migration.Spec.PodLabels)
	return labels
}

// PodAnnotations returns the annotations for the Migration pod, merging defaults with user-provided pod annotations.
func PodAnnotations(migration *supabasev1alpha1.Migration) map[string]string {
	annotations := map[string]string{}
	maps.Copy(annotations, migration.Spec.PodAnnotations)
	return annotations
}

// PodSpec constructs the PodSpec for a Migration pod.
func PodSpec(migration *supabasev1alpha1.Migration, db *supabasev1alpha1.ResolvedDatabase) corev1.PodSpec {
	podSpec := corev1.PodSpec{
		RestartPolicy: DefaultRestartPolicy,
		Containers:    []corev1.Container{MainContainer(migration, db)},
		Volumes: []corev1.Volume{
			{
				Name: DefaultVolumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: ConfigMapName(migration.Name),
						},
					},
				},
			},
		},
		NodeSelector:                  migration.Spec.NodeSelector,
		Tolerations:                   migration.Spec.Tolerations,
		Affinity:                      migration.Spec.Affinity,
		TerminationGracePeriodSeconds: migration.Spec.TerminationGracePeriodSeconds,
		SecurityContext:               migration.Spec.PodSecurityContext,
	}
	if migration.Spec.PriorityClassName != nil {
		podSpec.PriorityClassName = *migration.Spec.PriorityClassName
	}
	return podSpec
}

// MainContainer constructs the main migration container.
func MainContainer(migration *supabasev1alpha1.Migration, db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	container := corev1.Container{
		Name:    DefaultContainerName,
		Image:   ResolveImage(migration),
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{assets.MigrationApplyScript},
		Env: []corev1.EnvVar{
			helper.EnvVarFromSecret("PGPASSWORD", db.PasswordRef.Name, db.PasswordRef.Key),
			helper.EnvVar("PGHOST", db.Host),
			helper.EnvVar("PGPORT", fmt.Sprintf("%d", db.Port)),
			helper.EnvVar("PGUSER", db.User),
			helper.EnvVar("PGDATABASE", db.DBName),
			helper.EnvVar("MIGRATION_HASH", CalculateMigrationHash(migration)),
			helper.EnvVar("MIGRATION_TABLE", DefaultMigrationTable),
			helper.EnvVar("MIGRATION_BATCH_PATH", DefaultMigrationMountPath+"/"+DefaultConfigMapKey),
		},
		SecurityContext: migration.Spec.ContainerSecurityContext,
		VolumeMounts:    []corev1.VolumeMount{{Name: DefaultVolumeName, MountPath: DefaultMigrationMountPath, ReadOnly: true}},
	}
	if migration.Spec.ImagePullPolicy != nil {
		container.ImagePullPolicy = *migration.Spec.ImagePullPolicy
	}
	if migration.Spec.Resources != nil {
		container.Resources = *migration.Spec.Resources
	}
	container.Env = append(container.Env, migration.Spec.Env...)

	return container
}

// CalculateMigrationHash computes a SHA-256 hash over the ordered migration SQLs.
func CalculateMigrationHash(migration *supabasev1alpha1.Migration) string {
	h := sha256.New()
	for _, entry := range migration.Spec.Migrations {
		// Delimiter ensures concatenation is unambiguous
		h.Write([]byte(entry.SQL))
		h.Write([]byte("\x00MIGRATION\x00"))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
