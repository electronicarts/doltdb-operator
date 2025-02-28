package sql

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type AssumeRoleOpts struct {
	Epoch int
	Role  dolt.Role
}

func (c *Client) AssumeRole(ctx context.Context, opts AssumeRoleOpts) error {
	var status int

	q := fmt.Sprintf("CALL DOLT_ASSUME_CLUSTER_ROLE('%s', %d)", opts.Role, opts.Epoch)
	rows, err := c.db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.FromContext(ctx).Error(err, "error closing rows on CALL DOLT_ASSUME_CLUSTER_ROLE()")
		}
	}()

	if rows.Next() {
		err = rows.Scan(&status)
		if err != nil {
			return err
		}
		if status != 0 {
			return fmt.Errorf("error calling dolt_assume_role('%s', %d) was %d, not 0", opts.Role, opts.Epoch, status)
		}
	}

	if rows.Err() != nil {
		return rows.Err()
	}

	return nil
}

type TransitionStandbyOpts struct {
	Epoch               int
	MinCaughtUpStandbys int
	Hosts               []string
}

func (c *Client) TransitionToStandby(ctx context.Context, opts TransitionStandbyOpts) (int, error) {
	type TransitionResult struct {
		CaughtUp  int
		Database  string
		Remote    string
		RemoteURL string
		Parsed    *url.URL
	}
	var results []TransitionResult

	q := fmt.Sprintf("CALL DOLT_CLUSTER_TRANSITION_TO_STANDBY('%d', '%d')", opts.Epoch, opts.MinCaughtUpStandbys)
	rows, err := c.db.QueryContext(ctx, q)
	if err != nil {
		return -1, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.FromContext(ctx).Error(err, "error closing rows on CALL DOLT_CLUSTER_TRANSITION_TO_STANDBY()")
		}
	}()

	for rows.Next() {
		var res TransitionResult
		err = rows.Scan(&res.CaughtUp, &res.Database, &res.Remote, &res.RemoteURL)
		if err != nil {
			return -1, err
		}
		results = append(results, res)
	}
	if rows.Err() != nil {
		return -1, rows.Err()
	}

	numCaughtUp := make(map[string]int)
	for i := range results {
		var err error
		results[i].Parsed, err = url.Parse(results[i].RemoteURL)
		if err != nil {
			return -1, err
		}
		if results[i].CaughtUp == 1 {
			numCaughtUp[results[i].Parsed.Host] = numCaughtUp[results[i].Parsed.Host] + 1
		}
	}

	var maxCaughtUpHost string
	var maxCaughtUp int
	for k, v := range numCaughtUp {
		if v > maxCaughtUp {
			maxCaughtUpHost = k
			maxCaughtUp = v
		}
	}

	var maxCaughtUpParsedURL *url.URL
	for _, res := range results {
		if res.Parsed.Host == maxCaughtUpHost {
			maxCaughtUpParsedURL = res.Parsed
			break
		}
	}

	if maxCaughtUpParsedURL == nil {
		return -1, fmt.Errorf("internal error: did not find caught up URL of the caught up host: %s", maxCaughtUpHost)
	}
	caughtUpHostname := maxCaughtUpParsedURL.Hostname()

	for i, host := range opts.Hosts {
		if host == caughtUpHostname || strings.HasPrefix(host, caughtUpHostname) {
			return i, nil
		}
	}

	return -1, fmt.Errorf("internal error: did not find caught up URL of the caught up host: %s", maxCaughtUpHost)
}

func (c *Client) GetRoleAndEpoch(ctx context.Context) (string, int, error) {
	var role string
	var epoch int

	rows, err := c.db.QueryContext(ctx, "SELECT @@global.dolt_cluster_role, @@global.dolt_cluster_role_epoch")
	if err != nil {
		return "", 0, fmt.Errorf("error querying DoltDB cluster_role and epoch: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.FromContext(ctx).Error(err, "error closing rows on GetRoleAndEpoch")
		}
	}()

	if rows.Next() {
		err = rows.Scan(&role, &epoch)
		if err != nil {
			return "", 0, err
		}
	} else if rows.Err() == nil {
		return "", 0, errors.New("querying DoltDB and epoch should have return values, but did not")
	}
	if rows.Err() != nil {
		return "", 0, rows.Err()
	}

	return role, epoch, nil
}

func (c *Client) GetClusterStatus(ctx context.Context) ([]dolt.DoltStatus, error) {
	rows, err := c.db.QueryContext(ctx,
		"SELECT `database`, role, epoch, standby_remote, replication_lag_millis, last_update,"+
			"current_error FROM `dolt_cluster`.`dolt_cluster_status`",
	)
	if err != nil {
		return nil, fmt.Errorf("error loading dolt_cluster_status table: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.FromContext(ctx).Error(err, "error closing rows on GetClusterStatus")
		}
	}()

	var doltStates []dolt.DoltStatus

	for rows.Next() {
		var state dolt.DoltStatus
		err = rows.Scan(
			&state.Database,
			&state.Role,
			&state.Epoch,
			&state.Remote,
			&state.ReplicationLag,
			&state.LastUpdate,
			&state.CurrentError,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning dolt_cluster_status status row: %w", err)
		}
		doltStates = append(doltStates, state)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("error loading dolt_cluster_status rows: %w", err)
	}

	return doltStates, nil
}
