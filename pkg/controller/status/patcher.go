package status

import (
	"context"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PatcherDoltDB func(*doltv1alpha.DoltDBStatus) error

type PatcherVolumeSnapshot func(*doltv1alpha.SnapshotStatus) error

func PatchStatus(ctx context.Context, r client.Client, doltdb *doltv1alpha.DoltDB, patcher PatcherDoltDB) error {
	patch := client.MergeFrom(doltdb.DeepCopy())
	if err := patcher(&doltdb.Status); err != nil {
		return err
	}
	return r.Status().Patch(ctx, doltdb, patch)
}
