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

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PVCName returns the name of the data PersistentVolumeClaim for a SingleDatabase.
func PVCName(dbName string) string {
	return fmt.Sprintf("%s-db", dbName)
}

// BuildPVC constructs the PersistentVolumeClaim for a SingleDatabase.
func BuildPVC(db *supabasev1alpha1.SingleDatabase) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PVCName(db.Name),
			Namespace: db.Namespace,
			Labels:    DefaultLabels(db.Name),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: db.Spec.Storage.StorageClassName,
			AccessModes:      AccessModes(db),
			Resources:        StorageResources(db),
		},
	}
}

// AccessModes returns the desired access modes.
func AccessModes(db *supabasev1alpha1.SingleDatabase) []corev1.PersistentVolumeAccessMode {
	return db.Spec.Storage.AccessModes
}

// StorageResources returns the PVC resource requirements from the spec size.
func StorageResources(db *supabasev1alpha1.SingleDatabase) corev1.VolumeResourceRequirements {
	return corev1.VolumeResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: db.Spec.Storage.Size,
		},
	}
}
