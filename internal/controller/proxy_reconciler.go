package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func apiComponentsActive(project *platformv1alpha1.Project) bool {
	auth := project.Spec.Auth == nil || derefBool(project.Spec.Auth.Enabled, true)
	rest := project.Spec.Rest == nil || derefBool(project.Spec.Rest.Enabled, true)
	realtime := project.Spec.Realtime == nil || derefBool(project.Spec.Realtime.Enabled, true)
	storage := project.Spec.Storage == nil || derefBool(project.Spec.Storage.Enabled, true)
	functions := project.Spec.Functions == nil || derefBool(project.Spec.Functions.Enabled, true)
	meta := project.Spec.Meta == nil || derefBool(project.Spec.Meta.Enabled, true)
	return auth || rest || realtime || storage || functions || meta
}

func buildProxyEnvVars(project *platformv1alpha1.Project, proxyType string) []corev1.EnvVar {
	name := project.Name
	ns := project.Namespace
	jwtSecret := jwtSecretName(name)

	if proxyType == proxyAPIComponent {
		return []corev1.EnvVar{
			envVar("AUTH_ADDRESS", serviceHost(name, ns, "auth")),
			envVar("REST_ADDRESS", serviceHost(name, ns, "rest")),
			envVar("REALTIME_ADDRESS", serviceHost(name, ns, "realtime")),
			envVar("STORAGE_ADDRESS", serviceHost(name, ns, "storage")),
			envVar("FUNCTIONS_ADDRESS", serviceHost(name, ns, "functions")),
			envVar("META_ADDRESS", serviceHost(name, ns, "meta")),
			envVarFromSecret("ANON_KEY", jwtSecret, "anon-key"),
			envVarFromSecret("ANON_KEY_ASYMMETRIC", jwtSecret, "anon-key-asymmetric"),
			envVarFromSecret("SERVICE_ROLE_KEY", jwtSecret, "service-key"),
			envVarFromSecret("SERVICE_ROLE_KEY_ASYMMETRIC", jwtSecret, "service-key-asymmetric"),
			envVarFromSecret("SUPABASE_PUBLISHABLE_KEY", jwtSecret, "publishable-key"),
			envVarFromSecret("SUPABASE_SECRET_KEY", jwtSecret, "secret-key"),
		}
	}

	return []corev1.EnvVar{
		envVar("STUDIO_ADDRESS", serviceHost(name, ns, "studio")),
		envVarFromSecret("DASHBOARD_BASIC_AUTH", name+"-studio", ".htpasswd"),
	}
}

func (r *ProjectReconciler) reconcileProxy(ctx context.Context, project *platformv1alpha1.Project) error {
	logger := log.FromContext(ctx)

	// API proxy
	apiProxyEnabled := derefBool(project.Spec.HTTP.API.Enabled, true)
	if apiProxyEnabled && apiComponentsActive(project) {
		if err := r.reconcileProxyEndpoint(ctx, project, proxyAPIComponent); err != nil {
			return err
		}
	} else {
		if err := r.deleteProxyEndpoint(ctx, project, proxyAPIComponent); err != nil {
			return err
		}
	}

	// Studio proxy
	studioProxyEnabled := derefBool(project.Spec.HTTP.Studio.Enabled, true)
	studioEnabled := project.Spec.Studio == nil || derefBool(project.Spec.Studio.Enabled, true)
	if studioProxyEnabled && studioEnabled {
		if err := r.reconcileProxyEndpoint(ctx, project, proxyStudioComponent); err != nil {
			return err
		}
	} else {
		if err := r.deleteProxyEndpoint(ctx, project, proxyStudioComponent); err != nil {
			return err
		}
	}

	logger.Info("Reconciled proxy endpoints")
	return nil
}

