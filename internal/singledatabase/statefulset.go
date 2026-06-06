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

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatefulSetName returns the name of the StatefulSet for a SingleDatabase.
func StatefulSetName(dbName string) string {
	return fmt.Sprintf("%s-db", dbName)
}

// BuildStatefulSet constructs the StatefulSet for a SingleDatabase.
func BuildStatefulSet(db *supabasev1alpha1.SingleDatabase, secretHash, configMapHash string) *appsv1.StatefulSet {
	replicas := DefaultReplicas

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StatefulSetName(db.Name),
			Namespace: db.Namespace,
			Labels:    DefaultLabels(db.Name),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			Selector:    &metav1.LabelSelector{MatchLabels: DefaultLabels(db.Name)},
			ServiceName: ServiceName(db.Name),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      PodLabels(db),
					Annotations: PodAnnotations(db, secretHash, configMapHash),
				},
				Spec: PodSpec(db),
			},
		},
	}
}

// PodLabels returns the labels for the SingleDatabase pod, merging defaults with user-provided pod labels.
func PodLabels(db *supabasev1alpha1.SingleDatabase) map[string]string {
	labels := DefaultLabels(db.Name)
	maps.Copy(labels, db.Spec.PodLabels)
	return labels
}

// PodAnnotations returns the annotations for the SingleDatabase pod, merging hash annotations with user-provided pod annotations.
func PodAnnotations(db *supabasev1alpha1.SingleDatabase, secretHash, configMapHash string) map[string]string {
	annotations := map[string]string{
		DefaultSecretHashAnnotation:    secretHash,
		DefaultConfigMapHashAnnotation: configMapHash,
	}
	maps.Copy(annotations, db.Spec.PodAnnotations)
	return annotations
}

// PodSpec constructs the PodSpec for a SingleDatabase pod.
func PodSpec(db *supabasev1alpha1.SingleDatabase) corev1.PodSpec {
	return corev1.PodSpec{
		InitContainers: []corev1.Container{PasswordSyncInitContainer(db)},
		Containers:     []corev1.Container{MainContainer(db)},
		Volumes: []corev1.Volume{
			{
				Name: DefaultVolumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: PVCName(db.Name),
					},
				},
			},
		},
		NodeSelector:                  db.Spec.NodeSelector,
		Tolerations:                   db.Spec.Tolerations,
		Affinity:                      db.Spec.Affinity,
		PriorityClassName:             db.Spec.PriorityClassName,
		TerminationGracePeriodSeconds: db.Spec.TerminationGracePeriodSeconds,
		SecurityContext:               db.Spec.PodSecurityContext,
	}
}

// MainContainer constructs the main PostgreSQL container for a SingleDatabase.
func MainContainer(db *supabasev1alpha1.SingleDatabase) corev1.Container {
	secretName := SecretName(db.Name)
	configMapName := ConfigMapName(db.Name)

	container := corev1.Container{
		Name:            DefaultComponent,
		Image:           ResolveImage(db),
		ImagePullPolicy: db.Spec.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          DefaultContainerPortName,
				ContainerPort: DefaultPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			helper.EnvVarFromSecret("POSTGRES_PASSWORD", secretName, DefaultSecretPasswordKey),
			helper.EnvVarFromSecret("PGPASSWORD", secretName, DefaultSecretPasswordKey),
			helper.EnvVarFromConfigMap("POSTGRES_DB", configMapName, DefaultConfigMapKeyDatabase),
			helper.EnvVarFromConfigMap("PGDATABASE", configMapName, DefaultConfigMapKeyDatabase),
			helper.EnvVarFromConfigMap("POSTGRES_PORT", configMapName, DefaultConfigMapKeyPort),
			helper.EnvVarFromConfigMap("PGPORT", configMapName, DefaultConfigMapKeyPort),
			helper.EnvVar("POSTGRES_HOST", DefaultPostgresHost),
			helper.EnvVar("PGHOST", DefaultPostgresHost),
		},
		StartupProbe:    StartupProbe(db),
		ReadinessProbe:  ReadinessProbe(db),
		LivenessProbe:   LivenessProbe(db),
		Resources:       db.Spec.Resources,
		SecurityContext: db.Spec.ContainerSecurityContext,
		VolumeMounts:    []corev1.VolumeMount{{Name: DefaultVolumeName, MountPath: DefaultDataMountPath}},
	}
	container.Env = append(container.Env, db.Spec.Env...)

	return container
}

// PasswordSyncInitContainer constructs the init container that synchronizes
// the PostgreSQL password on disk with the Secret value.
func PasswordSyncInitContainer(db *supabasev1alpha1.SingleDatabase) corev1.Container {
	return corev1.Container{
		Name:            DefaultInitContainerName,
		Image:           ResolveImage(db),
		ImagePullPolicy: db.Spec.ImagePullPolicy,
		Command:         []string{"sh", "-c", assets.SingleDatabasePasswordSyncScript},
		Env: []corev1.EnvVar{
			helper.EnvVarFromSecret("PGPASSWORD", SecretName(db.Name), DefaultSecretPasswordKey),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: DefaultVolumeName, MountPath: DefaultDataMountPath},
		},
	}
}

// StartupProbe returns the startup probe for the database container.
// When db.Spec.Probes is nil or db.Spec.Probes.Startup is nil, a default pg_isready probe is returned.
func StartupProbe(db *supabasev1alpha1.SingleDatabase) *corev1.Probe {
	if db.Spec.Probes != nil && db.Spec.Probes.Startup != nil {
		return db.Spec.Probes.Startup
	}
	return DefaultStartupProbe()
}

// ReadinessProbe returns the readiness probe for the database container.
// When db.Spec.Probes is nil or db.Spec.Probes.Readiness is nil, a default pg_isready probe is returned.
func ReadinessProbe(db *supabasev1alpha1.SingleDatabase) *corev1.Probe {
	if db.Spec.Probes != nil && db.Spec.Probes.Readiness != nil {
		return db.Spec.Probes.Readiness
	}
	return DefaultReadinessProbe()
}

// LivenessProbe returns the liveness probe for the database container.
// When db.Spec.Probes is nil or db.Spec.Probes.Liveness is nil, a default pg_isready probe is returned.
func LivenessProbe(db *supabasev1alpha1.SingleDatabase) *corev1.Probe {
	if db.Spec.Probes != nil && db.Spec.Probes.Liveness != nil {
		return db.Spec.Probes.Liveness
	}
	return DefaultLivenessProbe()
}
