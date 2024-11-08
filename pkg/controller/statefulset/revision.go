package statefulset

import (
	"context"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetRevision(ctx context.Context, r client.Client, doltdb *doltv1alpha.DoltDB) (string, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(doltdb), &sts); err != nil {
		return "", err
	}
	return sts.Status.UpdateRevision, nil
}
