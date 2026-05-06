package controller

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func (r *ProjectReconciler) reconcileHTTPRoute(ctx context.Context, project *platformv1alpha1.Project) error {
	logger := log.FromContext(ctx)

	if err := r.reconcileSingleHTTPRoute(ctx, project, buildAPIHTTPRoute, "api"); err != nil {
		return err
	}

	if err := r.reconcileSingleHTTPRoute(ctx, project, buildStudioHTTPRoute, "studio"); err != nil {
		return err
	}

	logger.Info("Reconciled HTTPRoutes")
	return nil
}

func (r *ProjectReconciler) reconcileSingleHTTPRoute(
	ctx context.Context,
	project *platformv1alpha1.Project,
	builder func(*platformv1alpha1.Project) *gatewayv1.HTTPRoute,
	routeType string,
) error {
	logger := log.FromContext(ctx)

	desired := builder(project)
	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on %s HTTPRoute: %w", routeType, err)
	}

	existing := &gatewayv1.HTTPRoute{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("fetching %s HTTPRoute: %w", routeType, err)
	}

	if err != nil {
		logger.Info("Creating HTTPRoute", "name", desired.Name, "type", routeType)
		if createErr := r.Create(ctx, desired); createErr != nil {
			return fmt.Errorf("creating %s HTTPRoute: %w", routeType, createErr)
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
