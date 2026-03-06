// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package database

import (
	"context"
	"fmt"
	"time"

	"errors"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	sqlClient "github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/health"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientpkg "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SqlOptions struct {
	RequeueInterval time.Duration
	LogSql          bool
}

type SqlOpt func(*SqlOptions)

func WithRequeueInterval(interval time.Duration) SqlOpt {
	return func(opts *SqlOptions) {
		opts.RequeueInterval = interval
	}
}

func WithLogSql(logSql bool) SqlOpt {
	return func(opts *SqlOptions) {
		opts.LogSql = logSql
	}
}

type SqlReconciler struct {
	Client         client.Client
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready

	WrappedReconciler WrappedReconciler
	Finalizer         Finalizer

	SqlOptions
}

func NewSqlReconciler(client client.Client, cr *conditions.Ready, wr WrappedReconciler, f Finalizer,
	opts ...SqlOpt) Reconciler {
	reconciler := &SqlReconciler{
		Client:            client,
		RefResolver:       refresolver.New(client),
		ConditionReady:    cr,
		WrappedReconciler: wr,
		Finalizer:         f,
		SqlOptions: SqlOptions{
			RequeueInterval: 30 * time.Second,
			LogSql:          false,
		},
	}
	for _, setOpt := range opts {
		setOpt(&reconciler.SqlOptions)
	}
	return reconciler
}

func (r *SqlReconciler) Reconcile(ctx context.Context, resource Resource) (ctrl.Result, error) {
	if resource.IsBeingDeleted() {
		if result, err := r.Finalizer.Finalize(ctx, resource); !result.IsZero() || err != nil {
			return result, err
		}
		return ctrl.Result{}, nil
	}

	doltdb, err := r.RefResolver.DoltDB(ctx, resource.DoltDBRef(), resource.GetNamespace())
	if err != nil {
		patchErr := r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherRefResolver(err, doltdb))
		return ctrl.Result{}, fmt.Errorf("error getting DoltDB: %v", errors.Join(err, patchErr))
	}

	if result, err := WaitForDoltDB(ctx, r.Client, doltdb, r.LogSql); !result.IsZero() || err != nil {
		if err != nil {
			patchErr := r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherWithError(err))
			return result, errors.Join(err, patchErr)
		}

		return result, nil
	}

	doltdbClient, err := sqlClient.NewClientWithDoltDB(ctx, doltdb, r.RefResolver)
	if err != nil {
		msg := fmt.Sprintf("Error connecting to DoltDB: %v", err)
		patchErr := r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherFailed(msg))

		return r.retryResult(ctx, resource, errors.Join(err, patchErr))
	}
	defer func() {
		if err := doltdbClient.Close(); err != nil {
			log.FromContext(ctx).Error(err, "error closing DoltDB client")
		}
	}()

	reconcileErr := r.WrappedReconciler.Reconcile(ctx, doltdbClient)

	if reconcileErr != nil {
		msg := fmt.Sprintf("Error creating %s: %v", resource.GetName(), reconcileErr)
		patchErr := r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherFailed(msg))

		return r.retryResult(ctx, resource, errors.Join(reconcileErr, patchErr))
	}

	finalizerErr := r.Finalizer.AddFinalizer(ctx)
	if finalizerErr != nil {
		finalizerErr = fmt.Errorf("error adding finalizer to %s: %v", resource.GetName(), finalizerErr)
	}

	patchErr := r.WrappedReconciler.PatchStatus(ctx, r.ConditionReady.PatcherWithError(finalizerErr))

	return r.requeueResult(ctx, resource, errors.Join(finalizerErr, patchErr))
}

func (r *SqlReconciler) retryResult(ctx context.Context, resource Resource, err error) (ctrl.Result, error) {
	if resource.RetryInterval() != nil {
		log.FromContext(ctx).Error(err, "Error reconciling SQL resource", "resource", resource.GetName())
		return ctrl.Result{RequeueAfter: resource.RetryInterval().Duration}, nil
	}
	if err != nil {
		if r.LogSql {
			log.FromContext(ctx).V(1).Info("Error reconciling SQL resource", "err", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func (r *SqlReconciler) requeueResult(ctx context.Context, resource Resource, err error) (ctrl.Result, error) {
	if err != nil {
		log.FromContext(ctx).V(1).Info("Error reconciling SQL resource", "err", err)
		return ctrl.Result{Requeue: true}, nil
	}
	if resource.RequeueInterval() != nil {
		if r.LogSql {
			log.FromContext(ctx).V(1).Info("Requeuing SQL resource")
		}
		return ctrl.Result{RequeueAfter: resource.RequeueInterval().Duration}, nil
	}
	if r.RequeueInterval > 0 {
		if r.LogSql {
			log.FromContext(ctx).V(1).Info("Requeuing SQL resource")
		}
		return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
	}
	return ctrl.Result{}, nil
}

func WaitForDoltDB(ctx context.Context, client client.Client, doltdb *doltv1alpha.DoltDB,
	logSql bool) (ctrl.Result, error) {
	healthy, err := health.IsStatefulSetHealthy(
		ctx,
		client,
		clientpkg.ObjectKeyFromObject(doltdb),
		doltdb.InternalServiceKey(),
		health.WithDesiredReplicas(doltdb.Spec.Replicas),
		health.WithPort(doltdb.Spec.Server.Listener.Port),
		health.WithEndpointPolicy(health.EndpointPolicyAll),
	)

	if err != nil {
		return ctrl.Result{}, err
	}
	if !healthy {
		if logSql {
			log.FromContext(ctx).V(1).Info("DoltDB unhealthy. Requeuing SQL resource")
		}
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}
