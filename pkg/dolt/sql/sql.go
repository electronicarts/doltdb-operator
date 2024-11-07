package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-multierror"
	doltv1alpha1 "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
)

type Opts struct {
	Username string
	Password string
	Host     string
	Port     int32
	Database string
	Params   map[string]string
	Timeout  *time.Duration
}

type Opt func(*Opts)

func WithUsername(username string) Opt {
	return func(o *Opts) {
		o.Username = username
	}
}

func WithPassword(password string) Opt {
	return func(o *Opts) {
		o.Password = password
	}
}

func WitHost(host string) Opt {
	return func(o *Opts) {
		o.Host = host
	}
}

func WithPort(port int32) Opt {
	return func(o *Opts) {
		o.Port = port
	}
}

func WithDatabase(database string) Opt {
	return func(o *Opts) {
		o.Database = database
	}
}

func WithParams(params map[string]string) Opt {
	return func(o *Opts) {
		o.Params = params
	}
}

func WithTimeout(d time.Duration) Opt {
	return func(o *Opts) {
		o.Timeout = &d
	}
}

type Client struct {
	db *sql.DB
}

func NewClient(clientOpts ...Opt) (*Client, error) {
	opts := Opts{}
	for _, setOpt := range clientOpts {
		setOpt(&opts)
	}
	dsn, err := BuildDSN(opts)
	if err != nil {
		return nil, fmt.Errorf("error building DSN: %v", err)
	}
	db, err := Connect(dsn)
	if err != nil {
		return nil, err
	}
	return &Client{
		db: db,
	}, nil
}

func NewClientWithDoltDB(ctx context.Context, doltdb *doltv1alpha1.DoltDB, refResolver *refresolver.RefResolver,
	clientOpts ...Opt) (*Client, error) {
	password, err := refResolver.SecretKeyRef(ctx, doltdb.RootPasswordSecretKeyRef(), doltdb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading root password secret: %v", err)
	}
	opts := []Opt{
		WithUsername("root"),
		WithPassword(password),
		WitHost(func() string {
			return statefulset.ServiceFQDNWithService(
				doltdb.ObjectMeta,
				doltdb.PrimaryServiceKey().Name,
			)
		}()),
		WithPort(dolt.DatabasePort),
	}
	opts = append(opts, clientOpts...)
	return NewClient(opts...)
}

func NewInternalClientWithPodIndex(ctx context.Context, doltdb *doltv1alpha1.DoltDB, refResolver *refresolver.RefResolver,
	podIndex int, clientOpts ...Opt) (*Client, error) {
	opts := []Opt{
		WitHost(
			statefulset.PodFQDNWithService(
				doltdb.ObjectMeta,
				podIndex,
				doltdb.InternalServiceKey().Name,
			),
		),
	}
	opts = append(opts, clientOpts...)
	return NewClientWithDoltDB(ctx, doltdb, refResolver, opts...)
}

func BuildDSN(opts Opts) (string, error) {
	if opts.Host == "" || opts.Port == 0 {
		return "", errors.New("invalid opts: host and port are mandatory")
	}
	config := mysql.NewConfig()
	config.Net = "tcp"
	config.Addr = fmt.Sprintf("%s:%d", opts.Host, opts.Port)

	if opts.Timeout != nil {
		config.Timeout = *opts.Timeout
	} else {
		config.Timeout = 5 * time.Second
	}
	if opts.Username != "" && opts.Password != "" {
		config.User = opts.Username
		config.Passwd = opts.Password
	}
	if opts.Database != "" {
		config.DBName = opts.Database
	}
	if opts.Params != nil {
		config.Params = opts.Params
	}

	return config.FormatDSN(), nil
}

func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func ConnectWithOpts(opts Opts) (*sql.DB, error) {
	dsn, err := BuildDSN(opts)
	if err != nil {
		return nil, fmt.Errorf("error building DSN: %v", err)
	}
	return Connect(dsn)
}

func (c *Client) Close() error {
	return c.db.Close()
}

func (c *Client) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := c.db.ExecContext(ctx, sql, args...)
	return err
}

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

	defer rows.Close()

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
	defer rows.Close()

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
	defer rows.Close()
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
	rows, err := c.db.QueryContext(ctx, "SELECT `database`, role, epoch, standby_remote, replication_lag_millis, last_update, current_error FROM `dolt_cluster`.`dolt_cluster_status`")
	if err != nil {
		return nil, fmt.Errorf("error loading dolt_cluster_status table: %w", err)
	}
	defer rows.Close()

	var doltStates []dolt.DoltStatus

	for rows.Next() {
		var state dolt.DoltStatus
		err = rows.Scan(&state.Database, &state.Role, &state.Epoch, &state.Remote, &state.ReplicationLag, &state.LastUpdate, &state.CurrentError)
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
	var errBundle *multierror.Error
	var dbState = dolt.DBState{}

	role, epoch, err := c.GetRoleAndEpoch(ctx)
	if err != nil {
		dbState.Err = multierror.Append(dbState.Err, err)
		return dbState, err
	}
	dbState.Role = role
	dbState.Epoch = epoch

	version, err := c.GetVersion(ctx)
	errBundle = multierror.Append(errBundle, err)

	status, err := c.GetClusterStatus(ctx)
	errBundle = multierror.Append(errBundle, err)

	dbState.Version = version
	dbState.Status = status
	dbState.Err = errBundle.ErrorOrNil()

	return dbState, nil
}
