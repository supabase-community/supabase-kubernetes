package envoy

import (
	"testing"
	"time"

	"github.com/supabase-community/supabase-kubernetes/internal/assets"
	"sigs.k8s.io/yaml"
)

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
