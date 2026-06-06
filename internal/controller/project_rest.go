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

func (r *ProjectReconciler) ensureRest(ctx context.Context, project *supabasev1alpha1.Project) error {
	logger := log.FromContext(ctx)
	ref := project.Spec.RestRef
	if ref == nil {
		return nil
	}

	rest := &supabasev1alpha1.Rest{}
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: project.Namespace}, rest); err != nil {
		if apierrors.IsNotFound(err) {
			r.setCondition(project, ConditionTypeRestReady, metav1.ConditionFalse, "ComponentNotFound",
				fmt.Sprintf("Rest %q not found", ref.Name))
			logger.Error(err, "Rest resource not found", "rest", ref.Name)
			return fmt.Errorf("rest %q not found", ref.Name)
		}
		logger.Error(err, "Failed to get Rest", "rest", ref.Name)
		return err
	}

	image, err := r.resolveRestImage(rest, project)
	if err != nil {
		r.setCondition(project, ConditionTypeRestReady, metav1.ConditionFalse, "VersionNotSupported", err.Error())
		return err
	}

	if err := r.ensureRestService(ctx, project, rest); err != nil {
		logger.Error(err, "Failed to ensure Rest Service")
		r.setCondition(project, ConditionTypeRestReady, metav1.ConditionFalse, "ServiceFailed", err.Error())
		return err
	}

	if err := r.ensureRestDeployment(ctx, project, rest, image); err != nil {
		logger.Error(err, "Failed to ensure Rest Deployment")
		r.setCondition(project, ConditionTypeRestReady, metav1.ConditionFalse, "DeploymentFailed", err.Error())
		return err
	}

	r.setCondition(project, ConditionTypeRestReady, metav1.ConditionTrue, "ReconcileSucceeded",
		"Rest deployment reconciled successfully")
	return nil
}

func (r *ProjectReconciler) resolveRestImage(rest *supabasev1alpha1.Rest, project *supabasev1alpha1.Project) (string, error) {
	if rest.Spec.Image != "" {
		return rest.Spec.Image, nil
	}
	return images.Resolve(project.Spec.Version, images.ComponentRest)
}

func restResourceName(rest *supabasev1alpha1.Rest) string {
	return rest.Name + "-rest"
}

