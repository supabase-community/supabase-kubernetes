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
	// AuthSecretSAMLPrivateKey is the Secret data key that holds the SAML private key.
	AuthSecretSAMLPrivateKey = "saml-private-key"
)

// AuthSecretName returns the name of the Auth Secret for a Project.
func AuthSecretName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-auth", project.Name)
}

// AuthSecret constructs the Auth Secret for a Project.
func AuthSecret(project *supabasev1alpha1.Project) (*corev1.Secret, error) {
	if project.Spec.Auth == nil || !*project.Spec.Auth.Enable {
		return nil, nil
	}
	if project.Spec.Auth.SAML == nil || !*project.Spec.Auth.SAML.Enable {
		return nil, nil
	}

	samlPrivateKey, err := helper.GenerateSAMLPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generating SAML private key: %w", err)
	}

	sc := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AuthSecretName(project),
			Namespace: project.Namespace,
			Labels:    AuthLabels(project),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			AuthSecretSAMLPrivateKey: []byte(samlPrivateKey),
		},
	}

	return sc, nil
}
