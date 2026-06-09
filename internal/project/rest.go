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
	"github.com/supabase-community/supabase-kubernetes/internal/images"
)

// EnsureRest reconciles the Rest Deployment and Service for a Project.
func (r *Reconciler) EnsureRest(ctx context.Context, project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase) error {
	logger := log.FromContext(ctx)
	rest := project.Spec.Rest
	if rest == nil {
		return nil
	}

	image := r.resolveRestImage(project)

	if err := r.ensureRestService(ctx, project); err != nil {
		logger.Error(err, "Failed to ensure Rest Service")
		r.setCondition(project, ConditionTypeRestReady, metav1.ConditionFalse, "ServiceFailed", err.Error())
		return err
	}

	if err := r.ensureRestDeployment(ctx, project, db, image); err != nil {
		logger.Error(err, "Failed to ensure Rest Deployment")
		r.setCondition(project, ConditionTypeRestReady, metav1.ConditionFalse, "DeploymentFailed", err.Error())
		return err
	}

	r.setCondition(project, ConditionTypeRestReady, metav1.ConditionTrue, "ReconcileSucceeded",
		"Rest deployment reconciled successfully")
	return nil
}

func (r *Reconciler) resolveRestImage(project *supabasev1alpha1.Project) string {
	if project.Spec.Rest.Image != "" {
		return project.Spec.Rest.Image
	}
	return images.Resolve(project.Spec.Version, images.ComponentRest)
}

func restResourceName(project *supabasev1alpha1.Project) string {
	return project.Name + "-rest"
}

func (r *Reconciler) ensureRestService(ctx context.Context, project *supabasev1alpha1.Project) error {
	logger := log.FromContext(ctx).WithValues("service", restResourceName(project))
	rest := project.Spec.Rest

	svcSpec := rest.Service
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
			Name:        restResourceName(project),
			Namespace:   project.Namespace,
			Labels:      r.labelsForRest(project),
			Annotations: maps.Clone(svcSpec.Annotations),
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: r.selectorLabelsForRest(project),
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

func (r *Reconciler) ensureRestDeployment(ctx context.Context, project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase, image string) error {
	logger := log.FromContext(ctx).WithValues("deployment", restResourceName(project))
	rest := project.Spec.Rest

	replicas := int32(1)
	if rest.Replicas != nil {
		replicas = *rest.Replicas
	}

	labels := r.labelsForRest(project)
	selectorLabels := r.selectorLabelsForRest(project)

	podLabels := maps.Clone(selectorLabels)
	maps.Copy(podLabels, rest.PodLabels)

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restResourceName(project),
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
					Annotations: rest.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					Affinity:          rest.Affinity,
					NodeSelector:      rest.NodeSelector,
					Tolerations:       rest.Tolerations,
					PriorityClassName: rest.PriorityClassName,
					SecurityContext:   rest.PodSecurityContext,
					Containers: []corev1.Container{
						r.buildRestContainer(project, db, image),
					},
				},
			},
		},
	}

	if rest.TerminationGracePeriodSeconds != nil {
		desired.Spec.Template.Spec.TerminationGracePeriodSeconds = rest.TerminationGracePeriodSeconds
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

func (r *Reconciler) buildRestContainer(project *supabasev1alpha1.Project, db *supabasev1alpha1.ResolvedDatabase, image string) corev1.Container {
	rest := project.Spec.Rest
	resolved := db
	if resolved == nil {
		resolved = &supabasev1alpha1.ResolvedDatabase{}
	}

	dbSchemas := rest.DBSchemas
	if dbSchemas == "" {
		dbSchemas = "public,storage,graphql_public"
	}

	dbMaxRows := int32(1000)
	if rest.DBMaxRows != nil {
		dbMaxRows = *rest.DBMaxRows
	}

	dbExtraSearchPath := rest.DBExtraSearchPath
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
		ImagePullPolicy: rest.ImagePullPolicy,
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
		Resources:       rest.Resources,
		SecurityContext: rest.ContainerSecurityContext,
	}

	container.Env = append(container.Env, rest.Env...)
	if rest.Probes != nil {
		if rest.Probes.Startup != nil {
			container.StartupProbe = rest.Probes.Startup
		}
		if rest.Probes.Readiness != nil {
			container.ReadinessProbe = rest.Probes.Readiness
		}
		if rest.Probes.Liveness != nil {
			container.LivenessProbe = rest.Probes.Liveness
		}
	}

	return container
}

func (r *Reconciler) labelsForRest(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "rest",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/component":  "rest",
		"app.kubernetes.io/managed-by": "supabase-operator",
		"app.kubernetes.io/part-of":    project.Name,
	}
}

func (r *Reconciler) selectorLabelsForRest(project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "rest",
		"app.kubernetes.io/instance": project.Name,
	}
}
