package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func newTestProject(name, namespace string, listeners []platformv1alpha1.GatewayListenerSpec) *platformv1alpha1.Project {
	return &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: platformv1alpha1.ProjectSpec{
			Global:   platformv1alpha1.GlobalSpec{SiteURL: "https://app.example.com"},
			Gateway:  platformv1alpha1.GatewaySpec{GatewayClassName: "envoy", Host: "api.example.com", Listeners: listeners},
			Database: platformv1alpha1.DatabaseSpec{Host: "postgres.db.svc", PasswordRef: platformv1alpha1.SecretKeyRef{Name: "db-secret", Key: "password"}},
		},
	}
}

func TestBuildGateway_Name(t *testing.T) {
	project := newTestProject("my-project", "ns1", []platformv1alpha1.GatewayListenerSpec{{Name: "http", Protocol: "HTTP", Port: 80}})
	gw := buildGateway(project)
	if gw.Name != "my-project-gateway" {
		t.Errorf("expected name %q, got %q", "my-project-gateway", gw.Name)
	}
	if gw.Namespace != "ns1" {
		t.Errorf("expected namespace %q, got %q", "ns1", gw.Namespace)
	}
}

func TestBuildGateway_GatewayClassName(t *testing.T) {
	project := newTestProject("test", "default", []platformv1alpha1.GatewayListenerSpec{{Name: "http", Protocol: "HTTP", Port: 80}})
	gw := buildGateway(project)
	if gw.Spec.GatewayClassName != gatewayv1.ObjectName("envoy") {
		t.Errorf("expected gatewayClassName %q, got %q", "envoy", gw.Spec.GatewayClassName)
	}
}

func TestBuildGateway_ListenerConversion(t *testing.T) {
	project := newTestProject("test", "default", []platformv1alpha1.GatewayListenerSpec{{Name: "http", Protocol: "HTTP", Port: 80}, {Name: "https", Protocol: "HTTPS", Port: 443}})
	gw := buildGateway(project)
	if len(gw.Spec.Listeners) != 2 {
		t.Fatalf("expected 2 listeners, got %d", len(gw.Spec.Listeners))
	}
	if gw.Spec.Listeners[0].Protocol != gatewayv1.ProtocolType("HTTP") {
		t.Errorf("listener[0] protocol mismatch")
	}
	if gw.Spec.Listeners[1].Protocol != gatewayv1.ProtocolType("HTTPS") {
		t.Errorf("listener[1] protocol mismatch")
	}
}
