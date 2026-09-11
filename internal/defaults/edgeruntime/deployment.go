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

package edgeruntime

import (
	"fmt"
	"strconv"

	"github.com/supabase-community/supabase-kubernetes/internal/defaults"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults/function"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// FunctionsDeploymentName returns the name of the Functions Deployment for a Project.
func FunctionsDeploymentName(project *ResourceContext) string {
	return ComponentName(project.Names["EdgeRuntime"], "edge-runtime")
}

// FunctionsDeployment constructs the Functions Deployment for a Project.
func FunctionsDeployment(project *ResourceContext, db *supabasev1alpha1.ResolvedDatabase) (*appsv1.Deployment, error) {
	if project.Spec.EdgeRuntime == nil {
		return nil, nil
	}

	base := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: FunctionsLabels(project)},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{buildEdgeRuntimeContainer(project, db)}},
	}
	function.AddSync(&base.Spec, project.Name, FunctionsDeploymentName(project), true)
	template, err := helper.Overlay(base, project.Spec.EdgeRuntime.Pod)
	if err != nil {
		return nil, err
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      FunctionsDeploymentName(project),
			Namespace: project.Namespace,
			Labels:    FunctionsLabels(project),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: functionsReplicas(project),
			Selector: &metav1.LabelSelector{
				MatchLabels: FunctionsSelectorLabels(project),
			},
			Template: template,
		},
	}

	return deploy, nil
}

// functionsReplicas returns the number of Functions replicas from the spec or the default.
func functionsReplicas(project *ResourceContext) *int32 {
	if project.Spec.EdgeRuntime != nil && project.Spec.EdgeRuntime.Replicas != nil {
		return project.Spec.EdgeRuntime.Replicas
	}
	return ptr.To(int32(1))
}

// buildEdgeRuntimeContainer returns the EdgeRuntime container specification.
func buildEdgeRuntimeContainer(project *ResourceContext, db *supabasev1alpha1.ResolvedDatabase) corev1.Container {
	return corev1.Container{
		Name:            "edge-runtime",
		Image:           functionsImage(project),
		ImagePullPolicy: functionsImagePullPolicy(project),
		Args:            []string{"start", "--main-service", "/home/deno/functions/main"},
		Env:             buildFunctionsEnvVars(project, db),
		Ports:           functionsPorts(),
		Resources:       corev1.ResourceRequirements{},
		LivenessProbe:   functionsLivenessProbe(),
		ReadinessProbe:  functionsReadinessProbe(),
		StartupProbe:    functionsStartupProbe(),
		VolumeMounts:    []corev1.VolumeMount{{Name: function.SyncVolumeName, MountPath: "/home/deno/functions", ReadOnly: true}},
	}
}

// functionsImage returns the Functions image from the spec or the default image.
func functionsImage(project *ResourceContext) string {
	return defaults.DefaultEdgeRuntimeImage
}

// functionsImagePullPolicy returns the Functions image pull policy from the spec or the default.
func functionsImagePullPolicy(project *ResourceContext) corev1.PullPolicy {
	return corev1.PullIfNotPresent
}

// functionsPorts returns the container ports for the EdgeRuntime container.
func functionsPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "functions",
			ContainerPort: DefaultFunctionsPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

// functionsLivenessProbe returns the liveness probe for the EdgeRuntime container.
func functionsLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        functionsProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// functionsReadinessProbe returns the readiness probe for the EdgeRuntime container.
func functionsReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        functionsProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// functionsStartupProbe returns the startup probe for the EdgeRuntime container.
func functionsStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        functionsProbeHandler(),
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		TimeoutSeconds:      5,
		FailureThreshold:    10,
	}
}

// functionsProbeHandler returns the shared probe handler for Functions health checks.
func functionsProbeHandler() corev1.ProbeHandler {
	return corev1.ProbeHandler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"bash",
				"-c",
				"timeout 1 bash -c '</dev/tcp/127.0.0.1/9000'",
			},
		},
	}
}

// buildFunctionsEnvVars returns the environment variables for the EdgeRuntime container.
func buildFunctionsEnvVars(project *ResourceContext, db *supabasev1alpha1.ResolvedDatabase) []corev1.EnvVar {
	env := []corev1.EnvVar{
		helper.EnvVarFromSecret("JWT_SECRET", JWTSecretName(project), JWTSecretKey),
		helper.EnvVar("SUPABASE_URL", fmt.Sprintf(
			"http://%s.%s.svc.cluster.local:%d",
			EnvoyServiceName(project),
			project.Namespace,
			DefaultEnvoyPort,
		)),
		helper.EnvVar("SUPABASE_PUBLIC_URL", project.Spec.PublicURL),
		helper.EnvVarFromSecret("SUPABASE_ANON_KEY", JWTSecretName(project), JWTSecretAnonKey),
		helper.EnvVarFromSecret("SUPABASE_SERVICE_ROLE_KEY", JWTSecretName(project), JWTSecretServiceKey),
		helper.EnvVarFromSecret("SUPABASE_PUBLISHABLE_KEY", JWTSecretName(project), JWTSecretPublishableKey),
		helper.EnvVarFromSecret("SUPABASE_SECRET_KEY", JWTSecretName(project), JWTSecretOpaqueKey),
		helper.EnvVar("SUPABASE_PUBLISHABLE_KEYS", `{"default":"$(SUPABASE_PUBLISHABLE_KEY)"}`),
		helper.EnvVar("SUPABASE_SECRET_KEYS", `{"default":"$(SUPABASE_SECRET_KEY)"}`),
		helper.EnvVarFromSecret("DB_PASSWORD", db.PasswordRef.Name, db.PasswordRef.Key),
		helper.EnvVar("SUPABASE_DB_URL", fmt.Sprintf(
			"postgresql://postgres:$(DB_PASSWORD)@%s:%s/%s",
			db.Host,
			strconv.Itoa(int(db.Port)),
			db.DBName,
		)),
	}

	return helper.MergeEnvVars(env, project.Spec.EdgeRuntime.Config)
}
