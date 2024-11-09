package statefulset

import (
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	builder     *builder.Builder
}

// NewReconciler creates a new with the given client.
func NewReconciler(client client.Client, refResolver *refresolver.RefResolver, builder *builder.Builder) *Reconciler {
	return &Reconciler{
		Client:      client,
		refResolver: refResolver,
		builder:     builder,
	}
}
