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

	desired := buildHTTPRoute(project)
	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on HTTPRoute: %w", err)
	}

	existing := &gatewayv1.HTTPRoute{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("fetching HTTPRoute: %w", err)
	}

	if err != nil {
		logger.Info("Creating HTTPRoute", "name", desired.Name)
		if createErr := r.Create(ctx, desired); createErr != nil {
			return fmt.Errorf("creating HTTPRoute: %w", createErr)
		}
		return nil
	}

	existing.Spec = desired.Spec
	logger.Info("Updating HTTPRoute", "name", existing.Name)
	if updateErr := r.Update(ctx, existing); updateErr != nil {
		return fmt.Errorf("updating HTTPRoute: %w", updateErr)
	}

	return nil
}
