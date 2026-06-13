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

package singledatabase

import (
	"fmt"
	"maps"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// PostgresStatefulSetName returns the name of the StatefulSet for a SingleDatabase.
func PostgresStatefulSetName(db *supabasev1alpha1.SingleDatabase) string {
	return fmt.Sprintf("%s-postgres", db.Name)
}

// PostgresStatefulSet constructs the StatefulSet for a SingleDatabase.
func PostgresStatefulSet(db *supabasev1alpha1.SingleDatabase, secretHash string) (*appsv1.StatefulSet, error) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PostgresStatefulSetName(db),
			Namespace: db.Namespace,
			Labels:    PostgresLabels(db),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: PostgresServiceName(db),
			Replicas:    ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: PostgresSelectorLabels(db),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels(db),
					Annotations: podAnnotations(db, secretHash),
				},
				Spec: corev1.PodSpec{
					Affinity:                      db.Spec.Affinity,
					NodeSelector:                  db.Spec.NodeSelector,
					Tolerations:                   db.Spec.Tolerations,
					PriorityClassName:             ptr.Deref(db.Spec.PriorityClassName, ""),
					SecurityContext:               db.Spec.SecurityContext,
					TerminationGracePeriodSeconds: db.Spec.TerminationGracePeriodSeconds,
					InitContainers: []corev1.Container{
						buildPasswordSyncInitContainer(db),
					},
					Containers: []corev1.Container{
						buildPostgresContainer(db),
					},
					Volumes: []corev1.Volume{
						buildPostgresVolume(db),
					},
				},
			},
		},
	}

	return sts, nil
}

// podLabels returns the merged pod labels for a SingleDatabase.
func podLabels(db *supabasev1alpha1.SingleDatabase) map[string]string {
	labels := maps.Clone(PostgresLabels(db))
	maps.Copy(labels, db.Spec.PodLabels)
	return labels
}

// podAnnotations returns the merged pod annotations for a SingleDatabase,
// including an annotation with the secret hash when provided.
func podAnnotations(db *supabasev1alpha1.SingleDatabase, secretHash string) map[string]string {
	annotations := make(map[string]string, len(db.Spec.PodAnnotations)+1)
	if secretHash != "" {
		annotations["supabase.io/secret-hash"] = secretHash
	}
	maps.Copy(annotations, db.Spec.PodAnnotations)
	return annotations
}

// buildPostgresContainer returns the Postgres container specification.
func buildPostgresContainer(db *supabasev1alpha1.SingleDatabase) corev1.Container {
	return corev1.Container{
		Name:            "postgres",
		Image:           getImageOrDefault(db),
		ImagePullPolicy: getImagePullPolicyOrDefault(db),
		Args: []string{
			"-c", "config_file=/etc/postgresql/postgresql.conf",
			"-c", "log_min_messages=fatal",
		},
		Env:            buildPostgresEnvVars(db),
		Ports:          postgresPorts(),
		Resources:      ptr.Deref(db.Spec.Resources, corev1.ResourceRequirements{}),
		VolumeMounts:   postgresVolumeMounts(),
		LivenessProbe:  postgresLivenessProbe(),
		ReadinessProbe: postgresReadinessProbe(),
		StartupProbe:   postgresStartupProbe(),
		Lifecycle:      postgresLifecycle(),
	}
}

// buildPasswordSyncInitContainer returns the password-sync init container specification.
func buildPasswordSyncInitContainer(db *supabasev1alpha1.SingleDatabase) corev1.Container {
	return corev1.Container{
		Name:            "password-sync",
		Image:           getImageOrDefault(db),
		ImagePullPolicy: getImagePullPolicyOrDefault(db),
		Command:         []string{"/bin/sh", "-c"},
		Args:            []string{assets.SingleDatabasePasswordSyncScript},
		Env:             buildPostgresEnvVars(db),
		VolumeMounts:    postgresVolumeMounts(),
	}
}

// postgresVolumeMounts returns the volume mounts for the Postgres containers.
func postgresVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "postgres-data",
			MountPath: PostgresDataMountPath,
			SubPath:   PostgresDataSubPath,
		},
	}
}

// buildPostgresVolume returns the Postgres data volume specification.
func buildPostgresVolume(db *supabasev1alpha1.SingleDatabase) corev1.Volume {
	return corev1.Volume{
		Name: "postgres-data",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: PostgresPVCName(db),
			},
		},
	}
}

// postgresLifecycle returns the lifecycle hooks for the Postgres container.
func postgresLifecycle() *corev1.Lifecycle {
	return &corev1.Lifecycle{
		PreStop: &corev1.LifecycleHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"/bin/sh", "-c",
					"pg_ctl -D /var/lib/postgresql/data -w -t 60 -m fast stop",
				},
			},
		},
	}
}

// getImageOrDefault returns the Postgres image from the spec or the default image.
func getImageOrDefault(db *supabasev1alpha1.SingleDatabase) string {
	if db.Spec.Image != nil && *db.Spec.Image != "" {
		return *db.Spec.Image
	}
	return DefaultPostgresImage
}

// getImagePullPolicyOrDefault returns the image pull policy from the spec or the default.
func getImagePullPolicyOrDefault(db *supabasev1alpha1.SingleDatabase) corev1.PullPolicy {
	if db.Spec.ImagePullPolicy != nil {
		return *db.Spec.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

// buildPostgresEnvVars returns the environment variables for the Postgres containers.
func buildPostgresEnvVars(db *supabasev1alpha1.SingleDatabase) []corev1.EnvVar {
	secretName := PostgresSecretName(db)
	port := strconv.Itoa(int(DefaultPostgresPort))

	return []corev1.EnvVar{
		helper.EnvVar("POSTGRES_HOST", "/var/run/postgresql"),
		helper.EnvVar("PGPORT", port),
		helper.EnvVar("POSTGRES_PORT", port),
		helper.EnvVarFromSecret("PGPASSWORD", secretName, DefaultSecretKeyPassword),
		helper.EnvVarFromSecret("POSTGRES_PASSWORD", secretName, DefaultSecretKeyPassword),
		helper.EnvVar("PGDATABASE", DefaultPostgresDatabase),
		helper.EnvVar("POSTGRES_DB", DefaultPostgresDatabase),
	}
}

// postgresPorts returns the container ports for the Postgres container.
func postgresPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "postgres",
			ContainerPort: DefaultPostgresPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

// postgresLivenessProbe returns the liveness probe for the Postgres container.
func postgresLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        postgresProbeHandler(),
		InitialDelaySeconds: 30,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		FailureThreshold:    6,
	}
}

// postgresReadinessProbe returns the readiness probe for the Postgres container.
func postgresReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        postgresProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    10,
	}
}

// postgresStartupProbe returns the startup probe for the Postgres container.
func postgresStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        postgresProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    30,
	}
}

// postgresProbeHandler returns the shared probe handler for Postgres health checks.
func postgresProbeHandler() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"pg_isready",
				"-U",
				DefaultPostgresUser,
				"-h",
				"localhost",
			},
		},
	}
}
