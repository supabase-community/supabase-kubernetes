package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func TestBuildAPIHTTPRoute_UsesProjectGatewayRef(t *testing.T) {
	project := &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "my-project", Namespace: "app"},
		Spec: platformv1alpha1.ProjectSpec{
			Version: "2026.04.27",
			Global:  platformv1alpha1.GlobalSpec{SiteURL: "https://app.example.com"},
			HTTP: platformv1alpha1.HTTPSpec{
				API: platformv1alpha1.HTTPConfig{
					Protocol: "https",
					Hostname: "api.example.com",
				},
				Studio: platformv1alpha1.HTTPConfig{
					Protocol: "https",
					Hostname: "studio.example.com",
				},
			},
			Gateway: platformv1alpha1.GatewaySpec{
				API: platformv1alpha1.ExistingGatewayRef{
					Name:      "api-gw",
					Namespace: "infra-gateway",
				},
				Studio: platformv1alpha1.ExistingGatewayRef{
					Name:      "studio-gw",
					Namespace: "infra-gateway",
				},
			},
			Database: platformv1alpha1.DatabaseSpec{Host: "postgres.db.svc", PasswordRef: platformv1alpha1.SecretKeyRef{Name: "db-secret", Key: "password"}},
			Auth:     &platformv1alpha1.AuthSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/gotrue:test"}},
		},
	}

	route := buildAPIHTTPRoute(project)

	if route.Name != "my-project-gateway-api" {
		t.Fatalf("expected route name %q, got %q", "my-project-gateway-api", route.Name)
	}
	if len(route.Spec.ParentRefs) != 1 {
		t.Fatalf("expected one parentRef, got %d", len(route.Spec.ParentRefs))
	}
	if route.Spec.ParentRefs[0].Name != "api-gw" {
		t.Fatalf("expected parent gateway name %q, got %q", "api-gw", route.Spec.ParentRefs[0].Name)
	}
	if route.Spec.ParentRefs[0].Namespace == nil || string(*route.Spec.ParentRefs[0].Namespace) != "infra-gateway" {
		t.Fatalf("expected parent gateway namespace %q", "infra-gateway")
	}
	if len(route.Spec.Hostnames) != 1 || string(route.Spec.Hostnames[0]) != "api.example.com" {
		t.Fatalf("expected hostname %q", "api.example.com")
	}
}

func TestBuildStudioHTTPRoute_UsesProjectGatewayRef(t *testing.T) {
	project := &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "my-project", Namespace: "app"},
		Spec: platformv1alpha1.ProjectSpec{
			Version: "2026.04.27",
			Global:  platformv1alpha1.GlobalSpec{SiteURL: "https://app.example.com"},
			HTTP: platformv1alpha1.HTTPSpec{
				API: platformv1alpha1.HTTPConfig{
					Protocol: "https",
					Hostname: "api.example.com",
				},
				Studio: platformv1alpha1.HTTPConfig{
					Protocol: "https",
					Hostname: "studio.example.com",
				},
			},
			Gateway: platformv1alpha1.GatewaySpec{
				API: platformv1alpha1.ExistingGatewayRef{
					Name:      "api-gw",
					Namespace: "infra-gateway",
				},
				Studio: platformv1alpha1.ExistingGatewayRef{
					Name:      "studio-gw",
					Namespace: "infra-gateway",
				},
			},
			Database: platformv1alpha1.DatabaseSpec{Host: "postgres.db.svc", PasswordRef: platformv1alpha1.SecretKeyRef{Name: "db-secret", Key: "password"}},
			Auth:     &platformv1alpha1.AuthSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/gotrue:test"}},
		},
	}

	route := buildStudioHTTPRoute(project)

	if route.Name != "my-project-gateway-studio" {
		t.Fatalf("expected route name %q, got %q", "my-project-gateway-studio", route.Name)
	}
	if len(route.Spec.ParentRefs) != 1 {
		t.Fatalf("expected one parentRef, got %d", len(route.Spec.ParentRefs))
	}
	if route.Spec.ParentRefs[0].Name != "studio-gw" {
		t.Fatalf("expected parent gateway name %q, got %q", "studio-gw", route.Spec.ParentRefs[0].Name)
	}
	if route.Spec.ParentRefs[0].Namespace == nil || string(*route.Spec.ParentRefs[0].Namespace) != "infra-gateway" {
		t.Fatalf("expected parent gateway namespace %q", "infra-gateway")
	}
	if len(route.Spec.Hostnames) != 1 || string(route.Spec.Hostnames[0]) != "studio.example.com" {
		t.Fatalf("expected hostname %q", "studio.example.com")
	}
	if len(route.Spec.Rules) != 1 {
		t.Fatalf("expected one rule, got %d", len(route.Spec.Rules))
	}
}

