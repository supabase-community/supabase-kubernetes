package envoy

import (
	"strings"
	"testing"
	"time"

	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	"sigs.k8s.io/yaml"
)

const testAuthCluster = "auth"

type testRoute struct {
	Name  string `json:"name"`
	Match struct {
		Path   string `json:"path"`
		Prefix string `json:"prefix"`
	} `json:"match"`
	Route struct {
		Cluster       string `json:"cluster"`
		PrefixRewrite string `json:"prefix_rewrite"`
		Timeout       string `json:"timeout"`
		IdleTimeout   string `json:"idle_timeout"`
	} `json:"route"`
	Filters map[string]struct {
		Disabled bool `json:"disabled"`
		RBAC     struct {
			Rules struct {
				Action   string         `json:"action"`
				Policies map[string]any `json:"policies"`
			} `json:"rules"`
		} `json:"rbac"`
	} `json:"typed_per_filter_config"`
}

func renderedRoutes(t *testing.T, data envoyTemplateData) []testRoute {
	t.Helper()
	config, err := renderEnvoyTemplate("lds", assets.EnvoyLDSTemplate, data)
	if err != nil {
		t.Fatal(err)
	}
	var listener struct {
		Resources []struct {
			FilterChains []struct {
				Filters []struct {
					Config struct {
						RouteConfig struct {
							VirtualHosts []struct {
								Routes []testRoute `json:"routes"`
							} `json:"virtual_hosts"`
						} `json:"route_config"`
					} `json:"typed_config"`
				} `json:"filters"`
			} `json:"filter_chains"`
		} `json:"resources"`
	}
	if err := yaml.Unmarshal([]byte(config), &listener); err != nil {
		t.Fatal(err)
	}
	for _, resource := range listener.Resources {
		for _, chain := range resource.FilterChains {
			for _, filter := range chain.Filters {
				if hosts := filter.Config.RouteConfig.VirtualHosts; len(hosts) == 1 {
					return hosts[0].Routes
				}
			}
		}
	}
	t.Fatal("expected an HTTP connection manager with one virtual host")
	return nil
}

func TestFunctionsTimeouts(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		routes := renderedRoutes(t, envoyTemplateData{FunctionsCluster: envoyCluster{Enabled: enabled}})
		found := 0
		for _, route := range routes {
			if route.Name != "functions-v1-all" {
				continue
			}
			found++
			// The runtime must report its own structured error before Envoy times out.
			for name, limit := range map[string]struct {
				value   string
				runtime time.Duration
			}{
				"wall clock": {route.Route.Timeout, 400 * time.Second},
				"idle":       {route.Route.IdleTimeout, 150 * time.Second},
			} {
				duration, err := time.ParseDuration(limit.value)
				if err != nil || duration <= limit.runtime {
					t.Errorf("%s timeout %q must outlast runtime limit %s", name, limit.value, limit.runtime)
				}
			}
		}
		if (enabled && found != 1) || (!enabled && found != 0) {
			t.Fatalf("functions enabled=%t: got %d routes", enabled, found)
		}
	}
}

func matchRoute(routes []testRoute, path string) (testRoute, bool) {
	for _, route := range routes {
		if route.Match.Path == path || (route.Match.Prefix != "" && strings.HasPrefix(path, route.Match.Prefix)) {
			return route, true
		}
	}
	return testRoute{}, false
}

func TestAuthDiscoveryRoutes(t *testing.T) {
	routes := renderedRoutes(t, envoyTemplateData{
		AuthCluster:   envoyCluster{Enabled: true},
		StudioCluster: envoyCluster{Enabled: true},
	})
	for path, upstream := range map[string]string{
		"/.well-known/oauth-authorization-server/auth/v1": "/.well-known/oauth-authorization-server",
		"/auth/v1/.well-known/oauth-authorization-server": "/.well-known/oauth-authorization-server",
		"/auth/v1/.well-known/openid-configuration":       "/.well-known/openid-configuration",
		"/auth/v1/.well-known/jwks.json":                  "/.well-known/jwks.json",
	} {
		t.Run(path, func(t *testing.T) {
			route, found := matchRoute(routes, path)
			if !found || route.Route.Cluster != testAuthCluster || route.Route.PrefixRewrite != upstream {
				t.Fatalf("discovery request %q does not reach %q on Auth: %+v", path, upstream, route)
			}
			if !route.Filters["envoy.filters.http.basic_auth"].Disabled {
				t.Fatal("discovery must not require Studio credentials")
			}
			rules := route.Filters["envoy.filters.http.rbac"].RBAC.Rules
			if rules.Action != "ALLOW" || rules.Policies["allow_all"] == nil {
				t.Fatal("discovery must be accessible without an API key")
			}
		})
	}
	for _, path := range []string{
		"/.well-known/oauth-authorization-server",
		"/.well-known/oauth-authorization-server/auth/v1/unexpected",
	} {
		route, _ := matchRoute(routes, path)
		if route.Route.Cluster == testAuthCluster {
			t.Errorf("unsupported discovery path %q still reaches Auth", path)
		}
	}
	// The public discovery exceptions must not make the rest of Auth public.
	private, found := matchRoute(routes, "/auth/v1/user")
	if !found || private.Route.Cluster != testAuthCluster || private.Filters["envoy.filters.http.rbac"].RBAC.Rules.Policies["allow_all"] != nil {
		t.Fatal("regular Auth requests must still require an API key")
	}
}

func TestAuthDiscoveryRemovedWithComponent(t *testing.T) {
	routes := renderedRoutes(t, envoyTemplateData{StudioCluster: envoyCluster{Enabled: true}})
	for _, route := range routes {
		if route.Route.Cluster == testAuthCluster {
			t.Fatal("disabled Auth component still has a gateway route")
		}
	}
}
