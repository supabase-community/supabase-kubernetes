package controller

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func (r *ProjectReconciler) reconcileHTTPRoute(ctx context.Context, project *platformv1alpha1.Project) error {
	logger := log.FromContext(ctx)

	apiRoute := buildAPIHTTPRoute(project)
	if err := r.reconcileSingleHTTPRoute(ctx, project, apiRoute, "api"); err != nil {
		return err
	}

	studioRoute := buildStudioHTTPRoute(project)
	if err := r.reconcileSingleHTTPRoute(ctx, project, studioRoute, "studio"); err != nil {
		return err
	}

	if err := r.reconcileStudioBasicAuth(ctx, project); err != nil {
		return err
	}

	logger.Info("Reconciled HTTPRoutes")
	return nil
}

func (r *ProjectReconciler) reconcileStudioBasicAuth(ctx context.Context, project *platformv1alpha1.Project) error {
	logger := log.FromContext(ctx)

	desired := buildStudioBasicAuthSecurityPolicy(project)

	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion("gateway.envoyproxy.io/v1alpha1")
	existing.SetKind("SecurityPolicy")

	key := client.ObjectKey{
		Name:      project.Name + "-studio-basic-auth",
		Namespace: project.Namespace,
	}

	if desired == nil {
		// Studio disabled or basic auth not applicable: delete if exists
		if err := r.Get(ctx, key, existing); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("fetching studio SecurityPolicy: %w", err)
		}
		logger.Info("Deleting studio SecurityPolicy")
		if err := r.Delete(ctx, existing); err != nil {
			return fmt.Errorf("deleting studio SecurityPolicy: %w", err)
		}
		return nil
	}

	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on studio SecurityPolicy: %w", err)
	}

	// Read the dashboard secret so we can stamp its resourceVersion onto the
	// SecurityPolicy. This forces Envoy Gateway to re-sync whenever the secret
	// is deleted and recreated (or otherwise changed).
	dashboardSecret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      project.Name + "-dashboard",
		Namespace: project.Namespace,
	}
	if err := r.Get(ctx, secretKey, dashboardSecret); err != nil {
		return fmt.Errorf("getting dashboard secret for SecurityPolicy annotation: %w", err)
	}

	annotations := desired.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["supabase.io/dashboard-secret-version"] = dashboardSecret.ResourceVersion
	desired.SetAnnotations(annotations)

	if err := r.Get(ctx, key, existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("fetching studio SecurityPolicy: %w", err)
		}
		logger.Info("Creating studio SecurityPolicy")
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating studio SecurityPolicy: %w", err)
		}
		return nil
	}

	needsUpdate := false
	if existing.Object["spec"] == nil || !reflect.DeepEqual(existing.Object["spec"], desired.Object["spec"]) {
		existing.Object["spec"] = desired.Object["spec"]
		needsUpdate = true
	}

	existingAnnotations := existing.GetAnnotations()
	if existingAnnotations == nil {
		existingAnnotations = make(map[string]string)
	}
	if existingAnnotations["supabase.io/dashboard-secret-version"] != annotations["supabase.io/dashboard-secret-version"] {
		existingAnnotations["supabase.io/dashboard-secret-version"] = annotations["supabase.io/dashboard-secret-version"]
		existing.SetAnnotations(existingAnnotations)
		needsUpdate = true
	}

	if !needsUpdate {
		return nil
	}

	logger.Info("Updating studio SecurityPolicy")
	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("updating studio SecurityPolicy: %w", err)
	}

	return nil
}

func (r *ProjectReconciler) reconcileSingleHTTPRoute(
	ctx context.Context,
	project *platformv1alpha1.Project,
	desired *gatewayv1.HTTPRoute,
	routeType string,
) error {
	logger := log.FromContext(ctx)

	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on %s HTTPRoute: %w", routeType, err)
	}

	existing := &gatewayv1.HTTPRoute{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("fetching %s HTTPRoute: %w", routeType, err)
		}
		// Not found
		if len(desired.Spec.Rules) == 0 {
			return nil
		}
		logger.Info("Creating HTTPRoute", "name", desired.Name, "type", routeType)
		if createErr := r.Create(ctx, desired); createErr != nil {
			return fmt.Errorf("creating %s HTTPRoute: %w", routeType, createErr)
		}
		return nil
	}

	if len(desired.Spec.Rules) == 0 {
		logger.Info("Deleting HTTPRoute", "name", desired.Name, "type", routeType)
		if deleteErr := r.Delete(ctx, existing); deleteErr != nil {
			return fmt.Errorf("deleting %s HTTPRoute: %w", routeType, deleteErr)
		}
		return nil
	}

	existing.Spec = desired.Spec
	logger.Info("Updating HTTPRoute", "name", existing.Name, "type", routeType)
	if updateErr := r.Update(ctx, existing); updateErr != nil {
		return fmt.Errorf("updating %s HTTPRoute: %w", routeType, updateErr)
	}

	return nil
}
