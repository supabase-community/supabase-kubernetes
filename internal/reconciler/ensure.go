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

package reconciler

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Result represents the outcome of an EnsureResource call.
type Result string

const (
	// ResultCreated indicates the resource was created.
	ResultCreated Result = "Created"
	// ResultUpdated indicates the resource was updated.
	ResultUpdated Result = "Updated"
	// ResultUnchanged indicates the resource was already up to date.
	ResultUnchanged Result = "Unchanged"
)

// EnsureResource guarantees that the desired resource exists in the cluster.
// If the resource does not exist, it is created.
// If the resource exists, mutateFn is called with:
//   - existing: the object loaded from the cluster (already modified by Get)
//   - desired:  a copy of the object originally passed by the controller
//
// The object is then updated if any changes were made.
// When owner is not nil, SetControllerReference is applied automatically.
func EnsureResource(
	ctx context.Context,
	c client.Client,
	desired client.Object,
	owner client.Object,
	mutateFn func(existing client.Object, desired client.Object) error,
) (Result, error) {
	logger := log.FromContext(ctx).WithValues(
		"kind", fmt.Sprintf("%T", desired),
		"name", desired.GetName(),
		"namespace", desired.GetNamespace(),
	)

	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, desired, c.Scheme()); err != nil {
			return "", fmt.Errorf("setting owner reference: %w", err)
		}
	}

	// Keep a copy of the original desired state because the pointer passed to
	// CreateOrUpdate is overwritten when the object already exists in the cluster.
	original := desired.DeepCopyObject().(client.Object)

	result, err := controllerutil.CreateOrUpdate(ctx, c, desired, func() error {
		return mutateFn(desired, original)
	})
	if err != nil {
		return "", err
	}

	switch result {
	case controllerutil.OperationResultCreated:
		logger.Info("Created resource")
		return ResultCreated, nil
	case controllerutil.OperationResultUpdated:
		logger.V(1).Info("Updated resource")
		return ResultUpdated, nil
	default:
		logger.V(1).Info("Resource unchanged")
		return ResultUnchanged, nil
	}
}
