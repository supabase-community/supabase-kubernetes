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

package project

import (
	"fmt"
	"maps"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// StorageStatefulSetName returns the name of the Storage StatefulSet for a Project.
func StorageStatefulSetName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-storage", project.Name)
}

// StorageStatefulSet constructs the StatefulSet for the Storage component.
func StorageStatefulSet(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) (*appsv1.StatefulSet, error) {
	if project.Spec.Storage == nil || !*project.Spec.Storage.Enable {
		return nil, nil
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StorageStatefulSetName(project),
			Namespace: project.Namespace,
			Labels:    StorageLabels(project),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: StorageServiceName(project),
			Replicas:    storageReplicas(project),
			Selector: &metav1.LabelSelector{
				MatchLabels: StorageSelectorLabels(project),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      storagePodLabels(project),
					Annotations: storagePodAnnotations(project),
				},
				Spec: corev1.PodSpec{
					Affinity:                      project.Spec.Storage.Affinity,
					NodeSelector:                  project.Spec.Storage.NodeSelector,
					Tolerations:                   project.Spec.Storage.Tolerations,
					PriorityClassName:             ptr.Deref(project.Spec.Storage.PriorityClassName, ""),
					SecurityContext:               project.Spec.Storage.SecurityContext,
					TerminationGracePeriodSeconds: project.Spec.Storage.TerminationGracePeriodSeconds,
					Containers: []corev1.Container{
						buildStorageContainer(project, db),
					},
					Volumes: []corev1.Volume{
						buildStorageVolume(project),
					},
				},
			},
		},
	}

	return sts, nil
}

// storageReplicas returns the number of Storage replicas from the spec or the default.
func storageReplicas(project *supabasev1alpha1.Project) *int32 {
	if project.Spec.Storage != nil && project.Spec.Storage.Replicas != nil {
		return project.Spec.Storage.Replicas
	}
	return ptr.To(int32(1))
}

// storagePodLabels returns the merged pod labels for the Storage component.
func storagePodLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(StorageLabels(project))
	maps.Copy(labels, project.Spec.Storage.PodLabels)
	return labels
}

// storagePodAnnotations returns the merged pod annotations for the Storage component.
func storagePodAnnotations(project *supabasev1alpha1.Project) map[string]string {
	return project.Spec.Storage.PodAnnotations
}

// buildStorageContainer returns the Storage container specification.
func buildStorageContainer(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	return corev1.Container{
		Name:            "storage",
		Image:           storageImage(project),
		ImagePullPolicy: storageImagePullPolicy(project),
		Env:             buildStorageEnvVars(project, db),
		Ports:           storagePorts(),
		Resources:       ptr.Deref(project.Spec.Storage.Resources, corev1.ResourceRequirements{}),
		LivenessProbe:   storageLivenessProbe(),
		ReadinessProbe:  storageReadinessProbe(),
		StartupProbe:    storageStartupProbe(),
		VolumeMounts:    buildStorageVolumeMounts(),
	}
}

// storageImage returns the Storage image from the spec or the default image.
func storageImage(project *supabasev1alpha1.Project) string {
	if project.Spec.Storage.Image != nil && *project.Spec.Storage.Image != "" {
		return *project.Spec.Storage.Image
	}
	return DefaultStorageImage
}

// storageImagePullPolicy returns the Storage image pull policy from the spec or the default.
func storageImagePullPolicy(project *supabasev1alpha1.Project) corev1.PullPolicy {
	if project.Spec.Storage.ImagePullPolicy != nil {
		return *project.Spec.Storage.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

// storagePorts returns the container ports for the Storage container.
func storagePorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "storage",
			ContainerPort: DefaultStoragePort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

// storageLivenessProbe returns the liveness probe for the Storage container.
func storageLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        storageProbeHandler(),
		InitialDelaySeconds: 10,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// storageReadinessProbe returns the readiness probe for the Storage container.
func storageReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        storageProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// storageStartupProbe returns the startup probe for the Storage container.
func storageStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        storageProbeHandler(),
		InitialDelaySeconds: 10,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    6,
	}
}

// storageProbeHandler returns the shared probe handler for Storage health checks.
func storageProbeHandler() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"wget",
				"--no-verbose",
				"--tries=1",
				"--spider",
				fmt.Sprintf("http://127.0.0.1:%s/status", strconv.Itoa(int(DefaultStoragePort))),
			},
		},
	}
}

// buildStorageEnvVars returns the environment variables for the Storage container.
func buildStorageEnvVars(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) []corev1.EnvVar {
	jwtSecret := JWTSecretName(project)
	storageSecret := StorageSecretName(project)

	return []corev1.EnvVar{
		helper.EnvVarFromSecret("ANON_KEY", jwtSecret, JWTSecretAnonKey),
		helper.EnvVarFromSecret("SERVICE_KEY", jwtSecret, JWTSecretServiceKey),
		helper.EnvVar("POSTGREST_URL", fmt.Sprintf(
			"http://%s.%s.svc.cluster.local:%d",
			RestServiceName(project),
			project.Namespace,
			DefaultRestPort,
		)),
		helper.EnvVarFromSecret("AUTH_JWT_SECRET", jwtSecret, JWTSecretKey),
		helper.EnvVarFromSecret("JWT_JWKS", jwtSecret, JWTSecretJWKS),
		helper.EnvVarFromSecret("DB_PASSWORD", db.PasswordRef.Name, db.PasswordRef.Key),
		helper.EnvVar("DATABASE_URL", fmt.Sprintf(
			"postgres://supabase_storage_admin:$(DB_PASSWORD)@%s:%s/%s",
			db.Host,
			strconv.Itoa(int(db.Port)),
			db.DBName,
		)),
		helper.EnvVar("STORAGE_PUBLIC_URL", APIExternalURL(project)),
		helper.EnvVar("REQUEST_ALLOW_X_FORWARDED_PATH", "true"),
		helper.EnvVar("FILE_SIZE_LIMIT", storageFileSizeLimitOrDefault(project)),
		helper.EnvVar("STORAGE_BACKEND", "file"),
		helper.EnvVar("GLOBAL_S3_BUCKET", project.Name),
		helper.EnvVar("FILE_STORAGE_BACKEND_PATH", StorageDataMountPath),
		helper.EnvVar("TENANT_ID", project.Name),
		helper.EnvVar("REGION", project.Name),
		helper.EnvVar("ENABLE_IMAGE_TRANSFORMATION", "false"),
		helper.EnvVarFromSecret("S3_PROTOCOL_ACCESS_KEY_ID", storageSecret, StorageSecretAccessKeyID),
		helper.EnvVarFromSecret("S3_PROTOCOL_ACCESS_KEY_SECRET", storageSecret, StorageSecretAccessKeySecret),
	}
}

// storageFileSizeLimitOrDefault returns the file size limit from the spec or the default.
func storageFileSizeLimitOrDefault(project *supabasev1alpha1.Project) string {
	if project.Spec.Storage != nil && project.Spec.Storage.FileSizeLimit != nil {
		return strconv.FormatInt(*project.Spec.Storage.FileSizeLimit, 10)
	}
	return "52428800"
}

// buildStorageVolume returns the Storage data volume specification.
func buildStorageVolume(project *supabasev1alpha1.Project) corev1.Volume {
	return corev1.Volume{
		Name: "storage-data",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: StoragePVCName(project),
			},
		},
	}
}

// buildStorageVolumeMounts returns the volume mounts for the Storage container.
func buildStorageVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "storage-data",
			MountPath: StorageDataMountPath,
			SubPath:   StorageDataSubPath,
		},
	}
}
