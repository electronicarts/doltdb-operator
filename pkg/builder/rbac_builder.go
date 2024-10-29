package builder

import (
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// BuildServiceAccount creates a ServiceAccount object and sets the controller reference.
func (b *Builder) BuildServiceAccount(key types.NamespacedName, doltdb *doltv1alpha.DoltCluster) (*corev1.ServiceAccount, error) {
	objMeta :=
		NewMetadataBuilder(key).
			WithMetadata(&doltdb.ObjectMeta).
			Build()
	sa := &corev1.ServiceAccount{
		ObjectMeta: objMeta,
	}
	if err := controllerutil.SetControllerReference(doltdb, sa, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ServiceAccount: %v", err)
	}
	return sa, nil
}

// BuildRole creates a Role object with the specified rules and sets the controller reference.
func (b *Builder) BuildRole(key types.NamespacedName, doltdb *doltv1alpha.DoltCluster, rules []rbacv1.PolicyRule) (*rbacv1.Role, error) {
	objMeta :=
		NewMetadataBuilder(key).
			WithMetadata(&doltdb.ObjectMeta).
			Build()

	r := &rbacv1.Role{
		ObjectMeta: objMeta,
		Rules:      rules,
	}
	if err := controllerutil.SetControllerReference(doltdb, r, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Role: %v", err)
	}
	return r, nil
}

// BuildRoleBinding creates a RoleBinding object that binds the specified ServiceAccount to the RoleRef and sets the controller reference.
func (b *Builder) BuildRoleBinding(key types.NamespacedName, doltdb *doltv1alpha.DoltCluster, sa *corev1.ServiceAccount,
	roleRef rbacv1.RoleRef) (*rbacv1.RoleBinding, error) {
	objMeta :=
		NewMetadataBuilder(key).
			WithMetadata(&doltdb.ObjectMeta).
			Build()
	rb := &rbacv1.RoleBinding{
		ObjectMeta: objMeta,
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.GroupName,
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
		RoleRef: roleRef,
	}
	if err := controllerutil.SetControllerReference(doltdb, rb, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to RoleBinding: %v", err)
	}
	return rb, nil
}

// BuildClusterRoleBinding creates a ClusterRoleBinding object that binds the specified ServiceAccount to the RoleRef and sets the controller reference.
func (b *Builder) BuildClusterRoleBinding(key types.NamespacedName, doltdb *doltv1alpha.DoltCluster, sa *corev1.ServiceAccount,
	roleRef rbacv1.RoleRef) (*rbacv1.ClusterRoleBinding, error) {
	objMeta :=
		NewMetadataBuilder(key).
			WithMetadata(&doltdb.ObjectMeta).
			Build()
	rb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: objMeta,
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.GroupName,
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
		RoleRef: roleRef,
	}
	if err := controllerutil.SetControllerReference(doltdb, rb, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ClusterRoleBinding: %v", err)
	}
	return rb, nil
}
