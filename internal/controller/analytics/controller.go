package analytics

import (
	"context"
	"errors"
	"fmt"

	core "github.com/supabase-community/supabase-kubernetes/api/v1alpha1"
	"github.com/supabase-community/supabase-kubernetes/internal/controller/component"
	"github.com/supabase-community/supabase-kubernetes/internal/defaults"
	analyticsdefaults "github.com/supabase-community/supabase-kubernetes/internal/defaults/analytics"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errAnalyticsDatabaseConflict = errors.New("multiple Analytics resources reference the same database")

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.supabase.io,resources=analytics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.supabase.io,resources=analytics/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.supabase.io,resources=analytics/finalizers,verbs=update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &core.Analytics{}
	project, result, proceed, err := component.Start(ctx, r.Client, req, obj, &core.AnalyticsList{}, "Analytics")
	if err != nil || !proceed {
		return result, err
	}
	if err := r.checkDatabaseConflict(ctx, obj, project); err != nil {
		if !errors.Is(err, errAnalyticsDatabaseConflict) {
			return ctrl.Result{}, err
		}
		if cleanupErr := deleteAnalyticsDeployment(ctx, r.Client, obj); cleanupErr != nil {
			return ctrl.Result{}, cleanupErr
		}
		return component.NotReady(ctx, r.Client, obj, "ConflictingDatabase", err.Error())
	}
	db, result, proceed, err := component.ResolveDatabase(ctx, r.Client, obj, project)
	if err != nil || !proceed {
		return result, err
	}
	state := defaults.NewContext(project)
	state.Owner = obj
	state.Spec.Analytics = &obj.Spec
	state.Names["Analytics"] = obj.Name
	if err := r.reconcileSecret(ctx, state, db); err != nil {
		return component.NotReady(ctx, r.Client, obj, "SecretFailed", err.Error())
	}
	if err := r.reconcileService(ctx, state); err != nil {
		return component.NotReady(ctx, r.Client, obj, "ReconcileFailed", err.Error())
	}
	if err := r.reconcileDeployment(ctx, state, db); err != nil {
		return component.NotReady(ctx, r.Client, obj, "ReconcileFailed", err.Error())
	}
	if err := component.DeploymentReady(ctx, r.Client, client.ObjectKey{Namespace: obj.Namespace, Name: analyticsdefaults.AnalyticsDeploymentName(state)}); err != nil {
		return component.NotReady(ctx, r.Client, obj, "WorkloadNotReady", err.Error())
	}
	return component.Ready(ctx, r.Client, obj)
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return component.SetupWithManager(mgr, r, component.SetupOptions{
		Object:           &core.Analytics{},
		NewList:          func() client.ObjectList { return &core.AnalyticsList{} },
		Owns:             []client.Object{&appsv1.Deployment{}, &corev1.Service{}, &corev1.Secret{}},
		NamespaceRelated: []client.Object{&core.Analytics{}},
	})
}

func (r *Reconciler) checkDatabaseConflict(ctx context.Context, obj *core.Analytics, project *core.Project) error {
	list := &core.AnalyticsList{}
	if err := r.List(ctx, list, client.InNamespace(obj.Namespace)); err != nil {
		return err
	}
	for _, other := range list.Items {
		if other.Name == obj.Name || !other.DeletionTimestamp.IsZero() {
			continue
		}
		p := &core.Project{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: obj.Namespace, Name: other.Spec.ProjectRef.Name}, p); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if p.Spec.DatabaseRef == project.Spec.DatabaseRef {
			return fmt.Errorf("%w: %s", errAnalyticsDatabaseConflict, other.Name)
		}
	}
	return nil
}
