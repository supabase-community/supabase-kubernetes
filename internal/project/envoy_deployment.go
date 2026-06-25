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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// EnvoyDeploymentName returns the name of the Envoy Deployment for a Project.
func EnvoyDeploymentName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-envoy", project.Name)
}

// EnvoyDeployment constructs the Envoy Deployment for a Project.
func EnvoyDeployment(project *supabasev1alpha1.Project, configMapHash, secretHash string) (*appsv1.Deployment, error) {
	if project.Spec.Envoy == nil || !*project.Spec.Envoy.Enable {
		return nil, nil
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      EnvoyDeploymentName(project),
			Namespace: project.Namespace,
			Labels:    EnvoyLabels(project),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: envoyReplicas(project),
			Selector: &metav1.LabelSelector{
				MatchLabels: EnvoySelectorLabels(project),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      envoyPodLabels(project),
					Annotations: envoyPodAnnotations(project, configMapHash, secretHash),
				},
				Spec: corev1.PodSpec{
					Affinity:                      project.Spec.Envoy.Affinity,
					NodeSelector:                  project.Spec.Envoy.NodeSelector,
					Tolerations:                   project.Spec.Envoy.Tolerations,
					PriorityClassName:             ptr.Deref(project.Spec.Envoy.PriorityClassName, ""),
					SecurityContext:               project.Spec.Envoy.SecurityContext,
					TerminationGracePeriodSeconds: project.Spec.Envoy.TerminationGracePeriodSeconds,
					InitContainers: []corev1.Container{
						buildEnvoyInitContainer(project),
					},
					Containers: []corev1.Container{
						buildEnvoyContainer(project),
					},
					Volumes: []corev1.Volume{
						buildEnvoyConfigVolume(project),
						buildEnvoyRuntimeVolume(),
					},
				},
			},
		},
	}, nil
}

// envoyReplicas returns the number of Envoy replicas from the spec or the default.
func envoyReplicas(project *supabasev1alpha1.Project) *int32 {
	if project.Spec.Envoy != nil && project.Spec.Envoy.Replicas != nil {
		return project.Spec.Envoy.Replicas
	}
	return ptr.To(int32(1))
}

// envoyPodLabels returns the merged pod labels for the Envoy component.
func envoyPodLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(EnvoyLabels(project))
	maps.Copy(labels, project.Spec.Envoy.PodLabels)
	return labels
}

// envoyPodAnnotations returns the merged pod annotations for the Envoy component,
// including hashes that force a rolling update when the ConfigMap or Secret changes.
func envoyPodAnnotations(project *supabasev1alpha1.Project, configMapHash, secretHash string) map[string]string {
	annotations := make(map[string]string, len(project.Spec.Envoy.PodAnnotations)+2)
	annotations["supabase.io/config-hash"] = configMapHash
	annotations["supabase.io/secret-hash"] = secretHash
	maps.Copy(annotations, project.Spec.Envoy.PodAnnotations)
	return annotations
}

// buildEnvoyInitContainer returns the Envoy init container specification.
func buildEnvoyInitContainer(project *supabasev1alpha1.Project) corev1.Container {
	return corev1.Container{
		Name:            "envoy-init",
		Image:           envoyImage(project),
		ImagePullPolicy: envoyImagePullPolicy(project),
		Command:         []string{"/bin/sh", "-c"},
		Args:            []string{assets.ProjectEnvoyInitContainer},
		Env:             buildEnvoyEnvVars(project),
		VolumeMounts:    envoyInitVolumeMounts(),
	}
}

// buildEnvoyContainer returns the Envoy container specification.
func buildEnvoyContainer(project *supabasev1alpha1.Project) corev1.Container {
	return corev1.Container{
		Name:            "envoy",
		Image:           envoyImage(project),
		ImagePullPolicy: envoyImagePullPolicy(project),
		Args:            []string{"-c", fmt.Sprintf("%s/envoy.yaml", EnvoyConfigMountPath)},
		Env:             buildEnvoyEnvVars(project),
		Ports:           envoyPorts(),
		Resources:       ptr.Deref(project.Spec.Envoy.Resources, corev1.ResourceRequirements{}),
		VolumeMounts:    envoyRuntimeVolumeMounts(),
		LivenessProbe:   envoyLivenessProbe(),
		ReadinessProbe:  envoyReadinessProbe(),
	}
}

