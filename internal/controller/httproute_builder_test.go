package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func TestBuildHTTPRoute_UsesProjectGatewayRef(t *testing.T) {
	project := &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "my-project", Namespace: "app"},
		Spec: platformv1alpha1.ProjectSpec{
			Version: "2026.04.27",
			Global:  platformv1alpha1.GlobalSpec{SiteURL: "https://app.example.com"},
			HTTP: platformv1alpha1.HTTPSpec{
				Protocol: "https",
				Hostname: "api.example.com",
				GatewayRef: platformv1alpha1.ExistingGatewayRef{
					Name:      "shared-gw",
					Namespace: "infra-gateway",
				},
			},
			Database: platformv1alpha1.DatabaseSpec{Host: "postgres.db.svc", PasswordRef: platformv1alpha1.SecretKeyRef{Name: "db-secret", Key: "password"}},
			Auth:     &platformv1alpha1.AuthSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/gotrue:test"}},
		},
	}

	route := buildHTTPRoute(project)

	if route.Name != "my-project-gateway" {
		t.Fatalf("expected route name %q, got %q", "my-project-gateway", route.Name)
	}
	if len(route.Spec.ParentRefs) != 1 {
		t.Fatalf("expected one parentRef, got %d", len(route.Spec.ParentRefs))
	}
	if route.Spec.ParentRefs[0].Name != "shared-gw" {
		t.Fatalf("expected parent gateway name %q, got %q", "shared-gw", route.Spec.ParentRefs[0].Name)
	}
	if route.Spec.ParentRefs[0].Namespace == nil || string(*route.Spec.ParentRefs[0].Namespace) != "infra-gateway" {
		t.Fatalf("expected parent gateway namespace %q", "infra-gateway")
	}
	if len(route.Spec.Hostnames) != 1 || string(route.Spec.Hostnames[0]) != "api.example.com" {
		t.Fatalf("expected hostname %q", "api.example.com")
	}
}
