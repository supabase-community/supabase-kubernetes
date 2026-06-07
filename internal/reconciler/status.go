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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ConditionTypeReady indicates the resource is ready.
	ConditionTypeReady = "Ready"
)

// ConditionedObject represents a Kubernetes resource that carries status conditions.
type ConditionedObject interface {
	client.Object
	GetConditions() *[]metav1.Condition
}

// SetCondition sets a status condition on the provided object.
func SetCondition[T ConditionedObject](obj T, conditionType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(obj.GetConditions(), metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: obj.GetGeneration(),
		Reason:             reason,
		Message:            message,
	})
}

// SetReady sets the Ready condition to True on the provided object.
func SetReady[T ConditionedObject](obj T, reason, message string) {
	SetCondition(obj, ConditionTypeReady, metav1.ConditionTrue, reason, message)
}

// SetNotReady sets the Ready condition to False on the provided object.
func SetNotReady[T ConditionedObject](obj T, reason, message string) {
	SetCondition(obj, ConditionTypeReady, metav1.ConditionFalse, reason, message)
}
