package controller

import (
	"testing"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func TestEffectiveProtocol(t *testing.T) {
	tests := []struct {
		name string
		http platformv1alpha1.HTTPSpec
		want string
	}{
		{name: "http protocol", http: platformv1alpha1.HTTPSpec{Protocol: "http", Hostname: "api.example.com"}, want: "http"},
		{name: "https protocol", http: platformv1alpha1.HTTPSpec{Protocol: "https", Hostname: "api.example.com"}, want: "https"},
		{name: "case insensitive protocol", http: platformv1alpha1.HTTPSpec{Protocol: "HTTPS", Hostname: "api.example.com"}, want: "https"},
		{name: "invalid protocol defaults to http", http: platformv1alpha1.HTTPSpec{Protocol: "tcp", Hostname: "api.example.com"}, want: "http"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveProtocol(tt.http)
			if got != tt.want {
				t.Errorf("effectiveProtocol() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEffectivePort(t *testing.T) {
	httpPort := int32(8080)
	httpsPort := int32(8443)

	tests := []struct {
		name   string
		http   platformv1alpha1.HTTPSpec
		scheme string
		want   int32
	}{
		{name: "http with explicit port", http: platformv1alpha1.HTTPSpec{Protocol: "http", Hostname: "api.example.com", Port: &httpPort}, scheme: "http", want: 8080},
		{name: "https with explicit port", http: platformv1alpha1.HTTPSpec{Protocol: "https", Hostname: "api.example.com", Port: &httpsPort}, scheme: "https", want: 8443},
		{name: "http default port", http: platformv1alpha1.HTTPSpec{Protocol: "http", Hostname: "api.example.com"}, scheme: "http", want: 80},
		{name: "https default port", http: platformv1alpha1.HTTPSpec{Protocol: "https", Hostname: "api.example.com"}, scheme: "https", want: 443},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectivePort(tt.http, tt.scheme)
			if got != tt.want {
				t.Errorf("effectivePort() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildExternalURL(t *testing.T) {
	tests := []struct {
		name string
		http platformv1alpha1.HTTPSpec
		want string
	}{
		{name: "http standard port omitted", http: platformv1alpha1.HTTPSpec{Protocol: "http", Hostname: "api.example.com"}, want: "http://api.example.com"},
		{name: "https standard port omitted", http: platformv1alpha1.HTTPSpec{Protocol: "https", Hostname: "api.example.com"}, want: "https://api.example.com"},
		{name: "http non-standard port included", http: platformv1alpha1.HTTPSpec{Protocol: "http", Hostname: "api.example.com", Port: int32Ptr(8080)}, want: "http://api.example.com:8080"},
		{name: "https non-standard port included", http: platformv1alpha1.HTTPSpec{Protocol: "https", Hostname: "api.example.com", Port: int32Ptr(8443)}, want: "https://api.example.com:8443"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildExternalURL(tt.http)
			if got != tt.want {
				t.Errorf("buildExternalURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInternalURLUsesProjectHTTP(t *testing.T) {
	project := &platformv1alpha1.Project{
		Spec: platformv1alpha1.ProjectSpec{
			Version: "2026.04.27",
			HTTP: platformv1alpha1.HTTPSpec{
				Protocol: "https",
				Hostname: "api.example.com",
				GatewayRef: platformv1alpha1.ExistingGatewayRef{
					Name:      "gw",
					Namespace: "envoy-gateway-system",
				},
			},
		},
	}

	if got := InternalURL(project); got != "https://gw.envoy-gateway-system.svc.cluster.local" {
		t.Fatalf("InternalURL() = %q, want %q", got, "https://gw.envoy-gateway-system.svc.cluster.local")
	}
}

func int32Ptr(v int32) *int32 { return &v }
