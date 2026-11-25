package project

import (
	"context"
	"encoding/hex"
	"reflect"
	"testing"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	projectdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/project"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestKeysSecretBackfillsRealtimeCookieWithoutRotatingKeys(t *testing.T) {
	for _, existingCookie := range []string{"", "user-managed-cookie"} {
		t.Run(existingCookie, func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{core.AddToScheme, corev1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			p := &core.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "test", UID: "project"}}
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "demo-keys", Namespace: p.Namespace}, Data: map[string][]byte{
				projectdefaults.KeysSecretSecretKeyBase:    []byte("existing-secret-key-base"),
				projectdefaults.KeysSecretCryptoKey:        []byte("existing-crypto-key"),
				projectdefaults.KeysSecretVaultEncKey:      []byte("existing-vault-key"),
				projectdefaults.KeysSecretRealtimeDBEncKey: []byte("existing-db-key"),
				"custom-key": []byte("keep-me"),
			}}
			cookieKey := projectdefaults.KeysSecretRealtimeReleaseCookie
			if existingCookie != "" {
				secret.Data[cookieKey] = []byte(existingCookie)
			}
			if err := controllerutil.SetControllerReference(p, secret, scheme); err != nil {
				t.Fatal(err)
			}
			before := secret.DeepCopy()
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(p, secret).Build()
			r := &Reconciler{Client: c}
			ctx := context.Background()
			if err := r.ensureKeysSecret(ctx, p); err != nil {
				t.Fatal(err)
			}
			if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
				t.Fatal(err)
			}
			for key, value := range before.Data {
				if !reflect.DeepEqual(secret.Data[key], value) {
					t.Errorf("existing key %s changed", key)
				}
			}
			if existingCookie == "" {
				decoded, err := hex.DecodeString(string(secret.Data[cookieKey]))
				if err != nil || len(decoded) != 28 {
					t.Fatal("generated cookie must contain 28 random bytes encoded as hex")
				}
			}
			stable := secret.DeepCopy()
			if err := r.ensureKeysSecret(ctx, p); err != nil {
				t.Fatal(err)
			}
			if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
				t.Fatal(err)
			}
			if secret.ResourceVersion != stable.ResourceVersion || !reflect.DeepEqual(secret.Data, stable.Data) {
				t.Fatal("reconciliation rotated keys or updated an unchanged Secret")
			}
		})
	}
}
