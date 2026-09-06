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

package storage

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/supabase-community/supabase-kubernetes/internal/helper"
)

// StoragePVCName returns the name of the Storage PersistentVolumeClaim for a Project.
func StoragePVCName(project *ResourceContext) string {
	return ComponentName(project.Names["Storage"], "storage-data")
}

// StoragePVC constructs the PersistentVolumeClaim for the Storage component.
func StoragePVC(project *ResourceContext) (*corev1.PersistentVolumeClaim, error) {
	if project.Spec.Storage == nil {
		return nil, nil
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StoragePVCName(project),
			Namespace: project.Namespace,
			Labels:    StorageLabels(project),
		},
	}
	var err error
	pvc.Spec, err = buildStorageVolumeClaimSpec(project)
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

// buildStorageVolumeClaimSpec returns the PersistentVolumeClaimSpec for the Storage.
func buildStorageVolumeClaimSpec(project *ResourceContext) (corev1.PersistentVolumeClaimSpec, error) {
	base := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			},
		},
	}
	return helper.OverlayPersistentVolumeClaimSpec(base, project.Spec.Storage.Storage)
}
