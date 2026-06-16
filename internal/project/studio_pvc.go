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

// StudioPVCName returns the name of the Studio PersistentVolumeClaim for a Project.
func StudioPVCName(project *supabasev1alpha1.Project) string {
	return fmt.Sprintf("%s-studio-data", project.Name)
}

// StudioPVC constructs the PersistentVolumeClaim for a Project.
func StudioPVC(project *supabasev1alpha1.Project) (*corev1.PersistentVolumeClaim, error) {
	if project.Spec.Studio == nil || !*project.Spec.Studio.Enable {
		return nil, nil
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StudioPVCName(project),
			Namespace: project.Namespace,
			Labels:    StudioLabels(project),
		},
		Spec: buildStudioVolumeClaimSpec(project),
	}

	return pvc, nil
}

// buildStudioVolumeClaimSpec returns the PersistentVolumeClaimSpec for the Studio.
func buildStudioVolumeClaimSpec(project *supabasev1alpha1.Project) corev1.PersistentVolumeClaimSpec {
	return corev1.PersistentVolumeClaimSpec{
		AccessModes: project.Spec.Studio.Storage.AccessModes,
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: project.Spec.Studio.Storage.Size,
			},
		},
		StorageClassName: project.Spec.Studio.Storage.StorageClassName,
	}
}

// StudioPVCDeletionPolicy returns the deletion policy for the Studio PVC.
func StudioPVCDeletionPolicy(project *supabasev1alpha1.Project) supabasev1alpha1.DeletionPolicy {
	if project.Spec.Studio != nil && project.Spec.Studio.Storage.DeletionPolicy != nil {
		return *project.Spec.Studio.Storage.DeletionPolicy
	}
	return supabasev1alpha1.DeletionPolicyDelete
}
