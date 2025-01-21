package database

import (
	"context"
	"fmt"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	sqlClient "github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SqlFinalizer struct {
	Client      client.Client
	RefResolver *refresolver.RefResolver

	WrappedFinalizer WrappedFinalizer

	SqlOptions
}

func NewSqlFinalizer(client client.Client, wf WrappedFinalizer, opts ...SqlOpt) Finalizer {
	finalizer := &SqlFinalizer{
		Client:           client,
		RefResolver:      refresolver.New(client),
		WrappedFinalizer: wf,
		SqlOptions: SqlOptions{
			RequeueInterval: 30 * time.Second,
			LogSql:          false,
		},
	}
	for _, setOpt := range opts {
		setOpt(&finalizer.SqlOptions)
	}
	return finalizer
}

func (tf *SqlFinalizer) AddFinalizer(ctx context.Context) error {
	if tf.WrappedFinalizer.ContainsFinalizer() {
		return nil
	}
	if err := tf.WrappedFinalizer.AddFinalizer(ctx); err != nil {
		return fmt.Errorf("error adding finalizer in TemplateFinalizer: %v", err)
	}
	return nil
}

func (tf *SqlFinalizer) Finalize(ctx context.Context, resource Resource) (ctrl.Result, error) {
	if !tf.WrappedFinalizer.ContainsFinalizer() {
		return ctrl.Result{}, nil
	}

	doltdb, err := tf.RefResolver.DoltDB(ctx, resource.DoltDBRef(), resource.GetNamespace())
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := tf.WrappedFinalizer.RemoveFinalizer(ctx); err != nil {
				return ctrl.Result{}, fmt.Errorf("error removing %s finalizer: %v", resource.GetName(), err)
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("error getting doltdb: %v", err)
	}

	if result, err := WaitForDoltDB(ctx, tf.Client, doltdb, tf.LogSql); !result.IsZero() || err != nil {
		return result, err
	}

	doltdbClient, err := sqlClient.NewClientWithDoltDB(ctx, doltdb, tf.RefResolver)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error connecting to DoltDB: %v", err)
	}
	defer func() {
		if err := doltdbClient.Close(); err != nil {
			log.FromContext(ctx).Error(err, "error closing DoltDB client")
		}
	}()

	cleanupPolicy := ptr.Deref(resource.CleanupPolicy(), doltv1alpha.CleanupPolicyDelete)
	if cleanupPolicy == doltv1alpha.CleanupPolicyDelete {
		log.FromContext(ctx).Info("Cleaning up SQL resource")

		if err := tf.WrappedFinalizer.Reconcile(ctx, doltdbClient); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling in TemplateFinalizer: %v", err)
		}
	}

	if err := tf.WrappedFinalizer.RemoveFinalizer(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("error removing finalizer in TemplateFinalizer: %v", err)
	}
	return ctrl.Result{}, nil
}