// envoyImage returns the Envoy image from the spec or the default image.
func envoyImage(project *supabasev1alpha1.Project) string {
	if project.Spec.Envoy.Image != nil && *project.Spec.Envoy.Image != "" {
		return *project.Spec.Envoy.Image
	}
	return DefaultEnvoyImage
}

// envoyImagePullPolicy returns the Envoy image pull policy from the spec or the default.
func envoyImagePullPolicy(project *supabasev1alpha1.Project) corev1.PullPolicy {
	if project.Spec.Envoy.ImagePullPolicy != nil {
		return *project.Spec.Envoy.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

// envoyPorts returns the container ports for the Envoy container.
func envoyPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "envoy",
			ContainerPort: DefaultEnvoyPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

// buildEnvoyEnvVars returns the environment variables for the Envoy containers.
func buildEnvoyEnvVars(project *supabasev1alpha1.Project) []corev1.EnvVar {
	jwtSecret := JWTSecretName(project)
	envoySecret := EnvoySecretName(project)

	return []corev1.EnvVar{
		helper.EnvVarFromSecret("ANON_KEY", jwtSecret, JWTSecretAnonKey),
		helper.EnvVarFromSecret("SERVICE_ROLE_KEY", jwtSecret, JWTSecretServiceKey),
		helper.EnvVarFromSecret("SUPABASE_PUBLISHABLE_KEY", jwtSecret, JWTSecretPublishableKey),
		helper.EnvVarFromSecret("SUPABASE_SECRET_KEY", jwtSecret, JWTSecretOpaqueKey),
		helper.EnvVarFromSecret("ANON_KEY_ASYMMETRIC", jwtSecret, JWTSecretAnonKeyAsym),
		helper.EnvVarFromSecret("SERVICE_ROLE_KEY_ASYMMETRIC", jwtSecret, JWTSecretServiceKeyAsym),
		helper.EnvVarFromSecret("DASHBOARD_USERNAME", envoySecret, DefaultEnvoySecretKeyUsername),
		helper.EnvVarFromSecret("DASHBOARD_PASSWORD", envoySecret, DefaultEnvoySecretKeyPassword),
	}
}

// envoyLivenessProbe returns the liveness probe for the Envoy container.
func envoyLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        envoyProbeHandler(),
		InitialDelaySeconds: 10,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// envoyReadinessProbe returns the readiness probe for the Envoy container.
func envoyReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        envoyProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// envoyProbeHandler returns the shared probe handler for Envoy health checks.
func envoyProbeHandler() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"sh",
				"-c",
				"timeout 1 bash -c '</dev/tcp/127.0.0.1/8000'",
			},
		},
	}
}

// buildEnvoyConfigVolume returns the ConfigMap volume that holds the Envoy templates.
func buildEnvoyConfigVolume(project *supabasev1alpha1.Project) corev1.Volume {
	return corev1.Volume{
		Name: "envoy-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: EnvoyConfigMapName(project),
				},
			},
		},
	}
}

// buildEnvoyRuntimeVolume returns the emptyDir volume used for the generated Envoy configuration.
func buildEnvoyRuntimeVolume() corev1.Volume {
	return corev1.Volume{
		Name: "envoy-runtime",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

// envoyInitVolumeMounts returns the volume mounts for the Envoy init container.
func envoyInitVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "envoy-config",
			MountPath: EnvoyConfigSourcePath,
		},
		{
			Name:      "envoy-runtime",
			MountPath: EnvoyConfigMountPath,
		},
	}
}

// envoyRuntimeVolumeMounts returns the volume mounts for the Envoy container.
func envoyRuntimeVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "envoy-runtime",
			MountPath: EnvoyConfigMountPath,
		},
	}
}
