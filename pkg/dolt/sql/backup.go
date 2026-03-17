// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package sql

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var systemDatabases = map[string]bool{
	"information_schema": true,
	"mysql":              true,
	"dolt_cluster":       true,
}

// FilterSystemDatabases filters out DoltDB system databases from the given list.
func FilterSystemDatabases(databases []string) []string {
	var filtered []string
	for _, db := range databases {
		if !systemDatabases[strings.ToLower(db)] {
			filtered = append(filtered, db)
		}
	}
	return filtered
}

// ListDatabases returns a list of all user databases, excluding system databases.
func (c *Client) ListDatabases(ctx context.Context) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("error listing databases: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.FromContext(ctx).Error(err, "error closing rows on ListDatabases")
		}
	}()

	var databases []string
	for rows.Next() {
		var db string
		if err := rows.Scan(&db); err != nil {
			return nil, fmt.Errorf("error scanning database name: %w", err)
		}
		databases = append(databases, db)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating databases: %w", rows.Err())
	}

	return FilterSystemDatabases(databases), nil
}

// AddBackup registers a named backup destination for the current database.
func (c *Client) AddBackup(ctx context.Context, name, url string) error {
	var status int
	q := "CALL dolt_backup('add', ?, ?)"
	rows, err := c.db.QueryContext(ctx, q, name, url)
	if err != nil {
		// If backup already exists, this is not an error.
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return fmt.Errorf("error adding backup '%s': %w", name, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.FromContext(ctx).Error(err, "error closing rows on AddBackup")
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&status); err != nil {
			return fmt.Errorf("error scanning AddBackup status: %w", err)
		}
		if status != 0 {
			return fmt.Errorf("dolt_backup('add', '%s', '%s') returned status %d", name, url, status)
		}
	}
	if rows.Err() != nil {
		return rows.Err()
	}

	return nil
}

// SyncBackup syncs the current database to the named backup destination.
func (c *Client) SyncBackup(ctx context.Context, name string) error {
	var status int
	q := "CALL dolt_backup('sync', ?)"
	rows, err := c.db.QueryContext(ctx, q, name)
	if err != nil {
		return fmt.Errorf("error syncing backup '%s': %w", name, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.FromContext(ctx).Error(err, "error closing rows on SyncBackup")
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&status); err != nil {
			return fmt.Errorf("error scanning SyncBackup status: %w", err)
		}
		if status != 0 {
			return fmt.Errorf("dolt_backup('sync', '%s') returned status %d", name, status)
		}
	}
	if rows.Err() != nil {
		return rows.Err()
	}

	return nil
}

// SyncBackupURL performs a one-shot sync to a URL for the current database.
func (c *Client) SyncBackupURL(ctx context.Context, url string) error {
	var status int
	q := "CALL dolt_backup('sync-url', ?)"
	rows, err := c.db.QueryContext(ctx, q, url)
	if err != nil {
		return fmt.Errorf("error syncing backup to URL '%s': %w", url, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.FromContext(ctx).Error(err, "error closing rows on SyncBackupURL")
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&status); err != nil {
			return fmt.Errorf("error scanning SyncBackupURL status: %w", err)
		}
		if status != 0 {
			return fmt.Errorf("dolt_backup('sync-url', '%s') returned status %d", url, status)
		}
	}
	if rows.Err() != nil {
		return rows.Err()
	}

	return nil
}

// RemoveBackup removes a named backup destination for the current database.
func (c *Client) RemoveBackup(ctx context.Context, name string) error {
	var status int
	q := "CALL dolt_backup('remove', ?)"
	rows, err := c.db.QueryContext(ctx, q, name)
	if err != nil {
		return fmt.Errorf("error removing backup '%s': %w", name, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.FromContext(ctx).Error(err, "error closing rows on RemoveBackup")
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&status); err != nil {
			return fmt.Errorf("error scanning RemoveBackup status: %w", err)
		}
		if status != 0 {
			return fmt.Errorf("dolt_backup('remove', '%s') returned status %d", name, status)
		}
	}
	if rows.Err() != nil {
		return rows.Err()
	}

	return nil
}

// BackupDatabase sets the active database, registers a named backup
// destination, and syncs. The caller must ensure the backup name is stable
// per URL+database to enable incremental syncs and avoid address conflicts.
func (c *Client) BackupDatabase(ctx context.Context, database, backupName, backupURL string) error {
	if err := c.UseDatabase(ctx, database); err != nil {
		return fmt.Errorf("error setting database '%s': %w", database, err)
	}
	if err := c.AddBackup(ctx, backupName, backupURL); err != nil {
		return fmt.Errorf("error adding backup for database '%s': %w", database, err)
	}
	if err := c.SyncBackup(ctx, backupName); err != nil {
		return fmt.Errorf("error syncing backup for database '%s': %w", database, err)
	}
	return nil
}

// SyncBackupDatabase sets the active database and performs a one-shot sync to
// the given URL without registering a named backup. This avoids address
// conflicts but does not benefit from incremental syncs.
func (c *Client) SyncBackupDatabase(ctx context.Context, database, backupURL string) error {
	if err := c.UseDatabase(ctx, database); err != nil {
		return fmt.Errorf("error setting database '%s': %w", database, err)
	}
	if err := c.SyncBackupURL(ctx, backupURL); err != nil {
		return fmt.Errorf("error syncing backup for database '%s': %w", database, err)
	}
	return nil
}
