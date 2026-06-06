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

package controller

import (
	"context"
	"fmt"
	"maps"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	"github.com/supabase-community/supabase-kubernetes/internal/images"
)

func (r *ProjectReconciler) ensureRealtime(ctx context.Context, project *supabasev1alpha1.Project) error {
	logger := log.FromContext(ctx)
	ref := project.Spec.RealtimeRef
	if ref == nil {
		return nil
	}

	rt := &supabasev1alpha1.Realtime{}
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: project.Namespace}, rt); err != nil {
		if apierrors.IsNotFound(err) {
			r.setCondition(project, ConditionTypeRealtimeReady, metav1.ConditionFalse, "ComponentNotFound",
				fmt.Sprintf("Realtime %q not found", ref.Name))
			logger.Error(err, "Realtime resource not found", "realtime", ref.Name)
			return fmt.Errorf("realtime %q not found", ref.Name)
		}
		logger.Error(err, "Failed to get Realtime", "realtime", ref.Name)
		return err
	}

	image, err := r.resolveRealtimeImage(rt, project)
	if err != nil {
		r.setCondition(project, ConditionTypeRealtimeReady, metav1.ConditionFalse, "VersionNotSupported", err.Error())
		return err
	}

	if err := r.ensureRealtimeService(ctx, project, rt); err != nil {
		logger.Error(err, "Failed to ensure Realtime Service")
		r.setCondition(project, ConditionTypeRealtimeReady, metav1.ConditionFalse, "ServiceFailed", err.Error())
		return err
	}

	if err := r.ensureRealtimeDeployment(ctx, project, rt, image); err != nil {
		logger.Error(err, "Failed to ensure Realtime Deployment")
		r.setCondition(project, ConditionTypeRealtimeReady, metav1.ConditionFalse, "DeploymentFailed", err.Error())
		return err
	}

	r.setCondition(project, ConditionTypeRealtimeReady, metav1.ConditionTrue, "ReconcileSucceeded",
		"Realtime deployment reconciled successfully")
	return nil
}

func (r *ProjectReconciler) resolveRealtimeImage(rt *supabasev1alpha1.Realtime, project *supabasev1alpha1.Project) (string, error) {
	if rt.Spec.Image != "" {
		return rt.Spec.Image, nil
	}
	return images.Resolve(project.Spec.Version, images.ComponentRealtime)
}

func realtimeResourceName(rt *supabasev1alpha1.Realtime) string {
	return rt.Name + "-realtime"
}

func (r *ProjectReconciler) ensureRealtimeService(ctx context.Context, project *supabasev1alpha1.Project, rt *supabasev1alpha1.Realtime) error {
	logger := log.FromContext(ctx).WithValues("service", realtimeResourceName(rt))

	svcSpec := rt.Spec.Service
	if svcSpec == nil {
		svcSpec = &supabasev1alpha1.ServiceSpec{}
	}

	svcType := corev1.ServiceTypeClusterIP
	if svcSpec.Type != "" {
		svcType = svcSpec.Type
	}

	port := int32(4000)

	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        realtimeResourceName(rt),
			Namespace:   rt.Namespace,
			Labels:      r.labelsForRealtime(rt, project),
			Annotations: maps.Clone(svcSpec.Annotations),
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: r.selectorLabelsForRealtime(rt),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.FromInt32(4000),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on service: %w", err)
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting service: %w", err)
		}
		logger.Info("Creating Service")
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating service: %w", err)
		}
		logger.Info("Created Service")
		return nil
	}

	existing.Spec.Type = desired.Spec.Type
	existing.Spec.Selector = desired.Spec.Selector
	existing.Spec.Ports = desired.Spec.Ports
	existing.Annotations = desired.Annotations
	existing.Labels = desired.Labels

	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("updating service: %w", err)
	}
	logger.V(1).Info("Updated Service")
	return nil
}

