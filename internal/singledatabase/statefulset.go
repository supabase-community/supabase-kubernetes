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
func BuildStatefulSet(db *supabasev1alpha1.SingleDatabase, image, secretName, secretHash string) *appsv1.StatefulSet {
	replicas := DefaultReplicas
	labels, annotations := BuildLabelsAndAnnotations(db, secretHash)
	container := BuildMainContainer(db, image, secretName)
	podSpec := BuildPodSpec(db, image, container)

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StatefulSetName(db.Name),
			Namespace: db.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			Selector:    &metav1.LabelSelector{MatchLabels: DefaultLabels(db.Name)},
			ServiceName: ServiceName(db.Name),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: podSpec,
			},
		},
	}
}

func BuildLabelsAndAnnotations(db *supabasev1alpha1.SingleDatabase, secretHash string) (map[string]string, map[string]string) {
	labels := DefaultLabels(db.Name)
	maps.Copy(labels, db.Spec.PodLabels)

	annotations := map[string]string{
		DefaultSecretHashAnnotation: secretHash,
	}
	maps.Copy(annotations, db.Spec.PodAnnotations)

	return labels, annotations
}

func BuildMainContainer(db *supabasev1alpha1.SingleDatabase, image, secretName string) corev1.Container {
	container := corev1.Container{
		Name:            DefaultComponent,
		Image:           image,
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
			helper.EnvVarFromSecret("POSTGRES_DB", secretName, DefaultSecretDatabaseKey),
			helper.EnvVar("POSTGRES_HOST", DefaultPostgresHost),
			helper.EnvVar("POSTGRES_PORT", strconv.Itoa(int(DefaultPort))),
			helper.EnvVarFromSecret("PGPASSWORD", secretName, DefaultSecretPasswordKey),
			helper.EnvVar("PGPORT", strconv.Itoa(int(DefaultPort))),
			helper.EnvVarFromSecret("PGDATABASE", secretName, DefaultSecretDatabaseKey),
			helper.EnvVar("PGHOST", DefaultPostgresHost),
		},
		Resources:    db.Spec.Resources,
		VolumeMounts: []corev1.VolumeMount{{Name: DefaultVolumeName, MountPath: DefaultDataMountPath}},
	}
	container.Env = append(container.Env, db.Spec.Env...)

	if db.Spec.ContainerSecurityContext != nil {
		container.SecurityContext = db.Spec.ContainerSecurityContext
	}

	container.StartupProbe, container.ReadinessProbe, container.LivenessProbe = BuildProbes(db.Spec.Probes)

	return container
}

func BuildPodSpec(db *supabasev1alpha1.SingleDatabase, image string, mainContainer corev1.Container) corev1.PodSpec {
	initContainer := BuildPasswordSyncInitContainer(
		image,
		db.Spec.ImagePullPolicy,
		SecretName(db.Name),
	)

	podSpec := corev1.PodSpec{
		InitContainers: []corev1.Container{initContainer},
		Containers:     []corev1.Container{mainContainer},
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
	}

	if db.Spec.PodSecurityContext != nil {
		podSpec.SecurityContext = db.Spec.PodSecurityContext
	}

	return podSpec
}

// BuildPasswordSyncInitContainer constructs the init container that synchronizes
// the PostgreSQL password on disk with the Secret value.
func BuildPasswordSyncInitContainer(image string, imagePullPolicy corev1.PullPolicy, secretName string) corev1.Container {
	return corev1.Container{
		Name:            DefaultInitContainerName,
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Command:         []string{"sh", "-c", assets.SingleDatabasePasswordSyncScript},
		Env: []corev1.EnvVar{
			helper.EnvVarFromSecret("PGPASSWORD", secretName, DefaultSecretPasswordKey),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: DefaultVolumeName, MountPath: DefaultDataMountPath},
		},
	}
}

// BuildProbes returns the startup, readiness and liveness probes for the database container.
// When probes is nil, default pg_isready probes are returned.
func BuildProbes(probes *supabasev1alpha1.ComponentProbes) (*corev1.Probe, *corev1.Probe, *corev1.Probe) {
	if probes != nil {
		return probes.Startup, probes.Readiness, probes.Liveness
	}

	return DefaultProbes()
}
