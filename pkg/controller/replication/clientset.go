package replication

import (
	"context"
	"errors"
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	sqlClient "github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
)

type ReplicationClientSet struct {
	*sqlClient.ClientSet
}

func NewReplicationClientSet(doltCluster *doltv1alpha.DoltCluster, refResolver *refresolver.RefResolver) (*ReplicationClientSet, error) {
	return &ReplicationClientSet{
		ClientSet: sqlClient.NewClientSet(doltCluster, refResolver),
	}, nil
}

func (c *ReplicationClientSet) close() error {
	return c.Close()
}

func (c *ReplicationClientSet) clientForIndex(ctx context.Context, index int) (*sqlClient.Client, error) {
	return c.ClientForIndex(ctx, index)
}

func (c *ReplicationClientSet) currentPrimaryClient(ctx context.Context) (*sqlClient.Client, error) {
	if c.DoltDB.Status.CurrentPrimaryPodIndex == nil {
		return nil, errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	client, err := c.ClientForIndex(ctx, *c.DoltDB.Status.CurrentPrimaryPodIndex)
	if err != nil {
		return nil, fmt.Errorf("error getting current primary client: %v", err)
	}
	return client, nil
}

// func (c *ReplicationClientSet) newPrimaryClient(ctx context.Context) (*sqlClient.Client, error) {
// 	client, err := c.ClientForIndex(ctx, *c.DoltDB.Replication().Primary.PodIndex)
// 	if err != nil {
// 		return nil, fmt.Errorf("error getting new primary client: %v", err)
// 	}
// 	return client, nil
// }
