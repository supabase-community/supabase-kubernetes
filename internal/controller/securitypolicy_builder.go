package controller

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func buildStudioBasicAuthSecurityPolicy(project *platformv1alpha1.Project) *unstructured.Unstructured {
	studioEnabled := project.Spec.Studio == nil || derefBool(project.Spec.Studio.Enabled, true)
	if !studioEnabled {
		return nil
	}

	sp := &unstructured.Unstructured{}
	sp.SetAPIVersion("gateway.envoyproxy.io/v1alpha1")
	sp.SetKind("SecurityPolicy")
	sp.SetName(project.Name + "-studio-basic-auth")
	sp.SetNamespace(project.Namespace)
	sp.SetLabels(map[string]string{
		"app.kubernetes.io/name":       "supabase",
		"app.kubernetes.io/instance":   project.Name,
		"app.kubernetes.io/managed-by": "supabase-operator",
		"app.kubernetes.io/component":  "gateway-studio",
	})

	sp.Object["spec"] = map[string]any{
		"targetRefs": []map[string]any{
			{
				"group": "gateway.networking.k8s.io",
				"kind":  "HTTPRoute",
				"name":  studioHTTPRouteName(project),
			},
		},
		"basicAuth": map[string]any{
			"users": map[string]any{
				"name": project.Name + "-studio",
			},
		},
	}

	return sp
}
