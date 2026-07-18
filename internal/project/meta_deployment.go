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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// MetaDeploymentName returns the name of the Meta Deployment for a Project.
func MetaDeploymentName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-meta", project.Name)
}

// MetaDeployment constructs the Meta Deployment for a Project.
func MetaDeployment(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) (*appsv1.Deployment, error) {
	if project.Spec.Meta == nil || !*project.Spec.Meta.Enable {
		return nil, nil
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MetaDeploymentName(project),
			Namespace: project.Namespace,
			Labels:    MetaLabels(project),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: metaReplicas(project),
			Selector: &metav1.LabelSelector{
				MatchLabels: MetaSelectorLabels(project),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      metaPodLabels(project),
					Annotations: metaPodAnnotations(project),
				},
				Spec: corev1.PodSpec{
					Affinity:                      project.Spec.Meta.Affinity,
					NodeSelector:                  project.Spec.Meta.NodeSelector,
					Tolerations:                   project.Spec.Meta.Tolerations,
					PriorityClassName:             ptr.Deref(project.Spec.Meta.PriorityClassName, ""),
					SecurityContext:               project.Spec.Meta.SecurityContext,
					TerminationGracePeriodSeconds: project.Spec.Meta.TerminationGracePeriodSeconds,
					Containers: []corev1.Container{
						buildMetaContainer(project, db),
					},
				},
			},
		},
	}

	return deploy, nil
}

// metaReplicas returns the number of Meta replicas from the spec or the default.
func metaReplicas(project *supabasev1alpha1.Project) *int32 {
	if project.Spec.Meta != nil && project.Spec.Meta.Replicas != nil {
		return project.Spec.Meta.Replicas
	}
	return ptr.To(int32(1))
}

// metaPodLabels returns the merged pod labels for the Meta component.
func metaPodLabels(project *supabasev1alpha1.Project) map[string]string {
	labels := maps.Clone(MetaLabels(project))
	maps.Copy(labels, project.Spec.Meta.PodLabels)
	return labels
}

// metaPodAnnotations returns the merged pod annotations for the Meta component.
func metaPodAnnotations(project *supabasev1alpha1.Project) map[string]string {
	return project.Spec.Meta.PodAnnotations
}

// buildMetaContainer returns the Meta container specification.
func buildMetaContainer(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	return corev1.Container{
		Name:            "meta",
		Image:           metaImage(project),
		ImagePullPolicy: metaImagePullPolicy(project),
		Env:             buildMetaEnvVars(project, db),
		Ports:           metaPorts(),
		Resources:       ptr.Deref(project.Spec.Meta.Resources, corev1.ResourceRequirements{}),
		LivenessProbe:   metaLivenessProbe(),
		ReadinessProbe:  metaReadinessProbe(),
		StartupProbe:    metaStartupProbe(),
	}
}

// metaImage returns the Meta image from the spec or the default image.
func metaImage(project *supabasev1alpha1.Project) string {
	if project.Spec.Meta.Image != nil && *project.Spec.Meta.Image != "" {
		return *project.Spec.Meta.Image
	}
	return DefaultMetaImage
}

// metaImagePullPolicy returns the Meta image pull policy from the spec or the default.
func metaImagePullPolicy(project *supabasev1alpha1.Project) corev1.PullPolicy {
	if project.Spec.Meta.ImagePullPolicy != nil {
		return *project.Spec.Meta.ImagePullPolicy
	}
	return corev1.PullIfNotPresent
}

// metaPorts returns the container ports for the Meta container.
func metaPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "meta",
			ContainerPort: DefaultMetaPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

// metaLivenessProbe returns the liveness probe for the Meta container.
func metaLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        metaProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// metaReadinessProbe returns the readiness probe for the Meta container.
func metaReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        metaProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// metaStartupProbe returns the startup probe for the Meta container.
func metaStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        metaProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    10,
	}
}

// metaProbeHandler returns the shared probe handler for Meta health checks.
func metaProbeHandler() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path:   "/health",
			Port:   intstr.FromInt(int(DefaultMetaPort)),
			Scheme: corev1.URISchemeHTTP,
		},
	}
}

// buildMetaEnvVars returns the environment variables for the Meta container.
func buildMetaEnvVars(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) []corev1.EnvVar {
	return []corev1.EnvVar{
		helper.EnvVar("PG_META_PORT", strconv.Itoa(int(DefaultMetaPort))),
		helper.EnvVar("PG_META_DB_HOST", db.Host),
		helper.EnvVar("PG_META_DB_PORT", strconv.Itoa(int(db.Port))),
		helper.EnvVar("PG_META_DB_NAME", db.DBName),
		helper.EnvVar("PG_META_DB_USER", "postgres"),
		helper.EnvVarFromSecret("PG_META_DB_PASSWORD", db.PasswordRef.Name, db.PasswordRef.Key),
		helper.EnvVarFromSecret("CRYPTO_KEY", KeysSecretName(project), KeysSecretCryptoKey),
	}
}
