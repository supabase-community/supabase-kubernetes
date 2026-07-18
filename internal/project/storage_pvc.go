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
)

// StoragePVCName returns the name of the Storage PersistentVolumeClaim for a Project.
func StoragePVCName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-storage-data", project.Name)
}

// StoragePVC constructs the PersistentVolumeClaim for the Storage component.
func StoragePVC(project *supabasev1alpha1.Project) (*corev1.PersistentVolumeClaim, error) {
	if project.Spec.Storage == nil || !*project.Spec.Storage.Enable {
		return nil, nil
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StoragePVCName(project),
			Namespace: project.Namespace,
			Labels:    StorageLabels(project),
		},
		Spec: buildStorageVolumeClaimSpec(project),
	}

	return pvc, nil
}

// buildStorageVolumeClaimSpec returns the PersistentVolumeClaimSpec for the Storage.
func buildStorageVolumeClaimSpec(project *supabasev1alpha1.Project) corev1.PersistentVolumeClaimSpec {
	return corev1.PersistentVolumeClaimSpec{
		AccessModes: project.Spec.Storage.Storage.AccessModes,
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: project.Spec.Storage.Storage.Size,
			},
		},
		StorageClassName: project.Spec.Storage.Storage.StorageClassName,
	}
}

// StoragePVCDeletionPolicy returns the deletion policy for the Storage PVC.
func StoragePVCDeletionPolicy(project *supabasev1alpha1.Project) supabasev1alpha1.DeletionPolicy {
	if project.Spec.Storage != nil && project.Spec.Storage.Storage.DeletionPolicy != nil {
		return *project.Spec.Storage.Storage.DeletionPolicy
	}
	return supabasev1alpha1.DeletionPolicyDelete
}
