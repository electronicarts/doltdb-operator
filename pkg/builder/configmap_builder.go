// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ConfigMapOpts holds the options for building a ConfigMap.
type ConfigMapOpts struct {
	Metadata *metav1.ObjectMeta
	Key      types.NamespacedName
	Data     map[string]string
}

// HashConfigMapData computes a SHA256 hash of the ConfigMap data.
// The keys are sorted to ensure consistent hash values regardless of map iteration order.
func HashConfigMapData(data map[string]string) string {
	if len(data) == 0 {
		return ""
	}

	// Sort keys for deterministic hashing
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(fmt.Sprintf("%s=%s;", k, data[k])))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// BuildConfigMap creates a ConfigMap based on the provided options and sets the owner reference.
// It returns the created ConfigMap or an error if the operation fails.
func (b *Builder) BuildConfigMap(options ConfigMapOpts, doltdb *doltv1alpha.DoltDB) (*corev1.ConfigMap, error) {
	labels := NewLabelsBuilder().
		WithDoltSelectorLabels(doltdb).
		Build()

	objMeta := NewMetadataBuilder(options.Key).
		WithMetadata(options.Metadata).
		WithLabels(labels).
		Build()

	cm := &corev1.ConfigMap{
		ObjectMeta: objMeta,
		Data:       options.Data,
	}
	if err := controllerutil.SetControllerReference(doltdb, cm, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ConfigMap: %v", err)
	}
	return cm, nil
}
