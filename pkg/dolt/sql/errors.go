// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package sql

import (
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrInvalidUserIdentifier = fmt.Errorf("invalid user identifier")
	ErrBranchExists          = fmt.Errorf("branch already exists")
)

// mysqlErrReadOnly is the MySQL error code Dolt returns when a query is
// executed on a server that has already been transitioned to read-only/standby mode.
// Error 1105 (HY000): "database server is set to read only mode"
const mysqlErrReadOnly uint16 = 1105

// IsReadOnlyError checks if the error is a MySQL protocol error indicating the
// server is already in read-only mode, meaning the transition to standby already
// happened (e.g. from a previous reconciliation attempt or preStop hook).
func IsReadOnlyError(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == mysqlErrReadOnly
	}
	return false
}
