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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
)

// StatefulSetName returns the name of the StatefulSet for a SingleDatabase.
func StatefulSetName(dbName string) string {
	return fmt.Sprintf("%s-db", dbName)
}

// BuildStatefulSet constructs the StatefulSet for a SingleDatabase.
func BuildStatefulSet(db *supabasev1alpha1.SingleDatabase, image, secretName, credentialHash string) *appsv1.StatefulSet {
	replicas := int32(1)

	labels := DefaultLabels(db.Name)
	maps.Copy(labels, db.Spec.PodLabels)

	annotations := map[string]string{
		"supabase.io/secret-hash": credentialHash,
	}
	maps.Copy(annotations, db.Spec.PodAnnotations)

	container := corev1.Container{
		Name:            Component,
		Image:           image,
		ImagePullPolicy: db.Spec.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          "postgres",
				ContainerPort: Port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name: "POSTGRES_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "password",
					},
				},
			},
			{
				Name: "POSTGRES_DB",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "database",
					},
				},
			},
			{Name: "POSTGRES_HOST", Value: "/var/run/postgresql"},
			{Name: "POSTGRES_PORT", Value: "5432"},
			{
				Name: "PGPASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "password",
					},
				},
			},
			{Name: "PGPORT", Value: "5432"},
			{
				Name: "PGDATABASE",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "database",
					},
				},
			},
			{Name: "PGHOST", Value: "/var/run/postgresql"},
		},
		Resources:    db.Spec.Resources,
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/var/lib/postgresql/data"}},
	}
	container.Env = append(container.Env, db.Spec.Env...)

	if db.Spec.ContainerSecurityContext != nil {
		container.SecurityContext = db.Spec.ContainerSecurityContext
	}

	container.StartupProbe, container.ReadinessProbe, container.LivenessProbe = BuildProbes(db.Spec.Probes)

	initContainer := BuildPasswordSyncInitContainer(image, db.Spec.ImagePullPolicy, secretName)

	podSpec := corev1.PodSpec{
		InitContainers: []corev1.Container{initContainer},
		Containers:     []corev1.Container{container},
		Volumes: []corev1.Volume{
			{
				Name: "data",
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

// BuildPasswordSyncInitContainer constructs the init container that synchronizes
// the PostgreSQL password on disk with the Secret value.
func BuildPasswordSyncInitContainer(image string, imagePullPolicy corev1.PullPolicy, secretName string) corev1.Container {
	return corev1.Container{
		Name:            "password-sync",
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Command:         []string{"sh", "-c", assets.SingleDatabasePasswordSyncScript},
		Env: []corev1.EnvVar{
			{
				Name: "PGPASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "password",
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: "/var/lib/postgresql/data"},
		},
	}
}

// BuildProbes returns the startup, readiness and liveness probes for the database container.
// When probes is nil, default pg_isready probes are returned.
func BuildProbes(probes *supabasev1alpha1.ComponentProbes) (*corev1.Probe, *corev1.Probe, *corev1.Probe) {
	if probes != nil {
		return probes.Startup, probes.Readiness, probes.Liveness
	}

	pgIsReady := &corev1.ExecAction{
		Command: []string{"pg_isready", "-U", "postgres"},
	}

	startup := &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{Exec: pgIsReady},
		PeriodSeconds:    10,
		FailureThreshold: 30,
	}
	readiness := &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{Exec: pgIsReady},
		PeriodSeconds:    10,
		FailureThreshold: 3,
	}
	liveness := &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{Exec: pgIsReady},
		PeriodSeconds:    20,
		FailureThreshold: 3,
	}

	return startup, readiness, liveness
}
