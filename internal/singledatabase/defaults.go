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
	// DefaultSecretDatabaseKey is the Secret data key for the database name.
	DefaultSecretDatabaseKey = "database"
	// DefaultSecretPasswordKey is the Secret data key for the password.
	DefaultSecretPasswordKey = "password"
	// DefaultVolumeName is the name of the PostgreSQL data volume.
	DefaultVolumeName = "data"
	// DefaultSecretHashAnnotation is the annotation key for the secret hash.
	DefaultSecretHashAnnotation = "supabase.io/secret-hash"
	// DefaultDatabaseUser is the default PostgreSQL user for pg_isready.
	DefaultDatabaseUser = "postgres"
	// DefaultPostgresHost is the default PostgreSQL host.
	DefaultPostgresHost = "/var/run/postgresql"
	// DefaultDataMountPath is the default mount path for PostgreSQL data.
	DefaultDataMountPath = "/var/lib/postgresql/data"
	// DefaultInitContainerName is the default name for the password sync init container.
	DefaultInitContainerName = "password-sync"
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

// DefaultProbes returns the default startup, readiness and liveness probes for the database container.
func DefaultProbes() (*corev1.Probe, *corev1.Probe, *corev1.Probe) {
	pgIsReady := &corev1.ExecAction{
		Command: []string{"pg_isready", "-U", DefaultDatabaseUser},
	}

	startup := &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{Exec: pgIsReady},
		PeriodSeconds:    DefaultStartupProbePeriodSeconds,
		FailureThreshold: DefaultStartupProbeFailureThreshold,
	}
	readiness := &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{Exec: pgIsReady},
		PeriodSeconds:    DefaultReadinessProbePeriodSeconds,
		FailureThreshold: DefaultReadinessProbeFailureThreshold,
	}
	liveness := &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{Exec: pgIsReady},
		PeriodSeconds:    DefaultLivenessProbePeriodSeconds,
		FailureThreshold: DefaultLivenessProbeFailureThreshold,
	}

	return startup, readiness, liveness
}
