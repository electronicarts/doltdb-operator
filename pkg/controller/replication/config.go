package replication

import (
	"context"
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	sqlClient "github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReplicationConfig struct {
	client.Client
	builder     *builder.Builder
	refResolver *refresolver.RefResolver
}

func NewReplicationConfig(client client.Client, builder *builder.Builder) *ReplicationConfig {
	return &ReplicationConfig{
		Client:      client,
		builder:     builder,
		refResolver: refresolver.New(client),
	}
}

func (r *ReplicationConfig) ConfigurePrimary(
	ctx context.Context,
	doltdb *doltv1alpha.DoltCluster,
	client *sqlClient.Client,
	podIndex int,
	nextReplicationEpoch int,
) error {
	assumeRoleOpts := sqlClient.AssumeRoleOpts{
		Epoch: nextReplicationEpoch,
		Role:  dolt.PrimaryRoleValue,
	}

	if err := client.AssumeRole(ctx, assumeRoleOpts); err != nil {
		return fmt.Errorf("error configuring primary role and epoch %d for pod %s: %v",
			nextReplicationEpoch,
			statefulset.PodName(doltdb.ObjectMeta, podIndex),
			err,
		)
	}
	return nil
}

func (r *ReplicationConfig) ConfigureReplica(
	ctx context.Context,
	doltdb *doltv1alpha.DoltCluster,
	client *sqlClient.Client,
	podIndex int,
	nextReplicationEpoch int,
) error {
	assumeRoleOpts := sqlClient.AssumeRoleOpts{
		Epoch: nextReplicationEpoch,
		Role:  dolt.StandbyRoleValue,
	}

	if err := client.AssumeRole(ctx, assumeRoleOpts); err != nil {
		return fmt.Errorf("error configuring standby role and epoch %d for pod %s: %v",
			nextReplicationEpoch,
			statefulset.PodName(doltdb.ObjectMeta, podIndex),
			err,
		)
	}
	return nil
}
