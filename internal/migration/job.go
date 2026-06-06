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
	"maps"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	"github.com/supabase-community/supabase-kubernetes/internal/database"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// JobName returns the name of the Job that applies migrations.
func JobName(migrationName string) string {
	return fmt.Sprintf("%s-apply", migrationName)
}

// BuildJob constructs the migration Job.
func BuildJob(migration *supabasev1alpha1.Migration, db *database.ResolvedDatabase, image, batchHash string) *batchv1.Job {
	backoffLimit := int32(0)
	ttlSecondsAfterFinished := int32(86400)
	configMapName := ConfigMapName(migration.Name)

	script := assets.MigrationApplyScript

	env := make([]corev1.EnvVar, 0, 8+len(migration.Spec.Env))
	env = append(env,
		helper.EnvVarFromSecret("PGPASSWORD", db.SecretName, db.SecretPasswordKey),
		helper.EnvVarFromSecret("POSTGRES_PASSWORD", db.SecretName, db.SecretPasswordKey),
		helper.EnvVar("PGHOST", db.Host),
		helper.EnvVar("PGPORT", fmt.Sprintf("%d", db.Port)),
		helper.EnvVar("PGUSER", db.User),
		helper.EnvVar("POSTGRES_USER", db.User),
		helper.EnvVar("PGDATABASE", db.DBName),
		helper.EnvVar("MIGRATION_HASH", batchHash),
	)
	env = append(env, migration.Spec.Env...)

	container := corev1.Container{
		Name:            Component,
		Image:           image,
		ImagePullPolicy: migration.Spec.ImagePullPolicy,
		Command:         []string{"/bin/sh", "-c"},
		Args:            []string{script},
		Env:             env,
		Resources:       migration.Spec.Resources,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "migration-sql",
				MountPath: "/migrations",
				ReadOnly:  true,
			},
		},
	}

	if migration.Spec.ContainerSecurityContext != nil {
		container.SecurityContext = migration.Spec.ContainerSecurityContext
	}

	podLabels := maps.Clone(migration.Spec.PodLabels)
	if podLabels == nil {
		podLabels = map[string]string{}
	}

	podAnnotations := maps.Clone(migration.Spec.PodAnnotations)
	if podAnnotations == nil {
		podAnnotations = map[string]string{}
	}

	podSpec := corev1.PodSpec{
		RestartPolicy:                 corev1.RestartPolicyNever,
		Containers:                    []corev1.Container{container},
		NodeSelector:                  migration.Spec.NodeSelector,
		Affinity:                      migration.Spec.Affinity,
		Tolerations:                   migration.Spec.Tolerations,
		PriorityClassName:             migration.Spec.PriorityClassName,
		TerminationGracePeriodSeconds: migration.Spec.TerminationGracePeriodSeconds,
		Volumes: []corev1.Volume{
			{
				Name: "migration-sql",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapName,
						},
					},
				},
			},
		},
	}

	if migration.Spec.PodSecurityContext != nil {
		podSpec.SecurityContext = migration.Spec.PodSecurityContext
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(migration.Name),
			Namespace: migration.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: podAnnotations,
				},
				Spec: podSpec,
			},
		},
	}
}
