// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package wait

import (
	"context"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// PollUntilSucessOrContextCancel polls the provided function until it succeeds or the context is canceled.
// It logs any errors encountered during polling.
func PollUntilSucessOrContextCancel(ctx context.Context, logger logr.Logger, fn func(ctx context.Context) error) error {
	return kwait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		if err := fn(ctx); err != nil {
			logger.V(1).Info("Error polling", "err", err)
			return false, nil
		}
		return true, nil
	})
}

// PollWithDoltDB polls the provided function if the DoltDB resource exists and is retrievable.
// It uses PollUntilSucessOrContextCancel for polling.
func PollWithDoltDB(ctx context.Context, doltdbKey types.NamespacedName, client ctrlclient.Client, logger logr.Logger,
	fn func(ctx context.Context) error) error {
	return PollUntilSucessOrContextCancel(ctx, logger, func(ctx context.Context) error {
		if shouldPoll(ctx, doltdbKey, client, logger) {
			return fn(ctx)
		}
		return nil
	})
}

func shouldPoll(ctx context.Context, doltdbKey types.NamespacedName, client ctrlclient.Client, logger logr.Logger) bool {
	var doltdb doltv1alpha.DoltDB
	if err := client.Get(ctx, doltdbKey, &doltdb); err != nil {
		logger.V(1).Info("Error getting DoltDB", "err", err)
		return !apierrors.IsNotFound(err)
	}
	return true
}
