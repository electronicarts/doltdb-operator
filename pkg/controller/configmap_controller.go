package controller

import (
	"context"
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConfigMapReconciler is responsible for reconciling ConfigMaps.
type ConfigMapReconciler struct {
	client.Client
	Builder *builder.Builder
}

// NewConfigMapReconciler creates a new instance of ConfigMapReconciler.
func NewConfigMapReconciler(client client.Client, builder *builder.Builder) *ConfigMapReconciler {
	return &ConfigMapReconciler{
		Client:  client,
		Builder: builder,
	}
}

// ReconcileRequest contains the information needed to reconcile a ConfigMap.
type ConfigMapReconcileRequest struct {
	Metadata *metav1.ObjectMeta
	Owner    *doltv1alpha.DoltDB
	Key      types.NamespacedName
	Data     map[string]string
}

// Reconcile ensures that the desired state of the ConfigMap is reflected in the cluster.
// If the ConfigMap does not exist, it will be created. If it exists, it will be patched with the new data.
func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req *ConfigMapReconcileRequest) error {
	opts := builder.ConfigMapOpts{
		Metadata: req.Metadata,
		Key:      req.Key,
		Data:     req.Data,
	}
	configMap, err := r.Builder.BuildConfigMap(opts, req.Owner)
	if err != nil {
		return fmt.Errorf("error building ConfigMap: %v", err)
	}

	var existingConfigMap corev1.ConfigMap
	if err := r.Get(ctx, req.Key, &existingConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting ConfigMap: %v", err)
		}
		if err := r.Create(ctx, configMap); err != nil {
			return fmt.Errorf("error creating ConfigMap: %v", err)
		}
		return nil
	}

	patch := client.MergeFrom(existingConfigMap.DeepCopy())
	existingConfigMap.Data = configMap.Data
	return r.Patch(ctx, &existingConfigMap, patch)
}
