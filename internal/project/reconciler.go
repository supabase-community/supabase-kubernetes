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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	supabasev1alpha1 "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

const (
	// ConditionTypeRestReady indicates the Rest component readiness.
	ConditionTypeRestReady = "RestReady"
	// ConditionTypeAuthReady indicates the Auth component readiness.
	ConditionTypeAuthReady = "AuthReady"
	// ConditionTypeMetaReady indicates the Meta component readiness.
	ConditionTypeMetaReady = "MetaReady"
	// ConditionTypeRealtimeReady indicates the Realtime component readiness.
	ConditionTypeRealtimeReady = "RealtimeReady"

	// AuthPort is the container and service port used by the Auth component.
	AuthPort = 9999
)

// Reconciler holds the dependencies required to reconcile Project sub-components.
type Reconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

// setCondition sets a status condition on the Project.
func (r *Reconciler) setCondition(
	project *supabasev1alpha1.Project,
	conditionType string,
	status metav1.ConditionStatus,
	reason string,
	message string,
) {
	reconciler.SetCondition(project, conditionType, status, reason, message)
}

func namespacedName(name, namespace string) types.NamespacedName {
	return types.NamespacedName{Name: name, Namespace: namespace}
}

func clientIsNotFound(err error) bool {
	return apierrors.IsNotFound(err)
}
