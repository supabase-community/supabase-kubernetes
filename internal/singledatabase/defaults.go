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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

const (
	// DefaultAppName is the value for the name label.
	DefaultAppName = "supabase"
	// DefaultComponent is the Kubernetes component label value for the database.
	DefaultComponent = "singledatabase"
	// DefaultManagedBy is the value for the managed-by label.
	DefaultManagedBy = "supabase-operator"
	// DefaultPort is the PostgreSQL port.
	DefaultPort = int32(5432)
	// DefaultContainerPortName is the name of the PostgreSQL container port.
	DefaultContainerPortName = "postgres"
	// DefaultDatabase is the default database to use on postgres.
	DefaultDatabase = "postgres"
	// DefaultVolumeName is the name of the PostgreSQL data volume.
	DefaultVolumeName = "data"
	// DefaultSecretHashAnnotation is the annotation key for the secret hash.
	DefaultSecretHashAnnotation = "supabase.io/secret-hash"
	// DefaultConfigMapHashAnnotation is the annotation key for the configmap hash.
	DefaultConfigMapHashAnnotation = "supabase.io/configmap-hash"
	// DefaultDatabaseUser is the default PostgreSQL user.
	DefaultDatabaseUser = "supabase_admin"
	// DefaultPostgresHost is the default PostgreSQL host.
	DefaultPostgresHost = "/var/run/postgresql"
	// DefaultDataMountPath is the default mount path for PostgreSQL data.
	DefaultDataMountPath = "/var/lib/postgresql/data"
	// DefaultInitContainerName is the default name for the password sync init container.
	DefaultInitContainerName = "password-sync"
	// DefaultDatabaseImage is the default container image for SingleDatabase.
	DefaultDatabaseImage = "supabase/postgres:17.6.1.084"
	// DefaultReplicas is the default number of replicas for the StatefulSet.
	DefaultReplicas = int32(1)
	// DefaultServiceType is the default Service type.
	DefaultServiceType = corev1.ServiceTypeClusterIP
	// DefaultAccessMode is the default PVC access mode.
	DefaultAccessMode = corev1.ReadWriteOnce
	// DefaultStorageSize is the default PVC storage size.
	DefaultStorageSize = "10Gi"
	// DefaultStartupProbePeriodSeconds is the default period for the startup probe.
	DefaultStartupProbePeriodSeconds = int32(10)
	// DefaultStartupProbeFailureThreshold is the default failure threshold for the startup probe.
	DefaultStartupProbeFailureThreshold = int32(30)
	// DefaultReadinessProbePeriodSeconds is the default period for the readiness probe.
	DefaultReadinessProbePeriodSeconds = int32(10)
	// DefaultReadinessProbeFailureThreshold is the default failure threshold for the readiness probe.
	DefaultReadinessProbeFailureThreshold = int32(3)
	// DefaultLivenessProbePeriodSeconds is the default period for the liveness probe.
	DefaultLivenessProbePeriodSeconds = int32(20)
	// DefaultLivenessProbeFailureThreshold is the default failure threshold for the liveness probe.
	DefaultLivenessProbeFailureThreshold = int32(3)
	// DefaultSecretPasswordKey is the Secret data key for the password.
	DefaultSecretPasswordKey = "password"
	// DefaultConfigMapKeyPort is the ConfigMap data key for the PostgreSQL port.
	DefaultConfigMapKeyPort = "port"
	// DefaultConfigMapKeyDatabase is the ConfigMap data key for the database name.
	DefaultConfigMapKeyDatabase = "database"
	// DefaultConfigMapKeyUser is the ConfigMap data key for the database user.
	DefaultConfigMapKeyUser = "user"
)

// DefaultLabels returns the standard labels for SingleDatabase resources.
func DefaultLabels(instanceName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       DefaultAppName,
		"app.kubernetes.io/instance":   instanceName,
		"app.kubernetes.io/component":  DefaultComponent,
		"app.kubernetes.io/managed-by": DefaultManagedBy,
	}
}

// DefaultStorageResources returns the default storage resource requirements.
func DefaultStorageResources() corev1.VolumeResourceRequirements {
	return corev1.VolumeResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse(DefaultStorageSize),
		},
	}
}

// DefaultStorageAccessModes returns the default storage access modes.
func DefaultStorageAccessModes() []corev1.PersistentVolumeAccessMode {
	return []corev1.PersistentVolumeAccessMode{DefaultAccessMode}
}

func pgIsReadyProbe() *corev1.ExecAction {
	return &corev1.ExecAction{
		Command: []string{"pg_isready", "-U", DefaultDatabaseUser},
	}
}

// DefaultStartupProbe returns the default startup probe for the database container.
func DefaultStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{Exec: pgIsReadyProbe()},
		PeriodSeconds:    DefaultStartupProbePeriodSeconds,
		FailureThreshold: DefaultStartupProbeFailureThreshold,
	}
}

// DefaultReadinessProbe returns the default readiness probe for the database container.
func DefaultReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{Exec: pgIsReadyProbe()},
		PeriodSeconds:    DefaultReadinessProbePeriodSeconds,
		FailureThreshold: DefaultReadinessProbeFailureThreshold,
	}
}

// DefaultLivenessProbe returns the default liveness probe for the database container.
func DefaultLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{Exec: pgIsReadyProbe()},
		PeriodSeconds:    DefaultLivenessProbePeriodSeconds,
		FailureThreshold: DefaultLivenessProbeFailureThreshold,
	}
}

// ResolveImage returns the container image for a SingleDatabase.
// If db.Spec.Image is set, it returns that value directly.
// Otherwise, it resolves the image from the version/component registry.
func ResolveImage(db *supabasev1alpha1.SingleDatabase) string {
	if db.Spec.Image != "" {
		return db.Spec.Image
	}
	return DefaultDatabaseImage
}
