package rbac

import (
	"context"
	"fmt"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	"strings"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Reconciler is responsible for reconciling RBAC resources.
type Reconciler struct {
	client.Client
	builder *builder.Builder
}

// NewReconciler creates a new RBACReconciler.
func NewReconciler(client client.Client, builder *builder.Builder) *Reconciler {
	return &Reconciler{
		Client:  client,
		builder: builder,
	}
}

// ReconcileServiceAccount ensures that a ServiceAccount exists for the given DoltDB.
func (r *Reconciler) ReconcileServiceAccount(ctx context.Context, key types.NamespacedName, doltdb *doltv1alpha.DoltDB) (*corev1.ServiceAccount, error) {
	var existingSA corev1.ServiceAccount
	err := r.Get(ctx, key, &existingSA)
	if err == nil {
		return &existingSA, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("error getting ServiceAccount: %v", err)
	}

	sa, err := r.builder.BuildServiceAccount(key, doltdb)
	if err != nil {
		return nil, fmt.Errorf("error building ServiceAccount: %v", err)
	}
	if err := r.Create(ctx, sa); err != nil {
		return nil, fmt.Errorf("error creating ServiceAccount: %v", err)
	}
	return sa, nil
}

// ReconcileDoltRBAC ensures that all necessary RBAC resources exist for the given DoltDB.
func (r *Reconciler) ReconcileDoltRBAC(ctx context.Context, doltdb *doltv1alpha.DoltDB) error {
	key := doltdb.ServiceAccountKey()
	sa, err := r.ReconcileServiceAccount(ctx, key, doltdb)
	if err != nil {
		return fmt.Errorf("error reconciling ServiceAccount: %v", err)
	}

	role, err := r.reconcileRole(ctx, key, doltdb)
	if err != nil {
		return fmt.Errorf("error reconciling Role: %v", err)
	}

	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}
	if err := r.reconcileRoleBinding(ctx, key, doltdb, sa, roleRef); err != nil {
		return fmt.Errorf("error reconciling RoleBinding: %v", err)
	}

	authDelegatorRoleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     "system:auth-delegator",
	}
	authDelegatorKey := types.NamespacedName{
		Name:      fmt.Sprintf("%s:auth-delegator", authDelegatorRoleNameOrDefault(doltdb)),
		Namespace: doltdb.Namespace,
	}
	if err := r.reconcileClusterRoleBinding(ctx, authDelegatorKey, doltdb, sa, authDelegatorRoleRef); err != nil {
		return fmt.Errorf("error reconciling system:auth-delegator ClusterRoleBinding: %v", err)
	}

	return nil
}

// reconcileRole ensures that a Role exists for the given DoltDB.
func (r *Reconciler) reconcileRole(ctx context.Context, key types.NamespacedName, doltdb *doltv1alpha.DoltDB) (*rbacv1.Role, error) {
	var existingRole rbacv1.Role
	err := r.Get(ctx, key, &existingRole)
	if err == nil {
		return &existingRole, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("error getting Role: %v", err)
	}

	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				doltv1alpha.GroupVersion.Group,
			},
			Resources: []string{
				"doltdbs",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				corev1.GroupName,
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				volumesnapshotv1.GroupName,
			},
			Resources: []string{
				"volumesnapshots",
			},
			Verbs: []string{
				"get", "list", "watch", "create", "update", "patch", "delete",
			},
		},
	}

	role, err := r.builder.BuildRole(key, doltdb, rules)
	if err != nil {
		return nil, fmt.Errorf("error building Role: %v", err)
	}
	if err := r.Create(ctx, role); err != nil {
		return nil, fmt.Errorf("error creating Role: %v", err)
	}
	return role, nil
}

// reconcileRoleBinding ensures that a RoleBinding exists for the given DoltDB.
func (r *Reconciler) reconcileRoleBinding(ctx context.Context, key types.NamespacedName, doltdb *doltv1alpha.DoltDB, sa *corev1.ServiceAccount, roleRef rbacv1.RoleRef) error {
	var existingRB rbacv1.RoleBinding
	err := r.Get(ctx, key, &existingRB)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("error getting RoleBinding: %v", err)
	}

	rb, err := r.builder.BuildRoleBinding(key, doltdb, sa, roleRef)
	if err != nil {
		return fmt.Errorf("error building RoleBinding: %v", err)
	}
	if err := r.Create(ctx, rb); err != nil {
		return fmt.Errorf("error creating RoleBinding: %v", err)
	}
	return nil
}

// reconcileClusterRoleBinding ensures that a ClusterRoleBinding exists for the given DoltDB.
func (r *Reconciler) reconcileClusterRoleBinding(ctx context.Context, key types.NamespacedName, doltdb *doltv1alpha.DoltDB, sa *corev1.ServiceAccount, roleRef rbacv1.RoleRef) error {
	var existingCRB rbacv1.ClusterRoleBinding
	err := r.Get(ctx, key, &existingCRB)
	if err == nil {
		if !isOwnedBy(doltdb, &existingCRB) {
			return fmt.Errorf(
				"ClusterRoleBinding '%s' already exists",
				existingCRB.Name,
			)
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("error getting ClusterRoleBinding: %v", err)
	}

	crdb, err := r.builder.BuildClusterRoleBinding(key, doltdb, sa, roleRef)
	if err != nil {
		return fmt.Errorf("error building ClusterRoleBinding: %v", err)
	}
	if err := r.Create(ctx, crdb); err != nil {
		return fmt.Errorf("error creating ClusterRoleBinding: %v", err)
	}
	return nil
}

// isOwnedBy checks if the child object is owned by the owner object.
func isOwnedBy(owner client.Object, child client.Object) bool {
	ownerReferences := child.GetOwnerReferences()
	for _, ownerRef := range ownerReferences {
		if ownerRef.UID == owner.GetUID() {
			return true
		}
	}
	return false
}

// authDelegatorRoleNameOrDefault defines the ClusterRoleBinding name bound to system:auth-delegator.
// It falls back to the DoltDB name if AuthDelegatorRoleName is not set.
func authDelegatorRoleNameOrDefault(doltdb *doltv1alpha.DoltDB) string {
	name := fmt.Sprintf("%s-%s", doltdb.Name, doltdb.Namespace)
	parts := strings.Split(string(doltdb.UID), "-")
	if len(parts) > 0 {
		name += fmt.Sprintf("-%s", parts[0])
	}
	return name
}
