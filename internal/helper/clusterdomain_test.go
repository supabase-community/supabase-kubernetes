package helper

import "testing"

func TestServiceFQDN(t *testing.T) {
	t.Cleanup(func() { SetClusterDomain(DefaultClusterDomain) })

	if got := ServiceFQDN("supabase-postgres", "supabase"); got != "supabase-postgres.supabase.svc.cluster.local" {
		t.Fatalf("unexpected default name: %s", got)
	}

	cases := map[string]string{
		"example.local":  "supabase-postgres.supabase.svc.example.local",
		"example.local.": "supabase-postgres.supabase.svc.example.local",
		" example.local": "supabase-postgres.supabase.svc.example.local",
		"":               "supabase-postgres.supabase.svc.cluster.local",
	}
	for domain, want := range cases {
		SetClusterDomain(domain)
		if got := ServiceFQDN("supabase-postgres", "supabase"); got != want {
			t.Fatalf("domain %q: got %s, want %s", domain, got, want)
		}
	}
}
