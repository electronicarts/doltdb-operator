package builder

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type MetadataBuilder struct {
	objMeta metav1.ObjectMeta
}

// NewMetadataBuilder creates a new MetadataBuilder with the given NamespacedName.
func NewMetadataBuilder(key types.NamespacedName) *MetadataBuilder {
	return &MetadataBuilder{
		objMeta: metav1.ObjectMeta{
			Name:        key.Name,
			Namespace:   key.Namespace,
			Labels:      NewLabelsBuilder().Build(),
			Annotations: map[string]string{},
		},
	}
}

// WithReleaseLabel adds a release label to the metadata if the release string is not empty.
func (b *MetadataBuilder) WithReleaseLabel(release string) *MetadataBuilder {
	if release == "" {
		return b
	}
	return b.WithLabels(map[string]string{
		"release": release,
	})
}

// WithMetadata adds labels and annotations from the given DoltCluster metadata.
func (b *MetadataBuilder) WithMetadata(meta *metav1.ObjectMeta) *MetadataBuilder {
	if meta == nil {
		return b
	}
	for k, v := range meta.Labels {
		b.objMeta.Labels[k] = v
	}
	for k, v := range meta.Annotations {
		b.objMeta.Annotations[k] = v
	}
	return b
}

// WithLabels adds the given labels to the metadata.
func (b *MetadataBuilder) WithLabels(labels map[string]string) *MetadataBuilder {
	for k, v := range labels {
		b.objMeta.Labels[k] = v
	}
	return b
}

// WithAnnotations adds the given annotations to the metadata.
func (b *MetadataBuilder) WithAnnotations(annotations map[string]string) *MetadataBuilder {
	for k, v := range annotations {
		b.objMeta.Annotations[k] = v
	}
	return b
}

// Build returns the constructed ObjectMeta.
func (b *MetadataBuilder) Build() metav1.ObjectMeta {
	return b.objMeta
}
