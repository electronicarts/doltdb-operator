package statefulset

import "errors"

var (
	ErrSkipReconciliationPhase = errors.New("skipping reconciliation phase")
)
