package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

type routeDefinition struct {
	pathPrefix       string
	rewritePrefix    *string
	backendName      string
	backendPort      int32
	componentEnabled bool
}

func buildHTTPRoute(project *platformv1alpha1.Project) *gatewayv1.HTTPRoute {
	hostname := gatewayv1.Hostname(project.Spec.HTTP.Hostname)
	parentNamespace := gatewayv1.Namespace(project.Spec.HTTP.GatewayRef.Namespace)

	defs := []routeDefinition{
		{pathPrefix: "/realtime/v1/api", rewritePrefix: strPtr("/api"), backendName: componentServiceName(project.Name, "realtime"), backendPort: 4000, componentEnabled: project.Spec.Realtime != nil && derefBool(project.Spec.Realtime.Enabled, true)},
		{pathPrefix: "/auth/v1", rewritePrefix: strPtr("/"), backendName: componentServiceName(project.Name, "auth"), backendPort: 9999, componentEnabled: project.Spec.Auth != nil && derefBool(project.Spec.Auth.Enabled, true)},
		{pathPrefix: "/rest/v1", rewritePrefix: strPtr("/"), backendName: componentServiceName(project.Name, "rest"), backendPort: 3000, componentEnabled: project.Spec.Rest != nil && derefBool(project.Spec.Rest.Enabled, true)},
		{pathPrefix: "/graphql/v1", rewritePrefix: strPtr("/rpc/graphql"), backendName: componentServiceName(project.Name, "rest"), backendPort: 3000, componentEnabled: project.Spec.Rest != nil && derefBool(project.Spec.Rest.Enabled, true)},
		{pathPrefix: "/realtime/v1", rewritePrefix: strPtr("/socket"), backendName: componentServiceName(project.Name, "realtime"), backendPort: 4000, componentEnabled: project.Spec.Realtime != nil && derefBool(project.Spec.Realtime.Enabled, true)},
		{pathPrefix: "/storage/v1", rewritePrefix: strPtr("/"), backendName: componentServiceName(project.Name, "storage"), backendPort: 5000, componentEnabled: project.Spec.Storage != nil && derefBool(project.Spec.Storage.Enabled, true)},
		{pathPrefix: "/functions/v1", rewritePrefix: strPtr("/"), backendName: componentServiceName(project.Name, "functions"), backendPort: 9000, componentEnabled: project.Spec.Functions != nil && derefBool(project.Spec.Functions.Enabled, true)},
		{pathPrefix: "/pg", rewritePrefix: strPtr("/"), backendName: componentServiceName(project.Name, "meta"), backendPort: 8080, componentEnabled: project.Spec.Meta != nil && derefBool(project.Spec.Meta.Enabled, true)},
		{pathPrefix: "/", rewritePrefix: nil, backendName: componentServiceName(project.Name, "studio"), backendPort: 3000, componentEnabled: project.Spec.Studio != nil && derefBool(project.Spec.Studio.Enabled, true)},
	}

	rules := make([]gatewayv1.HTTPRouteRule, 0, len(defs))
	for _, def := range defs {
		if !def.componentEnabled {
			continue
		}
		rules = append(rules, buildHTTPRouteRule(def))
	}

	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      httpRouteName(project),
			Namespace: project.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "supabase",
				"app.kubernetes.io/instance":   project.Name,
				"app.kubernetes.io/managed-by": "supabase-operator",
				"app.kubernetes.io/component":  "gateway",
			},
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{
					Name:      gatewayv1.ObjectName(project.Spec.HTTP.GatewayRef.Name),
					Namespace: &parentNamespace,
				}},
			},
			Hostnames: []gatewayv1.Hostname{hostname},
			Rules:     rules,
		},
	}
}

func buildHTTPRouteRule(def routeDefinition) gatewayv1.HTTPRouteRule {
	pathType := gatewayv1.PathMatchPathPrefix
	port := gatewayv1.PortNumber(def.backendPort)

	rule := gatewayv1.HTTPRouteRule{
		Matches: []gatewayv1.HTTPRouteMatch{{
			Path: &gatewayv1.HTTPPathMatch{
				Type:  &pathType,
				Value: &def.pathPrefix,
			},
		}},
		BackendRefs: []gatewayv1.HTTPBackendRef{{
			BackendRef: gatewayv1.BackendRef{
				BackendObjectReference: gatewayv1.BackendObjectReference{
					Name: gatewayv1.ObjectName(def.backendName),
					Port: &port,
				},
			},
		}},
	}

	if def.rewritePrefix != nil {
		rule.Filters = []gatewayv1.HTTPRouteFilter{{
			Type: gatewayv1.HTTPRouteFilterURLRewrite,
			URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
				Path: &gatewayv1.HTTPPathModifier{
					Type:               gatewayv1.PrefixMatchHTTPPathModifier,
					ReplacePrefixMatch: def.rewritePrefix,
				},
			},
		}}
	}

	return rule
}

func strPtr(v string) *string {
	return &v
}

func httpRouteName(project *platformv1alpha1.Project) string {
	return project.Name + "-gateway"
}