func TestBuildAPIHTTPRoute_NoEnabledComponents_ReturnsEmptyRules(t *testing.T) {
	f := false
	project := &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "my-project", Namespace: "app"},
		Spec: platformv1alpha1.ProjectSpec{
			Version: "2026.04.27",
			Global:  platformv1alpha1.GlobalSpec{SiteURL: "https://app.example.com"},
			HTTP: platformv1alpha1.HTTPSpec{
				API: platformv1alpha1.HTTPConfig{
					Protocol: "https",
					Hostname: "api.example.com",
				},
				Studio: platformv1alpha1.HTTPConfig{
					Protocol: "https",
					Hostname: "studio.example.com",
				},
			},
			Gateway: platformv1alpha1.GatewaySpec{
				API: platformv1alpha1.ExistingGatewayRef{
					Name:      "api-gw",
					Namespace: "infra-gateway",
				},
			},
			Database:  platformv1alpha1.DatabaseSpec{Host: "postgres.db.svc", PasswordRef: platformv1alpha1.SecretKeyRef{Name: "db-secret", Key: "password"}},
			Auth:      &platformv1alpha1.AuthSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}},
			Rest:      &platformv1alpha1.RestSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}},
			Realtime:  &platformv1alpha1.RealtimeSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}},
			Storage:   &platformv1alpha1.StorageSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}},
			Meta:      &platformv1alpha1.MetaSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}},
			Functions: &platformv1alpha1.FunctionsSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}},
		},
	}

	route := buildAPIHTTPRoute(project)
	if len(route.Spec.Rules) != 0 {
		t.Fatalf("expected empty rules when all API components are disabled, got %d", len(route.Spec.Rules))
	}
}

func TestBuildStudioHTTPRoute_Disabled_ReturnsEmptyRules(t *testing.T) {
	f := false
	project := &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "my-project", Namespace: "app"},
		Spec: platformv1alpha1.ProjectSpec{
			Version: "2026.04.27",
			Global:  platformv1alpha1.GlobalSpec{SiteURL: "https://app.example.com"},
			HTTP: platformv1alpha1.HTTPSpec{
				API: platformv1alpha1.HTTPConfig{
					Protocol: "https",
					Hostname: "api.example.com",
				},
				Studio: platformv1alpha1.HTTPConfig{
					Protocol: "https",
					Hostname: "studio.example.com",
				},
			},
			Gateway: platformv1alpha1.GatewaySpec{
				Studio: platformv1alpha1.ExistingGatewayRef{
					Name:      "studio-gw",
					Namespace: "infra-gateway",
				},
			},
			Database: platformv1alpha1.DatabaseSpec{Host: "postgres.db.svc", PasswordRef: platformv1alpha1.SecretKeyRef{Name: "db-secret", Key: "password"}},
			Studio:   &platformv1alpha1.StudioSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Enabled: &f}},
		},
	}

	route := buildStudioHTTPRoute(project)
	if len(route.Spec.Rules) != 0 {
		t.Fatalf("expected empty rules when studio is disabled, got %d", len(route.Spec.Rules))
	}
}
