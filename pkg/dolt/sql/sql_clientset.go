package sql

import (
	"context"
	"fmt"
	"sync"

	doltv1alpha1 "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
)

type ClientSet struct {
	DoltDB        *doltv1alpha1.DoltCluster
	refResolver   *refresolver.RefResolver
	clientByIndex map[int]*Client
	mux           *sync.Mutex
}

// NewClientSet creates a new ClientSet instance.
func NewClientSet(doltdb *doltv1alpha1.DoltCluster, refResolver *refresolver.RefResolver) *ClientSet {
	return &ClientSet{
		DoltDB:        doltdb,
		refResolver:   refResolver,
		clientByIndex: make(map[int]*Client),
		mux:           &sync.Mutex{},
	}
}

// Close closes all clients in the ClientSet.
func (c *ClientSet) Close() error {
	for i, rc := range c.clientByIndex {
		if err := rc.Close(); err != nil {
			return fmt.Errorf("error closing replica '%d' client: %v", i, err)
		}
	}
	return nil
}

// ClientForIndex returns a client for the given index, creating it if necessary.
func (c *ClientSet) ClientForIndex(ctx context.Context, index int, clientOpts ...Opt) (*Client, error) {
	if err := c.validateIndex(index); err != nil {
		return nil, fmt.Errorf("invalid index. %v", err)
	}
	if c, ok := c.clientByIndex[index]; ok {
		return c, nil
	}
	client, err := NewInternalClientWithPodIndex(ctx, c.DoltDB, c.refResolver, index, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating replica '%d' client: %v", index, err)
	}
	c.mux.Lock()
	c.clientByIndex[index] = client
	c.mux.Unlock()
	return client, nil
}

// validateIndex checks if the given index is within the bounds of the DoltDB replicas.
func (c *ClientSet) validateIndex(index int) error {
	if index >= 0 && index < int(c.DoltDB.Spec.Replicas) {
		return nil
	}
	return fmt.Errorf("index '%d' out of DoltDB replicas bounds [0, %d]", index, c.DoltDB.Spec.Replicas-1)
}
