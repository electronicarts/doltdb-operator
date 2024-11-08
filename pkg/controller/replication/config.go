package replication

import (
	"context"
	"errors"
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	sqlClient "github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/health"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	doltdb *doltv1alpha.DoltDB,
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
	doltdb *doltv1alpha.DoltDB,
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

func (r *ReplicationConfig) GetNextPrimary(
	ctx context.Context,
	doltdb *doltv1alpha.DoltDB,
	client *sqlClient.Client,
	epoch int,
) (int, error) {
	if doltdb.Status.CurrentPrimaryPodIndex == nil {
		return -1, errors.New("'status.currentPrimaryPodIndex' must be set")
	}

	minCaughtUpStandbys := ptr.Deref(doltdb.Replication().Primary.MinCaughtUpStandbys, -1)
	numStandbys := int(doltdb.Spec.Replicas) - 1
	if minCaughtUpStandbys != -1 {
		return -1, errors.New("minCaughtUpStandbys must be greater than -1")
	}
	if minCaughtUpStandbys > numStandbys {
		return -1, errors.New("minCaughtUpStandbys must be less than the number of standbys")
	}

	healthyStandbys, err := health.HealthyDoltDBStandbys(ctx, r, doltdb)
	if err != nil {
		return -1, fmt.Errorf("error getting healthy DoltDB standby replicas: %v", err)
	}

	if len(healthyStandbys) < minCaughtUpStandbys {
		return -1, fmt.Errorf("not enough healthy standbys to transition to primary: %d/%d", len(healthyStandbys), minCaughtUpStandbys)
	}

	hosts := make([]string, len(healthyStandbys))
	for i, standby := range healthyStandbys {
		podIndex, err := statefulset.PodIndex(standby.Name)
		if err != nil {
			return -1, fmt.Errorf("error getting index for Pod '%s': %v", standby.Name, err)
		}
		hosts[i] = statefulset.PodShortFQDNWithServiceAndNamespace(doltdb.ObjectMeta, *podIndex, doltdb.InternalServiceKey().Name)
	}

	assumeRoleOpts := sqlClient.TransitionStandbyOpts{
		Epoch:               epoch,
		MinCaughtUpStandbys: minCaughtUpStandbys,
		Hosts:               hosts,
	}

	nextPrimary, err := client.TransitionToStandby(ctx, assumeRoleOpts)
	if err != nil {
		return -1, fmt.Errorf("error configuring transitioning primary %d to standby and finding next primary at epoch %d: %v",
			*doltdb.Status.CurrentPrimaryPodIndex,
			epoch,
			err,
		)
	}

	return nextPrimary, nil
}

func GetDBStates(ctx context.Context, doltdb *doltv1alpha.DoltDB, clientSet *ReplicationClientSet) []dolt.DBState {
	ret := make([]dolt.DBState, doltdb.Spec.Replicas)
	for i := 0; i < int(doltdb.Spec.Replicas); i++ {
		client, err := clientSet.ClientForIndex(ctx, i)
		if err != nil {
			continue
		}
		ret[i], err = client.GetDBState(ctx)
		if err != nil {
			log.FromContext(ctx).V(1).Error(err, "error getting DB state, skipping", "pod", statefulset.PodName(doltdb.ObjectMeta, i))
			continue
		}
	}
	return ret
}