func (r *ProjectReconciler) reconcileProxyEndpoint(ctx context.Context, project *platformv1alpha1.Project, proxyType string) error {
	logger := log.FromContext(ctx).WithValues("proxy", proxyType)

	var spec *platformv1alpha1.HTTPConfig
	if proxyType == proxyAPIComponent {
		spec = &project.Spec.HTTP.API
	} else {
		spec = &project.Spec.HTTP.Studio
	}

	image := ""
	if spec != nil {
		image = spec.Image
	}
	if image == "" {
		var err error
		image, err = resolveComponentImage(project, componentProxy, "")
		if err != nil {
			return fmt.Errorf("resolving proxy image: %w", err)
		}
	}

	replicas := int32(1)
	resources := corev1.ResourceRequirements{}
	serviceType := corev1.ServiceTypeClusterIP
	if spec != nil {
		if spec.Replicas != nil {
			replicas = *spec.Replicas
		}
		resources = spec.Resources
		if spec.ServiceType != "" {
			serviceType = spec.ServiceType
		}
	}

	envVars := buildProxyEnvVars(project, proxyType)

	// Reconcile ConfigMap
	cm := buildProxyConfigMap(project, proxyType)
	if err := controllerutil.SetControllerReference(project, cm, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on proxy ConfigMap: %w", err)
	}
	existingCM := &corev1.ConfigMap{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(cm), existingCM); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting proxy ConfigMap: %w", err)
		}
		logger.Info("Creating proxy ConfigMap", "name", cm.Name)
		if err := r.Create(ctx, cm); err != nil {
			return fmt.Errorf("creating proxy ConfigMap: %w", err)
		}
	} else {
		existingCM.Data = cm.Data
		logger.Info("Updating proxy ConfigMap", "name", cm.Name)
		if err := r.Update(ctx, existingCM); err != nil {
			return fmt.Errorf("updating proxy ConfigMap: %w", err)
		}
	}

	// Reconcile Deployment
	hash, err := r.computeEnvSecretHash(ctx, project.Namespace, envVars)
	if err != nil {
		return fmt.Errorf("computing secret hash for proxy %s: %w", proxyType, err)
	}
	deploy := buildProxyDeployment(project, proxyType, image, &replicas, resources, envVars)
	deploy.Spec.Template.Annotations = mergeAnnotations(deploy.Spec.Template.Annotations, map[string]string{
		"supabase.io/secret-hash": hash,
	})
	if err := controllerutil.SetControllerReference(project, deploy, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on proxy Deployment: %w", err)
	}
	existingDeploy := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(deploy), existingDeploy); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting proxy Deployment: %w", err)
		}
		logger.Info("Creating proxy Deployment", "name", deploy.Name)
		if err := r.Create(ctx, deploy); err != nil {
			return fmt.Errorf("creating proxy Deployment: %w", err)
		}
	} else {
		existingDeploy.Spec = deploy.Spec
		logger.Info("Updating proxy Deployment", "name", deploy.Name)
		if err := r.Update(ctx, existingDeploy); err != nil {
			return fmt.Errorf("updating proxy Deployment: %w", err)
		}
	}

	// Reconcile Service
	svc := buildProxyService(project, proxyType, serviceType)
	if err := controllerutil.SetControllerReference(project, svc, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on proxy Service: %w", err)
	}
	existingSvc := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(svc), existingSvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting proxy Service: %w", err)
		}
		logger.Info("Creating proxy Service", "name", svc.Name)
		if err := r.Create(ctx, svc); err != nil {
			return fmt.Errorf("creating proxy Service: %w", err)
		}
	} else {
		existingSvc.Spec = svc.Spec
		logger.Info("Updating proxy Service", "name", svc.Name)
		if err := r.Update(ctx, existingSvc); err != nil {
			return fmt.Errorf("updating proxy Service: %w", err)
		}
	}

	return nil
}

func (r *ProjectReconciler) deleteProxyEndpoint(ctx context.Context, project *platformv1alpha1.Project, proxyType string) error {
	logger := log.FromContext(ctx).WithValues("proxy", proxyType)

	svcName := proxyServiceName(project.Name, proxyType)
	svc := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Name: svcName, Namespace: project.Namespace}, svc); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting proxy Service for deletion: %w", err)
		}
	} else {
		logger.Info("Deleting proxy Service", "name", svcName)
		if err := r.Delete(ctx, svc); err != nil {
			return fmt.Errorf("deleting proxy Service: %w", err)
		}
	}

	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: svcName, Namespace: project.Namespace}, deploy); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting proxy Deployment for deletion: %w", err)
		}
	} else {
		logger.Info("Deleting proxy Deployment", "name", svcName)
		if err := r.Delete(ctx, deploy); err != nil {
			return fmt.Errorf("deleting proxy Deployment: %w", err)
		}
	}

	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, client.ObjectKey{Name: svcName, Namespace: project.Namespace}, cm); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting proxy ConfigMap for deletion: %w", err)
		}
	} else {
		logger.Info("Deleting proxy ConfigMap", "name", svcName)
		if err := r.Delete(ctx, cm); err != nil {
			return fmt.Errorf("deleting proxy ConfigMap: %w", err)
		}
	}

	return nil
}
