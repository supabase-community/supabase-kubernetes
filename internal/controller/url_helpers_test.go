package controller

import (
	"testing"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func TestEffectiveProtocol(t *testing.T) {
	tests := []struct {
		name      string
		listeners []platformv1alpha1.GatewayListenerSpec
		want      string
	}{
		{name: "single HTTP listener", listeners: []platformv1alpha1.GatewayListenerSpec{{Name: "http", Protocol: "HTTP", Port: 80}}, want: "http"},
		{name: "single HTTPS listener", listeners: []platformv1alpha1.GatewayListenerSpec{{Name: "https", Protocol: "HTTPS", Port: 443}}, want: "https"},
		{name: "TLS listener", listeners: []platformv1alpha1.GatewayListenerSpec{{Name: "tls", Protocol: "TLS", Port: 8443}}, want: "https"},
		{name: "mixed HTTP and HTTPS prefers HTTPS", listeners: []platformv1alpha1.GatewayListenerSpec{{Name: "http", Protocol: "HTTP", Port: 80}, {Name: "https", Protocol: "HTTPS", Port: 443}}, want: "https"},
		{name: "empty listeners defaults to http", listeners: []platformv1alpha1.GatewayListenerSpec{}, want: "http"},
		{name: "case insensitive protocol", listeners: []platformv1alpha1.GatewayListenerSpec{{Name: "secure", Protocol: "https", Port: 443}}, want: "https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveProtocol(tt.listeners)
			if got != tt.want {
				t.Errorf("effectiveProtocol() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEffectivePort(t *testing.T) {
	listeners := []platformv1alpha1.GatewayListenerSpec{{Name: "http", Protocol: "HTTP", Port: 8080}, {Name: "https", Protocol: "HTTPS", Port: 8443}}

	tests := []struct {
		name   string
		scheme string
		want   int32
	}{
		{"http scheme", "http", 8080},
		{"https scheme", "https", 8443},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectivePort(listeners, tt.scheme)
			if got != tt.want {
				t.Errorf("effectivePort() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildExternalURL(t *testing.T) {
	tests := []struct {
		name      string
		host      string
		listeners []platformv1alpha1.GatewayListenerSpec
		want      string
	}{
		{name: "HTTP standard port omitted", host: "api.example.com", listeners: []platformv1alpha1.GatewayListenerSpec{{Name: "http", Protocol: "HTTP", Port: 80}}, want: "http://api.example.com"},
		{name: "HTTPS standard port omitted", host: "api.example.com", listeners: []platformv1alpha1.GatewayListenerSpec{{Name: "https", Protocol: "HTTPS", Port: 443}}, want: "https://api.example.com"},
		{name: "HTTP non-standard port included", host: "api.example.com", listeners: []platformv1alpha1.GatewayListenerSpec{{Name: "http", Protocol: "HTTP", Port: 8080}}, want: "http://api.example.com:8080"},
		{name: "HTTPS non-standard port included", host: "api.example.com", listeners: []platformv1alpha1.GatewayListenerSpec{{Name: "https", Protocol: "HTTPS", Port: 8443}}, want: "https://api.example.com:8443"},
		{name: "mixed prefers HTTPS with standard port", host: "api.example.com", listeners: []platformv1alpha1.GatewayListenerSpec{{Name: "http", Protocol: "HTTP", Port: 80}, {Name: "https", Protocol: "HTTPS", Port: 443}}, want: "https://api.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildExternalURL(tt.host, tt.listeners)
			if got != tt.want {
				t.Errorf("buildExternalURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
