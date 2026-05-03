package v1alpha1_test

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
)

func minimalValidProject(name string) *platformv1alpha1.Project {
	return &platformv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: platformv1alpha1.ProjectSpec{
			Global: platformv1alpha1.GlobalSpec{
				SiteURL: "https://app.example.com",
			},
			Gateway: platformv1alpha1.GatewaySpec{
				GatewayClassName: "envoy",
				Host:             "api.example.com",
				Listeners: []platformv1alpha1.GatewayListenerSpec{
					{Name: "http", Protocol: "HTTP", Port: 80},
				},
			},
			Database: platformv1alpha1.DatabaseSpec{
				Host: "postgres.db.svc",
				PasswordRef: platformv1alpha1.SecretKeyRef{
					Name: "db-secret",
					Key:  "password",
				},
			},
			Studio:    &platformv1alpha1.StudioSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/studio:latest"}},
			Auth:      &platformv1alpha1.AuthSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/gotrue:latest"}},
			Rest:      &platformv1alpha1.RestSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "postgrest/postgrest:latest"}},
			Realtime:  &platformv1alpha1.RealtimeSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/realtime:latest"}},
			Storage:   &platformv1alpha1.StorageSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/storage-api:latest"}},
			Meta:      &platformv1alpha1.MetaSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/postgres-meta:latest"}},
			Functions: &platformv1alpha1.FunctionsSpec{ComponentSpec: platformv1alpha1.ComponentSpec{Image: "supabase/edge-runtime:latest"}},
		},
	}
}

func int32Ptr(i int32) *int32 { return &i }
func boolPtr(b bool) *bool    { return &b }
