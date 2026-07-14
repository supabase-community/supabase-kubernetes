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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

const (
	// KeysSecretSecretKeyBase is the Secret data key that holds the Rails secret key base.
	KeysSecretSecretKeyBase = "secret-key-base"

	// KeysSecretCryptoKey is the Secret data key that holds the crypto key.
	KeysSecretCryptoKey = "crypto-key"

	// KeysSecretVaultEncKey is the Secret data key that holds the Vault encryption key.
	KeysSecretVaultEncKey = "vault-enc-key"

	// KeysSecretRealtimeDBEncKey is the Secret data key that holds the Realtime DB encryption key.
	KeysSecretRealtimeDBEncKey = "realtime-db-enc-key"
)

// KeysSecretName returns the name of the Keys Secret for a Project.
func KeysSecretName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-keys", project.Name)
}

// KeysSecret constructs the Keys Secret for a Project.
func KeysSecret(project *supabasev1alpha1.Project) (*corev1.Secret, error) {
	secretKeyBase, err := helper.GenerateRandomHex(64)
	if err != nil {
		return nil, fmt.Errorf("generating secret key base: %w", err)
	}

	cryptoKey, err := helper.GenerateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generating crypto key: %w", err)
	}

	vaultEncKey, err := helper.GenerateRandomHex(16)
	if err != nil {
		return nil, fmt.Errorf("generating vault encryption key: %w", err)
	}

	realtimeDBEncKey, err := helper.GenerateRandomHex(8)
	if err != nil {
		return nil, fmt.Errorf("generating realtime db encryption key: %w", err)
	}

	sc := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      KeysSecretName(project),
			Namespace: project.Namespace,
			Labels:    ProjectLabels(project),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			KeysSecretSecretKeyBase:    []byte(secretKeyBase),
			KeysSecretCryptoKey:        []byte(cryptoKey),
			KeysSecretVaultEncKey:      []byte(vaultEncKey),
			KeysSecretRealtimeDBEncKey: []byte(realtimeDBEncKey),
		},
	}

	return sc, nil
}
