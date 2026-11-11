package envoy

import "testing"

func TestFunctionsOwnTheirCORSPolicy(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		routes := renderedRoutes(t, envoyTemplateData{
			FunctionsCluster: envoyCluster{Enabled: enabled},
			RestCluster:      envoyCluster{Enabled: true},
		})
		functionRoutes := 0
		for _, route := range routes {
			disabled := route.Filters["envoy.filters.http.cors"].Disabled
			if route.Name == "functions-v1-all" {
				functionRoutes++
				if !disabled {
					t.Fatal("Functions must receive preflights and control their own CORS responses")
				}
			} else if disabled {
				t.Fatal("disabling CORS for Functions must not disable it for other routes")
			}
		}
		if (enabled && functionRoutes != 1) || (!enabled && functionRoutes != 0) {
			t.Fatalf("functions enabled=%t: got %d routes", enabled, functionRoutes)
		}
	}
}
