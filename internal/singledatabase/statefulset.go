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
func StatefulSetName(db *supabasev1alpha1.SingleDatabase) string {
	return fmt.Sprintf("%s-db", db.Name)
}

// BuildStatefulSet constructs the StatefulSet for a SingleDatabase.
func BuildStatefulSet(db *supabasev1alpha1.SingleDatabase, secretHash, configMapHash string) *appsv1.StatefulSet {
	replicas := DefaultReplicas

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StatefulSetName(db),
			Namespace: db.Namespace,
			Labels:    DefaultLabels(db.Name),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			Selector:    &metav1.LabelSelector{MatchLabels: DefaultLabels(db.Name)},
			ServiceName: ServiceName(db),
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
	podSpec := corev1.PodSpec{
		InitContainers: []corev1.Container{PasswordSyncInitContainer(db)},
		Containers:     []corev1.Container{MainContainer(db)},
		Volumes: []corev1.Volume{
			{
				Name: DefaultVolumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: PVCName(db),
					},
				},
			},
		},
		NodeSelector:                  db.Spec.NodeSelector,
		Tolerations:                   db.Spec.Tolerations,
		Affinity:                      db.Spec.Affinity,
		TerminationGracePeriodSeconds: db.Spec.TerminationGracePeriodSeconds,
		SecurityContext:               db.Spec.SecurityContext,
	}
	if db.Spec.PriorityClassName != nil {
		podSpec.PriorityClassName = *db.Spec.PriorityClassName
	}
	return podSpec
}

// MainContainer constructs the main PostgreSQL container for a SingleDatabase.
func MainContainer(db *supabasev1alpha1.SingleDatabase) corev1.Container {
	secretName := SecretName(db)
	configMapName := ConfigMapName(db)

	container := corev1.Container{
		Name:  DefaultComponent,
		Image: ResolveImage(db),
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
		StartupProbe:   DefaultStartupProbe(),
		ReadinessProbe: DefaultReadinessProbe(),
		LivenessProbe:  DefaultLivenessProbe(),
		VolumeMounts:   []corev1.VolumeMount{{Name: DefaultVolumeName, MountPath: DefaultDataMountPath}},
	}
	if db.Spec.ImagePullPolicy != nil {
		container.ImagePullPolicy = *db.Spec.ImagePullPolicy
	}
	if db.Spec.Resources != nil {
		container.Resources = *db.Spec.Resources
	}

	return container
}

// PasswordSyncInitContainer constructs the init container that synchronizes
// the PostgreSQL password on disk with the Secret value.
func PasswordSyncInitContainer(db *supabasev1alpha1.SingleDatabase) corev1.Container {
	container := corev1.Container{
		Name:    DefaultInitContainerName,
		Image:   ResolveImage(db),
		Command: []string{"sh", "-c", assets.SingleDatabasePasswordSyncScript},
		Env: []corev1.EnvVar{
			helper.EnvVarFromSecret("PGPASSWORD", SecretName(db), DefaultSecretPasswordKey),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: DefaultVolumeName, MountPath: DefaultDataMountPath},
		},
	}
	if db.Spec.ImagePullPolicy != nil {
		container.ImagePullPolicy = *db.Spec.ImagePullPolicy
	}
	return container
}
