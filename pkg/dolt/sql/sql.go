// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
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

func NewClientWithDoltDB(ctx context.Context, doltdb *doltv1alpha.DoltDB, refResolver *refresolver.RefResolver,
	clientOpts ...Opt) (*Client, error) {
	password, err := refResolver.SecretKeyRef(ctx, doltdb.RootPasswordSecretKeyRef(), doltdb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading root password secret: %v", err)
	}
	username, err := refResolver.SecretKeyRef(ctx, doltdb.RootUserSecretKeyRef(), doltdb.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading root username secret: %v", err)
	}

	serviceName := doltdb.PrimaryServiceKey().Name
	if !doltdb.Replication().Enabled {
		serviceName = doltdb.ServiceKey().Name
	}

	opts := []Opt{
		WithUsername(username),
		WithPassword(password),
		WitHost(statefulset.ServiceFQDNWithService(doltdb.ObjectMeta, serviceName)),
		WithPort(doltdb.Spec.Server.Listener.Port),
	}
	opts = append(opts, clientOpts...)
	return NewClient(opts...)
}

func NewInternalClientWithPodIndex(ctx context.Context, doltdb *doltv1alpha.DoltDB, refResolver *refresolver.RefResolver,
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
	config.ParseTime = true

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

func (c *Client) Query(ctx context.Context, sql string, args ...any) (*sql.Rows, error) {
	return c.db.QueryContext(ctx, sql, args...)
}

func (c *Client) QueryRow(ctx context.Context, sql string, args ...any) *sql.Row {
	return c.db.QueryRowContext(ctx, sql, args...)
}
