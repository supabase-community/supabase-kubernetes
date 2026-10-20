package analytics

import (
	"fmt"
	"net"
	"net/url"
	"strconv"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AuthSecretPublicTokenKey  = "public-access-token"
	AuthSecretPrivateTokenKey = "private-access-token"
	AuthSecretURLKey          = "url"
)

// AnalyticsAuthSecretName returns the name of the Analytics credentials Secret.
func AnalyticsAuthSecretName(project *ResourceContext) string {
	return ComponentName(project.Names["Analytics"], "analytics-auth")
}

// AnalyticsAuthSecret constructs the tokens and backend connection for Analytics.
func AnalyticsAuthSecret(project *ResourceContext, db *core.ResolvedDatabase, password string) (*corev1.Secret, error) {
	publicToken, err := helper.GenerateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generating analytics public token: %w", err)
	}
	privateToken, err := helper.GenerateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generating analytics private token: %w", err)
	}
	connection := url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(db.User, password),
		Host:   net.JoinHostPort(db.Host, strconv.Itoa(int(db.Port))),
		Path:   "/_supabase",
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AnalyticsAuthSecretName(project),
			Namespace: project.Namespace,
			Labels:    AnalyticsLabels(project),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			AuthSecretPublicTokenKey:  []byte(publicToken),
			AuthSecretPrivateTokenKey: []byte(privateToken),
			AuthSecretURLKey:          []byte(connection.String()),
		},
	}, nil
}