func (r *ProjectReconciler) ensureRealtimeDeployment(ctx context.Context, project *supabasev1alpha1.Project, rt *supabasev1alpha1.Realtime, image string) error {
	logger := log.FromContext(ctx).WithValues("deployment", realtimeResourceName(rt))

	replicas := int32(1)
	if rt.Spec.Replicas != nil {
		replicas = *rt.Spec.Replicas
	}

	labels := r.labelsForRealtime(rt, project)
	selectorLabels := r.selectorLabelsForRealtime(rt)

	podLabels := maps.Clone(selectorLabels)
	maps.Copy(podLabels, rt.Spec.PodLabels)

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      realtimeResourceName(rt),
			Namespace: rt.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: rt.Spec.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					Affinity:          rt.Spec.Affinity,
					NodeSelector:      rt.Spec.NodeSelector,
					Tolerations:       rt.Spec.Tolerations,
					PriorityClassName: rt.Spec.PriorityClassName,
					SecurityContext:   rt.Spec.PodSecurityContext,
					Containers: []corev1.Container{
						r.buildRealtimeContainer(rt, project, image),
					},
				},
			},
		},
	}

	if rt.Spec.TerminationGracePeriodSeconds != nil {
		desired.Spec.Template.Spec.TerminationGracePeriodSeconds = rt.Spec.TerminationGracePeriodSeconds
	}

	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on deployment: %w", err)
	}

	existing := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting deployment: %w", err)
		}
		logger.Info("Creating Deployment")
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating deployment: %w", err)
		}
		logger.Info("Created Deployment")
		return nil
	}

	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Selector = desired.Spec.Selector
	existing.Spec.Template = desired.Spec.Template
	existing.Labels = desired.Labels

	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("updating deployment: %w", err)
	}
	logger.V(1).Info("Updated Deployment")
	return nil
}

func (r *ProjectReconciler) buildRealtimeContainer(rt *supabasev1alpha1.Realtime, project *supabasev1alpha1.Project, image string) corev1.Container {
	resolved := project.Status.ResolvedDatabase
	if resolved == nil {
		resolved = &supabasev1alpha1.ResolvedDatabaseStatus{}
	}

	projectJWTSecret := fmt.Sprintf("%s-jwt", project.Name)
	projectKeysSecret := fmt.Sprintf("%s-keys", project.Name)

	container := corev1.Container{
		Name:            "realtime",
		Image:           image,
		ImagePullPolicy: rt.Spec.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: 4000,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			helper.EnvVarFromSecret("DB_PASSWORD", resolved.PasswordRef.Name, resolved.PasswordRef.Key),
			helper.EnvVarFromSecret("SECRET_KEY_BASE", projectKeysSecret, "secret-key-base"),
			helper.EnvVarFromSecret("JWT_SECRET", projectJWTSecret, "jwt-secret"),
			helper.EnvVarFromSecret("API_JWT_SECRET", projectJWTSecret, "jwt-secret"),
			helper.EnvVarFromSecret("API_JWT_JWKS", projectJWTSecret, "jwt-jwks"),
			helper.EnvVar("DB_HOST", resolved.Host),
			helper.EnvVar("DB_PORT", strconv.Itoa(int(resolved.Port))),
			helper.EnvVar("DB_USER", "supabase_admin"),
			helper.EnvVar("DB_NAME", resolved.DBName),
			helper.EnvVar("PORT", "4000"),
			helper.EnvVar("DB_AFTER_CONNECT_QUERY", "SET search_path TO _realtime"),
			helper.EnvVar("DB_ENC_KEY", "supabaserealtime"),
			helper.EnvVar("ERL_AFLAGS", "-proto_dist inet_tcp"),
			helper.EnvVar("DNS_NODES", "''"),
			helper.EnvVar("RLIMIT_NOFILE", "10000"),
			helper.EnvVar("APP_NAME", "realtime"),
			helper.EnvVar("SEED_SELF_HOST", "true"),
			helper.EnvVar("RUN_JANITOR", "true"),
			helper.EnvVar("DISABLE_HEALTHCHECK_LOGGING", "true"),
		},
		Resources:       rt.Spec.Resources,
		SecurityContext: rt.Spec.ContainerSecurityContext,
	}
	container.Env = append(container.Env, rt.Spec.Env...)

	if rt.Spec.Probes != nil {
		if rt.Spec.Probes.Startup != nil {
			container.StartupProbe = rt.Spec.Probes.Startup
		}
		if rt.Spec.Probes.Readiness != nil {
			container.ReadinessProbe = rt.Spec.Probes.Readiness
		}
		if rt.Spec.Probes.Liveness != nil {
			container.LivenessProbe = rt.Spec.Probes.Liveness
		}
	}

	return container
}

func (r *ProjectReconciler) labelsForRealtime(rt *supabasev1alpha1.Realtime, project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "realtime",
		"app.kubernetes.io/instance":   rt.Name,
		"app.kubernetes.io/component":  "realtime",
		"app.kubernetes.io/managed-by": "supabase-operator",
		"app.kubernetes.io/part-of":    project.Name,
	}
}

func (r *ProjectReconciler) selectorLabelsForRealtime(rt *supabasev1alpha1.Realtime) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "realtime",
		"app.kubernetes.io/instance": rt.Name,
	}
}
