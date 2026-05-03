package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func (r *ProjectReconciler) reconcileGateway(ctx context.Context, project *platformv1alpha1.Project) error {
	logger := log.FromContext(ctx)

	desired := buildGateway(project)
	if err := controllerutil.SetControllerReference(project, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on Gateway: %w", err)
	}

	existing := &gatewayv1.Gateway{}
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("fetching Gateway: %w", err)
	}

	if err != nil {
		logger.Info("Creating Gateway", "name", desired.Name)
		if createErr := r.Create(ctx, desired); createErr != nil {
			return fmt.Errorf("creating Gateway: %w", createErr)
		}
		return nil
	}

	existing.Spec = desired.Spec
	logger.Info("Updating Gateway", "name", existing.Name)
	if updateErr := r.Update(ctx, existing); updateErr != nil {
		return fmt.Errorf("updating Gateway: %w", updateErr)
	}
	return nil
}

func buildGateway(project *platformv1alpha1.Project) *gatewayv1.Gateway {
	hostname := gatewayv1.Hostname(project.Spec.Gateway.Host)

	listeners := make([]gatewayv1.Listener, 0, len(project.Spec.Gateway.Listeners))
	for _, l := range project.Spec.Gateway.Listeners {
		listeners = append(listeners, gatewayv1.Listener{
			Name:     gatewayv1.SectionName(l.Name),
			Protocol: gatewayv1.ProtocolType(l.Protocol),
			Port:     gatewayv1.PortNumber(l.Port),
			Hostname: &hostname,
		})
	}

	return &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      project.Name + "-gateway",
			Namespace: project.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "supabase",
				"app.kubernetes.io/instance":   project.Name,
				"app.kubernetes.io/managed-by": "supabase-operator",
				"app.kubernetes.io/component":  "gateway",
			},
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName(project.Spec.Gateway.GatewayClassName),
			Listeners:        listeners,
		},
	}
}
