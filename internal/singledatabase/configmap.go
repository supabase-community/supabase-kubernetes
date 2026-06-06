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

package singledatabase

import (
	"fmt"
	"strconv"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapName returns the name of the ConfigMap for a SingleDatabase.
func ConfigMapName(dbName string) string {
	return fmt.Sprintf("%s-db", dbName)
}

// BuildConfigMap constructs the ConfigMap for a SingleDatabase.
func BuildConfigMap(db *supabasev1alpha1.SingleDatabase) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(db.Name),
			Namespace: db.Namespace,
			Labels:    DefaultLabels(db.Name),
		},
		Data: map[string]string{
			DefaultConfigMapKeyPort:     strconv.Itoa(int(DefaultPort)),
			DefaultConfigMapKeyDatabase: DefaultDatabase,
			DefaultConfigMapKeyUser:     DefaultDatabaseUser,
		},
	}
}
