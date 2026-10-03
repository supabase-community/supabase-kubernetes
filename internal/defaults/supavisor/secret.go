package supavisor

import (
	"net"
	"net/url"
	"strconv"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SupavisorSecretName(ctx *ResourceContext) string {
	return ComponentName(ctx.Names["Supavisor"], "supavisor-auth")
}

func SupavisorSecret(ctx *ResourceContext, db *core.ResolvedDatabase, password string) *corev1.Secret {
	connection := &url.URL{
		Scheme: "ecto", User: url.UserPassword("supabase_admin", password),
		Host: net.JoinHostPort(db.Host, strconv.Itoa(int(db.Port))), Path: "/_supabase",
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: SupavisorSecretName(ctx), Namespace: ctx.Namespace, Labels: SupavisorLabels(ctx)},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{DatabaseURLKey: []byte(connection.String())},
	}
}
