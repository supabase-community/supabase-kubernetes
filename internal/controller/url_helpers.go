package controller

import (
	"fmt"
	"strings"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

const (
	httpScheme  = "http"
	httpsScheme = "https"
)

func effectiveProtocol(listeners []platformv1alpha1.GatewayListenerSpec) string {
	for _, l := range listeners {
		if strings.EqualFold(l.Protocol, "HTTPS") || strings.EqualFold(l.Protocol, "TLS") {
			return httpsScheme
		}
	}
	return httpScheme
}

func effectivePort(listeners []platformv1alpha1.GatewayListenerSpec, scheme string) int32 {
	for _, l := range listeners {
		proto := strings.ToLower(l.Protocol)
		if scheme == httpsScheme && (proto == "https" || proto == "tls") {
			return l.Port
		}
		if scheme == httpScheme && proto == "http" {
			return l.Port
		}
	}
	return 0
}

func buildExternalURL(host string, listeners []platformv1alpha1.GatewayListenerSpec) string {
	scheme := effectiveProtocol(listeners)
	port := effectivePort(listeners, scheme)

	if (scheme == httpScheme && port == 80) || (scheme == httpsScheme && port == 443) || port == 0 {
		return fmt.Sprintf("%s://%s", scheme, host)
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, port)
}

func supabasePublicURL(project *platformv1alpha1.Project) string {
	return buildExternalURL(project.Spec.Gateway.Host, project.Spec.Gateway.Listeners)
}

func supabaseInternalURL(project *platformv1alpha1.Project) string {
	return fmt.Sprintf("http://%s-gateway.%s.svc.cluster.local", project.Name, project.Namespace)
}

func storagePublicURL(project *platformv1alpha1.Project) string {
	return fmt.Sprintf("%s/storage/v1", supabasePublicURL(project))
}

// PublicURL returns the public-facing base URL for the project.
func PublicURL(project *platformv1alpha1.Project) string {
	return supabasePublicURL(project)
}

// InternalURL returns the cluster-internal URL for the project.
func InternalURL(project *platformv1alpha1.Project) string {
	return supabaseInternalURL(project)
}

// StoragePublicURL returns the public URL for the storage API.
func StoragePublicURL(project *platformv1alpha1.Project) string {
	return storagePublicURL(project)
}
