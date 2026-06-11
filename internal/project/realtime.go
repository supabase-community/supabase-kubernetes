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
	"context"
	"fmt"
	"maps"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// EnsureRealtime reconciles the Realtime Deployment and Service for a Project.
func (r *Reconciler) EnsureRealtime(ctx context.Context, project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	logger := log.FromContext(ctx)
	rt := project.Spec.Realtime
	if rt == nil {
		return nil
	}

	image := r.resolveRealtimeImage(project)

	if err := r.ensureRealtimeService(ctx, project); err != nil {
		logger.Error(err, "Failed to ensure Realtime Service")
		r.setCondition(project, ConditionTypeRealtimeReady, metav1.ConditionFalse, "ServiceFailed", err.Error())
		return err
	}

	if err := r.ensureRealtimeDeployment(ctx, project, db, image); err != nil {
		logger.Error(err, "Failed to ensure Realtime Deployment")
		r.setCondition(project, ConditionTypeRealtimeReady, metav1.ConditionFalse, "DeploymentFailed", err.Error())
		return err
	}

	r.setCondition(project, ConditionTypeRealtimeReady, metav1.ConditionTrue, "ReconcileSucceeded",
		"Realtime deployment reconciled successfully")
	return nil
}

func (r *Reconciler) resolveRealtimeImage(project *supabasev1alpha1.Project) string {
	if project.Spec.Realtime.Image != nil && *project.Spec.Realtime.Image != "" {
		return *project.Spec.Realtime.Image
	}
	return DefaultRealtimeImage
}

func realtimeResourceName(project *supabasev1alpha1.Project) string {
	return project.Name + "-realtime"
}

func (r *Reconciler) ensureRealtimeService(ctx context.Context, project *supabasev1alpha1.Project) error {
	logger := log.FromContext(ctx).WithValues("service", realtimeResourceName(project))
	rt := project.Spec.Realtime

	svcSpec := rt.Service
	if svcSpec == nil {
		svcSpec = &supabasev1alpha1.ServiceSpec{}
	}

	svcType := corev1.ServiceTypeClusterIP
	if svcSpec.Type != nil && *svcSpec.Type != "" {
		svcType = *svcSpec.Type
	}

	port := int32(4000)

	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        realtimeResourceName(project),
			Namespace:   project.Namespace,
			Labels:      r.labelsForRealtime(project),
			Annotations: maps.Clone(svcSpec.Annotations),
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: r.selectorLabelsForRealtime(project),
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
	err := r.Client.Get(ctx, namespacedName(desired.Name, desired.Namespace), existing)
	if err != nil {
		if !clientIsNotFound(err) {
			return fmt.Errorf("getting service: %w", err)
		}
		logger.Info("Creating Service")
		if err := r.Client.Create(ctx, desired); err != nil {
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

	if err := r.Client.Update(ctx, existing); err != nil {
		return fmt.Errorf("updating service: %w", err)
	}
	logger.V(1).Info("Updated Service")
	return nil
}

func (r *Reconciler) ensureRealtimeDeployment(ctx context.Context, project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase, image string) error {
	logger := log.FromContext(ctx).WithValues("deployment", realtimeResourceName(project))
	rt := project.Spec.Realtime

	replicas := int32(1)
	if rt.Replicas != nil {
		replicas = *rt.Replicas
	}

	labels := r.labelsForRealtime(project)
	selectorLabels := r.selectorLabelsForRealtime(project)

	podLabels := maps.Clone(selectorLabels)
	maps.Copy(podLabels, rt.PodLabels)

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      realtimeResourceName(project),
			Namespace: project.Namespace,
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
					Annotations: rt.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					Affinity:        rt.Affinity,
					NodeSelector:    rt.NodeSelector,
					Tolerations:     rt.Tolerations,
					SecurityContext: rt.SecurityContext,
					Containers: []corev1.Container{
						r.buildRealtimeContainer(project, db, image),
					},
				},
			},
		},
	}
	if rt.PriorityClassName != nil {
		desired.Spec.Template.Spec.PriorityClassName = *rt.PriorityClassName
	}

	if rt.TerminationGracePeriodSeconds != nil {
		desired.Spec.Template.Spec.TerminationGracePeriodSeconds = rt.TerminationGracePeriodSeconds
	}

	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on deployment: %w", err)
	}

	existing := &appsv1.Deployment{}
	err := r.Client.Get(ctx, namespacedName(desired.Name, desired.Namespace), existing)
	if err != nil {
		if !clientIsNotFound(err) {
			return fmt.Errorf("getting deployment: %w", err)
		}
		logger.Info("Creating Deployment")
		if err := r.Client.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating deployment: %w", err)
		}
		logger.Info("Created Deployment")
		return nil
	}

	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Selector = desired.Spec.Selector
	existing.Spec.Template = desired.Spec.Template
	existing.Labels = desired.Labels

	if err := r.Client.Update(ctx, existing); err != nil {
		return fmt.Errorf("updating deployment: %w", err)
	}
	logger.V(1).Info("Updated Deployment")
	return nil
}

func (r *Reconciler) buildRealtimeContainer(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase, image string) corev1.Container {
	rt := project.Spec.Realtime
	resolved := db
	if resolved == nil {
		resolved = &supabasev1alpha1.ResolvedDatabase{}
	}

	projectJWTSecret := fmt.Sprintf("%s-jwt", project.Name)
	projectKeysSecret := fmt.Sprintf("%s-keys", project.Name)

	container := corev1.Container{
		Name:  "realtime",
		Image: image,
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
	}
	if rt.ImagePullPolicy != nil {
		container.ImagePullPolicy = *rt.ImagePullPolicy
	}
	if rt.Resources != nil {
		container.Resources = *rt.Resources
	}

	return container
}

func (r *Reconciler) labelsForRealtime(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "realtime",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/component":  "realtime",
		"app.kubernetes.io/managed-by": "supabase-operator",
		"app.kubernetes.io/part-of":    project.Name,
	}
}

func (r *Reconciler) selectorLabelsForRealtime(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "realtime",
		"app.kubernetes.io/instance": project.Name,
	}
}
