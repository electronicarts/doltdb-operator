package refresolver

import (
	"context"
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RefResolver struct {
	client client.Client
}

// New creates a new RefResolver instance.
func New(client client.Client) *RefResolver {
	return &RefResolver{
		client: client,
	}
}

// DoltDB retrieves a DoltDB resource based on the provided DoltDBRef and namespace.
func (r *RefResolver) DoltDB(ctx context.Context, ref *doltv1alpha.DoltDBRef,
	namespace string) (*doltv1alpha.DoltDB, error) {
	key := types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}
	if ref.Namespace != "" {
		key.Namespace = ref.Namespace
	}

	var doltdb doltv1alpha.DoltDB
	if err := r.client.Get(ctx, key, &doltdb); err != nil {
		return nil, err
	}
	return &doltdb, nil
}

// DoltDBPodRef retrieves a Pod resource based on the provided DoltDB and podIndex.
func (r *RefResolver) DoltDBPodRef(ctx context.Context, doltdb *doltv1alpha.DoltDB, index int) (*corev1.Pod, error) {
	key := types.NamespacedName{
		Name:      statefulset.PodName(doltdb.ObjectMeta, index),
		Namespace: doltdb.Namespace,
	}
	var pod corev1.Pod
	if err := r.client.Get(ctx, key, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

// DoltDBFromAnnotation retrieves a DoltDB resource based on the annotation in the provided ObjectMeta.
func (r *RefResolver) DoltDBFromAnnotation(ctx context.Context, objMeta metav1.ObjectMeta) (*doltv1alpha.DoltDB, error) {
	doltdbAnnotation, ok := objMeta.Annotations[dolt.Annotation]
	if !ok {
		return nil, ErrDoltClusterAnnotationNotFound
	}

	var doltdb doltv1alpha.DoltDB
	key := types.NamespacedName{
		Name:      doltdbAnnotation,
		Namespace: objMeta.Namespace,
	}
	if err := r.client.Get(ctx, key, &doltdb); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("error getting DoltDB from annotation '%s': %v", objMeta.Name, err)
	}
	return &doltdb, nil
}

// SecretKeyRef retrieves the value of a specific key from a Secret resource based on the provided SecretKeySelector and namespace.
func (r *RefResolver) SecretKeyRef(ctx context.Context, selector doltv1alpha.SecretKeySelector,
	namespace string) (string, error) {
	key := types.NamespacedName{
		Name:      selector.Name,
		Namespace: namespace,
	}
	var secret corev1.Secret
	if err := r.client.Get(ctx, key, &secret); err != nil {
		return "", fmt.Errorf("error getting Secret: %v", err)
	}

	data, ok := secret.Data[selector.Key]
	if !ok {
		return "", fmt.Errorf("secret key \"%s\" not found", selector.Key)
	}
	return string(data), nil
}

// ConfigMapKeyRef retrieves the value of a specific key from a ConfigMap resource based on the provided ConfigMapKeySelector and namespace.
func (r *RefResolver) ConfigMapKeyRef(ctx context.Context, selector *doltv1alpha.ConfigMapKeySelector,
	namespace string) (string, error) {
	key := types.NamespacedName{
		Name:      selector.Name,
		Namespace: namespace,
	}
	var configMap corev1.ConfigMap
	if err := r.client.Get(ctx, key, &configMap); err != nil {
		return "", fmt.Errorf("error getting ConfigMap: %v", err)
	}

	data, ok := configMap.Data[selector.Key]
	if !ok {
		return "", fmt.Errorf("ConfigMap key \"%s\" not found", selector.Key)
	}
	return data, nil
}
