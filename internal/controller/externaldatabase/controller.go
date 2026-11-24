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

package externaldatabase

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/reconciler"
)

// Reconciler resolves an ExternalDatabase without modifying the database or its Secret.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&core.ExternalDatabase{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.mapSecretToDatabases)).
		Named("externaldatabase").
		Complete(r)
}

func (r *Reconciler) mapSecretToDatabases(ctx context.Context, obj client.Object) []reconcile.Request {
	databases := &core.ExternalDatabaseList{}
	if err := r.List(ctx, databases, client.InNamespace(obj.GetNamespace())); err != nil {
		log.FromContext(ctx).Error(err, "Failed to list ExternalDatabases for Secret")
		return nil
	}
	var requests []reconcile.Request
	for _, db := range databases.Items {
		if db.Spec.PasswordRef.Name == obj.GetName() {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&db)})
		}
	}
	return requests
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=externaldatabases,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.supabase.io,resources=externaldatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile publishes connection parameters after resolving the password reference.
// Ready describes configuration availability, not database health or compatibility.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting ExternalDatabase reconciliation")
	db := &core.ExternalDatabase{}
	if err := r.Get(ctx, req.NamespacedName, db); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !db.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	secret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: db.Spec.PasswordRef.Name}, secret)
	if apierrors.IsNotFound(err) {
		return r.notReady(ctx, db, "SecretNotFound", "Referenced password Secret does not exist")
	}
	if err != nil {
		logger.Error(err, "Failed to get Secret")
		_, statusErr := r.notReady(ctx, db, "SecretReadFailed", "Could not read referenced password Secret")
		if statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{}, err
	}
	if !secret.DeletionTimestamp.IsZero() {
		return r.notReady(ctx, db, "SecretNotFound", "Referenced password Secret is deleting")
	}
	if _, ok := secret.Data[db.Spec.PasswordRef.Key]; !ok {
		return r.notReady(ctx, db, "SecretKeyNotFound", fmt.Sprintf("Referenced password Secret does not contain key %q", db.Spec.PasswordRef.Key))
	}
	db.Status.ResolvedDatabase = &core.ResolvedDatabase{
		Host: db.Spec.Host, Port: db.Spec.Port, DBName: db.Spec.DBName,
		User: db.Spec.User, PasswordRef: db.Spec.PasswordRef,
	}
	reconciler.SetReady(db, "ReconcileSucceeded", "Database connection parameters and password reference resolved")
	return ctrl.Result{}, reconciler.UpdateStatus(ctx, r.Client, db)
}

func (r *Reconciler) notReady(ctx context.Context, db *core.ExternalDatabase, reason, message string) (ctrl.Result, error) {
	db.Status.ResolvedDatabase = nil
	reconciler.SetNotReady(db, reason, message)
	return ctrl.Result{}, reconciler.UpdateStatus(ctx, r.Client, db)
}
