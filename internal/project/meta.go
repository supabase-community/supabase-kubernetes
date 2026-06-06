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

// EnsureMeta reconciles the Meta Deployment and Service for a Project.
func (r *Reconciler) EnsureMeta(ctx context.Context, project *supabasev1alpha1.Project) error {
	logger := log.FromContext(ctx)
	ref := project.Spec.MetaRef
	if ref == nil {
		return nil
	}

	m := &supabasev1alpha1.Meta{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: project.Namespace}, m); err != nil {
		if apierrors.IsNotFound(err) {
			r.setCondition(project, ConditionTypeMetaReady, metav1.ConditionFalse, "ComponentNotFound",
				fmt.Sprintf("Meta %q not found", ref.Name))
			logger.Error(err, "Meta resource not found", "meta", ref.Name)
			return fmt.Errorf("meta %q not found", ref.Name)
		}
		logger.Error(err, "Failed to get Meta", "meta", ref.Name)
		return err
	}

	image, err := r.resolveMetaImage(m, project)
	if err != nil {
		r.setCondition(project, ConditionTypeMetaReady, metav1.ConditionFalse, "VersionNotSupported", err.Error())
		return err
	}

	if err := r.ensureMetaService(ctx, project, m); err != nil {
		logger.Error(err, "Failed to ensure Meta Service")
		r.setCondition(project, ConditionTypeMetaReady, metav1.ConditionFalse, "ServiceFailed", err.Error())
		return err
	}

	if err := r.ensureMetaDeployment(ctx, project, m, image); err != nil {
		logger.Error(err, "Failed to ensure Meta Deployment")
		r.setCondition(project, ConditionTypeMetaReady, metav1.ConditionFalse, "DeploymentFailed", err.Error())
		return err
	}

	r.setCondition(project, ConditionTypeMetaReady, metav1.ConditionTrue, "ReconcileSucceeded",
		"Meta deployment reconciled successfully")
	return nil
}

func (r *Reconciler) resolveMetaImage(m *supabasev1alpha1.Meta, project *supabasev1alpha1.Project) (string, error) {
	if m.Spec.Image != "" {
		return m.Spec.Image, nil
	}
	return images.Resolve(project.Spec.Version, images.ComponentMeta)
}

func metaResourceName(m *supabasev1alpha1.Meta) string {
	return m.Name + "-meta"
}

func (r *Reconciler) ensureMetaService(ctx context.Context, project *supabasev1alpha1.Project, m *supabasev1alpha1.Meta) error {
	logger := log.FromContext(ctx).WithValues("service", metaResourceName(m))

	svcSpec := m.Spec.Service
	if svcSpec == nil {
		svcSpec = &supabasev1alpha1.ServiceSpec{}
	}

	svcType := corev1.ServiceTypeClusterIP
	if svcSpec.Type != "" {
		svcType = svcSpec.Type
	}

	port := int32(8080)

	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        metaResourceName(m),
			Namespace:   m.Namespace,
			Labels:      r.labelsForMeta(m, project),
			Annotations: maps.Clone(svcSpec.Annotations),
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: r.selectorLabelsForMeta(m),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.FromInt32(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on service: %w", err)
	}

	existing := &corev1.Service{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
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

func (r *Reconciler) ensureMetaDeployment(ctx context.Context, project *supabasev1alpha1.Project, m *supabasev1alpha1.Meta, image string) error {
	logger := log.FromContext(ctx).WithValues("deployment", metaResourceName(m))

	replicas := int32(1)
	if m.Spec.Replicas != nil {
		replicas = *m.Spec.Replicas
	}

	labels := r.labelsForMeta(m, project)
	selectorLabels := r.selectorLabelsForMeta(m)

	podLabels := maps.Clone(selectorLabels)
	maps.Copy(podLabels, m.Spec.PodLabels)

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metaResourceName(m),
			Namespace: m.Namespace,
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
					Annotations: m.Spec.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					Affinity:          m.Spec.Affinity,
					NodeSelector:      m.Spec.NodeSelector,
					Tolerations:       m.Spec.Tolerations,
					PriorityClassName: m.Spec.PriorityClassName,
					SecurityContext:   m.Spec.PodSecurityContext,
					Containers: []corev1.Container{
						r.buildMetaContainer(m, project, image),
					},
				},
			},
		},
	}

	if m.Spec.TerminationGracePeriodSeconds != nil {
		desired.Spec.Template.Spec.TerminationGracePeriodSeconds = m.Spec.TerminationGracePeriodSeconds
	}

	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on deployment: %w", err)
	}

	existing := &appsv1.Deployment{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
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

func (r *Reconciler) buildMetaContainer(m *supabasev1alpha1.Meta, project *supabasev1alpha1.Project, image string) corev1.Container {
	resolved := project.Status.ResolvedDatabase
	if resolved == nil {
		resolved = &supabasev1alpha1.ResolvedDatabaseStatus{}
	}

	projectKeysSecret := fmt.Sprintf("%s-keys", project.Name)

	container := corev1.Container{
		Name:            "meta",
		Image:           image,
		ImagePullPolicy: m.Spec.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: 8080,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			helper.EnvVarFromSecret("PG_META_DB_PASSWORD", resolved.PasswordRef.Name, resolved.PasswordRef.Key),
			helper.EnvVarFromSecret("CRYPTO_KEY", projectKeysSecret, "crypto-key"),
			helper.EnvVar("PG_META_DB_HOST", resolved.Host),
			helper.EnvVar("PG_META_DB_PORT", strconv.Itoa(int(resolved.Port))),
			helper.EnvVar("PG_META_DB_NAME", resolved.DBName),
			helper.EnvVar("PG_META_DB_USER", "supabase_admin"),
			helper.EnvVar("PG_META_DB_SSL_MODE", "disable"),
			helper.EnvVar("PG_META_PORT", "8080"),
		},
		Resources:       m.Spec.Resources,
		SecurityContext: m.Spec.ContainerSecurityContext,
	}
	container.Env = append(container.Env, m.Spec.Env...)

	if m.Spec.Probes != nil {
		if m.Spec.Probes.Startup != nil {
			container.StartupProbe = m.Spec.Probes.Startup
		}
		if m.Spec.Probes.Readiness != nil {
			container.ReadinessProbe = m.Spec.Probes.Readiness
		}
		if m.Spec.Probes.Liveness != nil {
			container.LivenessProbe = m.Spec.Probes.Liveness
		}
	}

	return container
}

func (r *Reconciler) labelsForMeta(m *supabasev1alpha1.Meta, project *supabasev1alpha1.Project) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "meta",
		"app.kubernetes.io/instance":   m.Name,
		"app.kubernetes.io/component":  "meta",
		"app.kubernetes.io/managed-by": "supabase-operator",
		"app.kubernetes.io/part-of":    project.Name,
	}
}

func (r *Reconciler) selectorLabelsForMeta(m *supabasev1alpha1.Meta) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "meta",
		"app.kubernetes.io/instance": m.Name,
	}
}
