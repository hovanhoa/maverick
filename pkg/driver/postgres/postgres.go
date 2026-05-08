package postgres

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/core/retries"
	"github.com/hovanhoa/llmgateway/pkg/driver"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DriverName driver.Name = "postgres"
)

var (
	validDBNameRegex     = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	dbAlreadyExistsRegex = regexp.MustCompile(`already exists`)
)

// Config configures the database connection.
type Config struct {
	Host string
	Port string
	User string
	Pass string
	Name string
}

func (c Config) GetSSLMode() string {
	if strings.HasSuffix(c.Host, ".rds.amazonaws.com") {
		return "require"
	}

	return "disable"
}

func (c Config) Serialize() map[string]string {
	return map[string]string{
		"host":     c.Host,
		"port":     c.Port,
		"user":     c.User,
		"password": c.Pass,
		"dbname":   c.Name,
		"sslmode":  c.GetSSLMode(),
	}
}

func (c Config) DSN() (connStr string) {
	psqlInfo := c.Serialize()

	// Construct the connection string
	for k, v := range psqlInfo {
		if len(v) > 0 {
			connStr += k + "=" + v + " "
		}
	}

	return strings.TrimSpace(connStr)
}

func (c Config) String() string {
	u := make(url.Values)
	u.Set("Host", c.Host)
	u.Set("Port", c.Port)
	u.Set("User", c.User)
	u.Set("Pass", c.Pass)
	u.Set("Name", c.Name)
	return u.Encode()
}

// Database implements a SQL DB backed by Postgres.
type Database struct {
	DB *pgxpool.Pool

	closed bool
	config Config

	// If a transaction is active, tx will not be nil and should be used
	// to perform queries against the database.
	tx     pgx.Tx
	parent *Database
}

// New creates a new connection to the given Postgres instance, attempts to create
// the given database name, and then returns a connection configured to talk to that
// database.
func New(ctx context.Context, config Config) (*Database, error) {
	// validate the given db name
	if !validDBNameRegex.MatchString(config.Name) {
		return nil, errors.New("Invalid database name %q. Must match `^[a-zA-Z0-9_]+$`.", config.Name)
	}

	// first connect to the default postgres db
	setupConn, err := getConn(ctx, Config{
		Host: config.Host,
		Port: config.Port,
		User: config.User,
		Pass: config.Pass,
		Name: "postgres",
	})
	if err != nil {
		return nil, errors.Wrap(err, "error connecting to db %q", "postgres")
	}

	defer setupConn.Close()

	// try to create the desired database
	_, err = setupConn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %q", config.Name))
	if err != nil {
		// ignore the error if the database already exists
		if !dbAlreadyExistsRegex.MatchString(err.Error()) {
			return nil, errors.Wrap(err, "could not create db %q", config.Name)
		}
	}

	// then try to connect to the desired database
	conn, err := getConn(ctx, config)
	if err != nil {
		return nil, errors.Wrap(err, "error connecting to db %q", config.Name)
	}

	return &Database{DB: conn, config: config}, nil
}

func getConn(ctx context.Context, config Config) (*pgxpool.Pool, error) {
	connStr := config.DSN()
	pgxConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, errors.Wrap(err, "ParseConfig(%q)", connStr)
	}

	pgxConfig.MaxConns = 25
	pgxConfig.MaxConnIdleTime = 5 * time.Minute
	pgxConfig.MinIdleConns = 4
	pgxConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// Greedily clean up idle connections in test mode and set higher max connections limit
	// since we may have a lot of tests running in parallel.
	if env.GetEnvironment() == env.Test || env.GetEnvironment() == env.Local {
		pgxConfig.MaxConns = 2
		pgxConfig.MinIdleConns = 0
		pgxConfig.MaxConnIdleTime = 10 * time.Millisecond
		pgxConfig.HealthCheckPeriod = 1 * time.Second
	}

	// TODO: tracing
	conn, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		return nil, errors.Wrap(err, "pgxpool.NewWithConfig(%q)", connStr)
	}

	if err := conn.Ping(ctx); err != nil {
		return nil, errors.Wrap(err, "conn.PingContext")
	}

	return conn, nil
}

// Runner returns an object connected to the DB to run queries.
func (db *Database) Runner() driver.SQLRunner {
	if db.tx != nil {
		return NewTracingRunner(db.config, db.tx)
	}

	return NewTracingRunner(db.config, db.DB)
}

func (db *Database) Builder() sq.StatementBuilderType {
	return sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
}

func (db *Database) Close() error {
	db.closed = true
	db.DB.Close()
	return nil
}

func (db *Database) IsClosed() bool {
	return db.closed
}

func (db Database) GetDriverName() driver.Name {
	return DriverName
}

// WithTransaction runs the given function in a transaction.
func (db *Database) WithTransaction(ctx context.Context, txFn driver.SQLWithTxFunc) error {
	backoff := retries.NewBackoff().
		WithInterval(100 * time.Millisecond).
		WithMaxInterval(5 * time.Second).
		WithMaxJitter(500 * time.Millisecond).
		WithMaxRetries(4)

	retryable := func(r *retries.RetryControl) error {
		nextDB, err := db.withTx(ctx)
		if err != nil {
			return err
		}

		return nextDB.maybeCloseTx(ctx, txFn(ctx, nextDB, r))
	}

	// If we are already in a transaction, don't start another backoff
	if db.tx != nil {
		return retryable(retries.NewRetryControl(&backoff))
	}

	return retries.WithBackoff(&backoff, retryable)
}

func (db *Database) withTx(ctx context.Context) (*Database, error) {
	if db.tx != nil {
		return &Database{DB: db.DB, tx: db.tx, parent: db}, nil
	}

	// Use READ COMMITTED isolation level (PostgreSQL default)
	// This works correctly with FOR UPDATE locks, allowing transactions to queue
	// instead of causing serialization conflicts. REPEATABLE READ + FOR UPDATE
	// causes "could not serialize access" errors even on SELECT statements.
	tx, err := db.DB.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return nil, err
	}

	return &Database{DB: db.DB, tx: tx, parent: nil}, nil
}

func (db *Database) maybeCloseTx(ctx context.Context, err error) error {
	// Don't close this transaction yet as it's nested
	if db.parent != nil {
		return err
	}

	if err != nil {
		_ = db.tx.Rollback(ctx)
		return err
	}

	if err = db.tx.Commit(ctx); err != nil {
		_ = db.tx.Rollback(ctx)
		return err
	}

	return nil
}
