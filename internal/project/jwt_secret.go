/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package project

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

const (
	// DefaultAPIKeyExpiry is the lifetime used for static API keys such as
	// ANON_KEY and SERVICE_ROLE_KEY. It matches the 5-year expiry used by the
	// Supabase self-hosted generate-keys.sh script.
	DefaultAPIKeyExpiry = 5 * 365 * 24 * time.Hour

	// JWTSecretKey is the Secret data key that holds the symmetric JWT secret.
	JWTSecretKey = "jwt-secret"

	// JWTSecretAnonKey is the Secret data key that holds the anonymous role JWT.
	JWTSecretAnonKey = "anon-key"

	// JWTSecretServiceKey is the Secret data key that holds the service_role JWT.
	JWTSecretServiceKey = "service-key"

	// JWTSecretKeys is the Secret data key that holds the JWK set for signing.
	JWTSecretKeys = "jwt-keys"

	// JWTSecretJWKS is the Secret data key that holds the public JWKS.
	JWTSecretJWKS = "jwt-jwks"

	// JWTSecretAnonKeyAsym is the Secret data key that holds the asymmetric anon JWT.
	JWTSecretAnonKeyAsym = "anon-key-asymmetric"

	// JWTSecretServiceKeyAsym is the Secret data key that holds the asymmetric service_role JWT.
	JWTSecretServiceKeyAsym = "service-key-asymmetric"

	// JWTSecretPublishableKey is the Secret data key that holds the publishable API key.
	JWTSecretPublishableKey = "publishable-key"

	// JWTSecretOpaqueKey is the Secret data key that holds the opaque API key.
	JWTSecretOpaqueKey = "secret-key"
)

// JWTSecretName returns the name of the JWT Secret for a Project.
func JWTSecretName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-jwt", project.Name)
}

// JWTSecret constructs the JWT Secret for a Project.
func JWTSecret(project *supabasev1alpha1.Project) (*corev1.Secret, error) {
	jwtSecretHex, err := helper.GenerateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generating JWT secret bytes: %w", err)
	}

	jwtSecretBytes, err := hex.DecodeString(jwtSecretHex)
	if err != nil {
		return nil, fmt.Errorf("decoding JWT secret hex: %w", err)
	}

	jwtSecret := base64.StdEncoding.EncodeToString(jwtSecretBytes)

	now := time.Now()
	apiKeyExpiry := DefaultAPIKeyExpiry

	anonKey, err := helper.GenerateJWTToken(jwtSecret, "anon", now, apiKeyExpiry)
	if err != nil {
		return nil, fmt.Errorf("generating anon key: %w", err)
	}

	serviceKey, err := helper.GenerateJWTToken(jwtSecret, "service_role", now, apiKeyExpiry)
	if err != nil {
		return nil, fmt.Errorf("generating service key: %w", err)
	}

	ecKey, err := helper.GenerateECP256Keypair()
	if err != nil {
		return nil, fmt.Errorf("generating EC keypair: %w", err)
	}

	kid, err := helper.GenerateRandomHex(16)
	if err != nil {
		return nil, fmt.Errorf("generating key id: %w", err)
	}

	ecPrivateJWK := helper.ECPrivateKeyToJWK(ecKey, kid)
	ecPublicJWK := helper.ECPublicKeyToJWK(ecKey, kid)
	octJWK := helper.SymmetricKeyToJWK(jwtSecret)

	jwtKeys, err := helper.BuildJWTKeys(ecPrivateJWK, octJWK)
	if err != nil {
		return nil, fmt.Errorf("building JWT keys: %w", err)
	}

	jwtJWKS, err := helper.BuildJWTJWKS(ecPublicJWK, octJWK)
	if err != nil {
		return nil, fmt.Errorf("building JWT JWKS: %w", err)
	}

	anonKeyAsym, err := helper.SignES256JWT(ecKey, kid, "anon", now, apiKeyExpiry)
	if err != nil {
		return nil, fmt.Errorf("generating asymmetric anon key: %w", err)
	}

	serviceKeyAsym, err := helper.SignES256JWT(ecKey, kid, "service_role", now, apiKeyExpiry)
	if err != nil {
		return nil, fmt.Errorf("generating asymmetric service key: %w", err)
	}

	publishableKey, err := helper.GenerateOpaqueKey("sb_publishable_")
	if err != nil {
		return nil, fmt.Errorf("generating publishable key: %w", err)
	}

	secretKey, err := helper.GenerateOpaqueKey("sb_secret_")
	if err != nil {
		return nil, fmt.Errorf("generating secret key: %w", err)
	}

	sc := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JWTSecretName(project),
			Namespace: project.Namespace,
			Labels:    ProjectLabels(project),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			JWTSecretKey:            []byte(jwtSecret),
			JWTSecretAnonKey:        []byte(anonKey),
			JWTSecretServiceKey:     []byte(serviceKey),
			JWTSecretKeys:           []byte(jwtKeys),
			JWTSecretJWKS:           []byte(jwtJWKS),
			JWTSecretAnonKeyAsym:    []byte(anonKeyAsym),
			JWTSecretServiceKeyAsym: []byte(serviceKeyAsym),
			JWTSecretPublishableKey: []byte(publishableKey),
			JWTSecretOpaqueKey:      []byte(secretKey),
		},
	}

	return sc, nil
}
