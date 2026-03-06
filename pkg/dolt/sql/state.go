// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package sql

import (
	"context"
	"fmt"

	"errors"

	"github.com/electronicarts/doltdb-operator/pkg/dolt"
)

func (c *Client) GetVersion(ctx context.Context) (string, error) {
	row := c.db.QueryRowContext(ctx, "SELECT dolt_version()")
	if row.Err() != nil {
		return "", fmt.Errorf("error loading dolt_version table function: %w", row.Err())
	}

	var version string
	err := row.Scan(&version)
	if err != nil {
		return "", fmt.Errorf("error scanning column of dolt_version table as string: %w", err)
	}

	return version, nil
}

func (c *Client) GetDBState(ctx context.Context) (dolt.DBState, error) {
	var dbState = dolt.DBState{}

	role, epoch, err := c.GetRoleAndEpoch(ctx)
	if err != nil {
		dbState.Err = err
		return dbState, err
	}
	dbState.Role = role
	dbState.Epoch = epoch

	version, versionErr := c.GetVersion(ctx)
	status, statusErr := c.GetClusterStatus(ctx)

	dbState.Version = version
	dbState.Status = status
	dbState.Err = errors.Join(versionErr, statusErr)

	return dbState, nil
}