func (r *ProjectReconciler) ensureRestService(ctx context.Context, project *supabasev1alpha1.Project, rest *supabasev1alpha1.Rest) error {
	logger := log.FromContext(ctx).WithValues("service", restResourceName(rest))

	svcSpec := rest.Spec.Service
	if svcSpec == nil {
		svcSpec = &supabasev1alpha1.ServiceSpec{}
	}

	svcType := corev1.ServiceTypeClusterIP
	if svcSpec.Type != "" {
		svcType = svcSpec.Type
	}

	port := int32(3000)

	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        restResourceName(rest),
			Namespace:   rest.Namespace,
			Labels:      r.labelsForRest(rest, project),
			Annotations: maps.Clone(svcSpec.Annotations),
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: r.selectorLabelsForRest(rest),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.FromInt32(3000),
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

func (r *ProjectReconciler) ensureRestDeployment(ctx context.Context, project *supabasev1alpha1.Project, rest *supabasev1alpha1.Rest, image string) error {
	logger := log.FromContext(ctx).WithValues("deployment", restResourceName(rest))

	replicas := int32(1)
	if rest.Spec.Replicas != nil {
		replicas = *rest.Spec.Replicas
	}

	labels := r.labelsForRest(rest, project)
	selectorLabels := r.selectorLabelsForRest(rest)

	podLabels := maps.Clone(selectorLabels)
	maps.Copy(podLabels, rest.Spec.PodLabels)

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restResourceName(rest),
			Namespace: rest.Namespace,
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
					Annotations: rest.Spec.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					Affinity:          rest.Spec.Affinity,
					NodeSelector:      rest.Spec.NodeSelector,
					Tolerations:       rest.Spec.Tolerations,
					PriorityClassName: rest.Spec.PriorityClassName,
					SecurityContext:   rest.Spec.PodSecurityContext,
					Containers: []corev1.Container{
						r.buildRestContainer(rest, project, image),
					},
				},
			},
		},
	}

	if rest.Spec.TerminationGracePeriodSeconds != nil {
		desired.Spec.Template.Spec.TerminationGracePeriodSeconds = rest.Spec.TerminationGracePeriodSeconds
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

func (r *ProjectReconciler) buildRestContainer(rest *supabasev1alpha1.Rest, project *supabasev1alpha1.Project, image string) corev1.Container {
	resolved := project.Status.ResolvedDatabase
	if resolved == nil {
		resolved = &supabasev1alpha1.ResolvedDatabaseStatus{}
	}

	dbSchemas := rest.Spec.DBSchemas
	if dbSchemas == "" {
		dbSchemas = "public,storage,graphql_public"
	}

	dbMaxRows := int32(1000)
	if rest.Spec.DBMaxRows != nil {
		dbMaxRows = *rest.Spec.DBMaxRows
	}

	dbExtraSearchPath := rest.Spec.DBExtraSearchPath
	if dbExtraSearchPath == "" {
		dbExtraSearchPath = "public"
	}

	jwtExpiry := "3600"
	if project.Spec.JWTExpirySeconds != nil {
		jwtExpiry = strconv.Itoa(int(*project.Spec.JWTExpirySeconds))
	}

	projectJWTSecret := fmt.Sprintf("%s-jwt", project.Name)

	container := corev1.Container{
		Name:            "rest",
		Image:           image,
		ImagePullPolicy: rest.Spec.ImagePullPolicy,
		Command:         []string{"postgrest"},
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: 3000,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			helper.EnvVarFromSecret("PGRST_DB_PASSWORD", resolved.PasswordRef.Name, resolved.PasswordRef.Key),
			helper.EnvVarFromSecret("PGRST_JWT_SECRET", projectJWTSecret, "jwt-jwks"),
			helper.EnvVarFromSecret("PGRST_APP_SETTINGS_JWT_SECRET", projectJWTSecret, "jwt-secret"),
			helper.EnvVar("PGRST_DB_URI", fmt.Sprintf("postgres://authenticator:%s@%s:%d/%s",
				"$(PGRST_DB_PASSWORD)",
				resolved.Host,
				resolved.Port,
				resolved.DBName,
			)),
			helper.EnvVar("PGRST_DB_SCHEMAS", dbSchemas),
			helper.EnvVar("PGRST_DB_MAX_ROWS", strconv.Itoa(int(dbMaxRows))),
			helper.EnvVar("PGRST_DB_EXTRA_SEARCH_PATH", dbExtraSearchPath),
			helper.EnvVar("PGRST_DB_ANON_ROLE", "anon"),
			helper.EnvVar("PGRST_DB_USE_LEGACY_GUCS", "false"),
			helper.EnvVar("PGRST_APP_SETTINGS_JWT_EXP", jwtExpiry),
		},
		Resources:       rest.Spec.Resources,
		SecurityContext: rest.Spec.ContainerSecurityContext,
	}

	container.Env = append(container.Env, rest.Spec.Env...)
	if rest.Spec.Probes != nil {
		if rest.Spec.Probes.Startup != nil {
			container.StartupProbe = rest.Spec.Probes.Startup
		}
		if rest.Spec.Probes.Readiness != nil {
			container.ReadinessProbe = rest.Spec.Probes.Readiness
		}
		if rest.Spec.Probes.Liveness != nil {
			container.LivenessProbe = rest.Spec.Probes.Liveness
		}
	}

	return container
}

func (r *ProjectReconciler) labelsForRest(rest *supabasev1alpha1.Rest, project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "rest",
		"app.kubernetes.io/instance":   rest.Name,
		"app.kubernetes.io/component":  "rest",
		"app.kubernetes.io/managed-by": "supabase-operator",
		"app.kubernetes.io/part-of":    project.Name,
	}
}

func (r *ProjectReconciler) selectorLabelsForRest(rest *supabasev1alpha1.Rest) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "rest",
		"app.kubernetes.io/instance": rest.Name,
	}
}
