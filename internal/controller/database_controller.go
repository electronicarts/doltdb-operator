/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/controller/database"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	databaseFinalizerName = "database.k8s.dolthub.com/finalizer"
)

// DatabaseReconciler reconciles a Database object
type DatabaseReconciler struct {
	client.Client
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready
	SqlOpts        []database.SqlOpt
	Scheme         *runtime.Scheme
}

func NewDatabaseReconciler(client client.Client, refResolver *refresolver.RefResolver, conditionReady *conditions.Ready,
	sqlOpts ...database.SqlOpt) *DatabaseReconciler {
	return &DatabaseReconciler{
		Client:         client,
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
		SqlOpts:        sqlOpts,
	}
}

// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=databases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=databases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=databases/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var db doltv1alpha.Database
	if err := r.Get(ctx, req.NamespacedName, &db); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.WithValues("namespace", req.NamespacedName, "database", db.Spec.Name).
		Info("Running reconciler for Database")

	dbReconciler := newWrappedDatabaseReconciler(r.Client, r.RefResolver, &db)
	dbFinalizer := newWrappedDatabaseFinalizer(r.Client, &db)
	finalizerCtrl := database.NewSqlFinalizer(r.Client, dbFinalizer, r.SqlOpts...)
	dbCtrl := database.NewSqlReconciler(r.Client, r.ConditionReady, dbReconciler, finalizerCtrl, r.SqlOpts...)

	result, err := dbCtrl.Reconcile(ctx, &db)
	if err != nil {
		return result, fmt.Errorf("error reconciling in TemplateReconciler: %v", err)
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&doltv1alpha.Database{}).
		Complete(r)
}

type wrappedDatabaseReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	database    *doltv1alpha.Database
}

func newWrappedDatabaseReconciler(client client.Client, refResolver *refresolver.RefResolver,
	database *doltv1alpha.Database) database.WrappedReconciler {
	return &wrappedDatabaseReconciler{
		Client:      client,
		refResolver: refResolver,
		database:    database,
	}
}

func (wr *wrappedDatabaseReconciler) Reconcile(ctx context.Context, doltdbClient *sql.Client) error {
	opts := sql.DatabaseOpts{
		CharSet:   wr.database.Spec.CharSet,
		Collation: wr.database.Spec.Collation,
	}
	log := log.FromContext(ctx)

	log.V(0).Info("Creating database", "database", wr.database.Name())
	if err := doltdbClient.CreateDatabase(ctx, wr.database.Name(), opts); err != nil {
		return fmt.Errorf("error creating database in DoltDB: %v", err)
	}

	log.V(0).Info("Setting database session", "database", wr.database.Name())
	if err := doltdbClient.UseDatabase(ctx, wr.database.Name()); err != nil {
		return fmt.Errorf("error calling use database in DoltDB: %v", err)
	}
	log.V(0).Info("Creating dolt ignore", "patterns", wr.database.Spec.DoltIgnorePatterns)
	if err := doltdbClient.CreateDoltIgnore(
		ctx,
		wr.database.Spec.DoltIgnorePatterns,
	); err != nil {
		return fmt.Errorf("error creating dolt_ignore in DoltDB: %v", err)
	}

	log.V(0).Info("Creating branches", "branches", wr.database.Spec.SystemBranches)
	// The order of these operations is important because
	// CreateBranches causes side effects on the database connections
	// as explained here: https://docs.dolthub.com/sql-reference/version-control/dolt-sql-procedures#dolt_checkout
	if err := doltdbClient.CreateBranches(
		ctx,
		wr.database.Spec.SystemBranches,
	); err != nil {
		return fmt.Errorf("error creating branches in DoltDB: %v", err)
	}

	return nil
}

func (wr *wrappedDatabaseReconciler) PatchStatus(ctx context.Context, patcher conditions.Patcher) error {
	patch := client.MergeFrom(wr.database.DeepCopy())
	patcher(&wr.database.Status)

	if err := wr.Client.Status().Patch(ctx, wr.database, patch); err != nil {
		return fmt.Errorf("error patching Database status: %v", err)
	}
	return nil
}

type wrappedDatabaseFinalizer struct {
	client.Client
	database *doltv1alpha.Database
}

func newWrappedDatabaseFinalizer(client client.Client, database *doltv1alpha.Database) database.WrappedFinalizer {
	return &wrappedDatabaseFinalizer{
		Client:   client,
		database: database,
	}
}

func (wf *wrappedDatabaseFinalizer) AddFinalizer(ctx context.Context) error {
	if wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.database, func(database *doltv1alpha.Database) {
		controllerutil.AddFinalizer(database, databaseFinalizerName)
	})
}

func (wf *wrappedDatabaseFinalizer) RemoveFinalizer(ctx context.Context) error {
	if !wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.database, func(database *doltv1alpha.Database) {
		controllerutil.RemoveFinalizer(database, databaseFinalizerName)
	})
}

func (wr *wrappedDatabaseFinalizer) ContainsFinalizer() bool {
	return controllerutil.ContainsFinalizer(wr.database, databaseFinalizerName)
}

func (wf *wrappedDatabaseFinalizer) Reconcile(ctx context.Context, doltdbClient *sql.Client) error {
	if err := doltdbClient.DropDatabase(ctx, wf.database.Name()); err != nil {
		return fmt.Errorf("error dropping database in DoltDB: %v", err)
	}
	return nil
}

func (wr *wrappedDatabaseFinalizer) patch(ctx context.Context, database *doltv1alpha.Database,
	patchFn func(*doltv1alpha.Database)) error {
	patch := ctrlClient.MergeFrom(database.DeepCopy())
	patchFn(database)

	if err := wr.Client.Patch(ctx, database, patch); err != nil {
		return fmt.Errorf("error patching Database finalizer: %v", err)
	}
	return nil

}
