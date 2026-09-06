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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// PostgresPVCName returns the name of the data PersistentVolumeClaim for a SingleDatabase.
func PostgresPVCName(db *supabasev1alpha1.SingleDatabase) string {
	return fmt.Sprintf("%s-postgres-data", db.Name)
}

// PostgresPVC constructs the PersistentVolumeClaim for a SingleDatabase.
func PostgresPVC(db *supabasev1alpha1.SingleDatabase) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PostgresPVCName(db),
			Namespace: db.Namespace,
			Labels:    PostgresLabels(db),
		},
	}
	var err error
	pvc.Spec, err = buildPostgresVolumeClaimSpec(db)
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

// buildPostgresVolumeClaimSpec returns the PersistentVolumeClaimSpec for the SingleDatabase.
func buildPostgresVolumeClaimSpec(db *supabasev1alpha1.SingleDatabase) (corev1.PersistentVolumeClaimSpec, error) {
	base := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			},
		},
	}
	return helper.OverlayPersistentVolumeClaimSpec(base, db.Spec.Storage)
}
