package helper

import (
	"fmt"
	"strings"
)

// DefaultClusterDomain is the domain a cluster serves unless kubelet and CoreDNS
// were configured with another one.
const DefaultClusterDomain = "cluster.local"

// clusterDomain is read on every reconciliation and written once, from the
// manager flags, before the controllers start.
var clusterDomain = DefaultClusterDomain

// SetClusterDomain sets the domain used to build in-cluster service names. An
// empty value keeps the default. Call it during startup only, before the
// manager starts the controllers.
func SetClusterDomain(domain string) {
	domain = strings.Trim(strings.TrimSpace(domain), ".")
	if domain == "" {
		domain = DefaultClusterDomain
	}
	clusterDomain = domain
}

// ClusterDomain returns the domain in use for in-cluster service names.
func ClusterDomain() string {
	return clusterDomain
}

// ServiceFQDN returns the fully qualified DNS name of a Service.
func ServiceFQDN(name, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.%s", name, namespace, clusterDomain)
}
