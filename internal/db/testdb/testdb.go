// Package testdb provides utilities to spin up and run a
// fully functional database in unit and integration tests.
package testdb

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/db/migrations"
	"github.com/hovanhoa/llmgateway/pkg/core/encoding"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/driver/memkv"
	"github.com/hovanhoa/llmgateway/pkg/driver/postgres"
)

func getLocalConfig(name string) postgres.Config {
	return postgres.Config{
		Host: "localhost",
		User: "postgres",
		Pass: "postgres",
		Port: "54320",
		Name: name,
	}
}

// Per-binary migrated template database. The first call to NewTestDatabase
// creates this template (running migrations exactly once); subsequent calls
// clone it with `CREATE DATABASE ... TEMPLATE <tpl>`, which in pg16 is a
// filesystem-level block copy and is an order of magnitude cheaper than
// re-running migrations per test.
var (
	templateOnce sync.Once
	templateName string
	templateErr  error
)

// ensureTemplateDatabase lazily creates (once per test binary) a postgres
// database with all migrations applied, marked IS_TEMPLATE true so it can
// be used as a source for fast `CREATE DATABASE ... TEMPLATE` clones
// without requiring the caller to disconnect concurrent sessions from it.
func ensureTemplateDatabase(ctx context.Context) (string, error) {
	templateOnce.Do(func() {
		name := encoding.NewRandomIdentifier("tpl")

		client, err := postgres.New(ctx, getLocalConfig(name))
		if err != nil {
			templateErr = errors.Wrap(err, "create template db %q", name)
			return
		}
		if err := migrations.NewMigrator(client, memkv.New()).Up(ctx); err != nil {
			_ = client.Close()
			templateErr = errors.Wrap(err, "migrate template db %q", name)
			return
		}
		if err := client.Close(); err != nil {
			templateErr = errors.Wrap(err, "close template db %q", name)
			return
		}

		// ALTER DATABASE ... IS_TEMPLATE true prevents any further client
		// sessions from connecting to the template, so CREATE DATABASE
		// clones never have to contend for a connection-free snapshot.
		admin, err := postgres.New(ctx, getLocalConfig("postgres"))
		if err != nil {
			templateErr = errors.Wrap(err, "connect admin db to mark template")
			return
		}
		defer func() { _ = admin.Close() }()
		if _, err := admin.DB.Exec(ctx,
			fmt.Sprintf(`ALTER DATABASE %q IS_TEMPLATE true`, name)); err != nil {
			templateErr = errors.Wrap(err, "mark %q as template", name)
			return
		}

		templateName = name
	})
	return templateName, templateErr
}

// cloneFromTemplate creates dbName as a copy of the migrated template.
// CREATE DATABASE cannot run inside a transaction, so we issue it over a
// short-lived admin connection.
func cloneFromTemplate(ctx context.Context, dbName, tpl string) error {
	admin, err := postgres.New(ctx, getLocalConfig("postgres"))
	if err != nil {
		return errors.Wrap(err, "connect admin db to clone %q", dbName)
	}
	defer func() { _ = admin.Close() }()

	if _, err := admin.DB.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %q TEMPLATE %q`, dbName, tpl)); err != nil {
		return errors.Wrap(err, "clone template %q into %q", tpl, dbName)
	}
	return nil
}

// Create a DB object connecting to the created embedded postgres instance
func connectTestDatabase(ctx context.Context, t *testing.T, runMigrations bool) (*db.Database, error) {
	dbName := encoding.NewRandomIdentifier("db")

	// Fast path: clone from the pre-migrated template rather than running
	// the full migration chain per test. postgres.New below will notice
	// the database already exists and skip its own CREATE DATABASE.
	if runMigrations {
		tpl, err := ensureTemplateDatabase(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to prepare migrated template")
		}
		if err := cloneFromTemplate(ctx, dbName, tpl); err != nil {
			return nil, err
		}
	}

	sqlClient, err := postgres.New(ctx, getLocalConfig(dbName))
	if err != nil {
		return nil, err
	}

	// Use a simple in-memory key-value store for the KV database
	kvStore := memkv.New()

	t.Cleanup(func() {
		if err := sqlClient.Close(); err != nil {
			t.Fatal(errors.Wrap(err, "error closing postgres connection"))
		}

		dropClient, err := postgres.New(ctx, getLocalConfig("postgres"))
		if err == nil {
			_, _ = dropClient.DB.Exec(ctx, fmt.Sprintf("DROP DATABASE %q", dbName))
			_ = dropClient.Close()
		}
	})

	database := db.New(sqlClient, kvStore)
	return database, nil
}

// SetupDatabaseAndRunTests spins up an instance of embedded postgres and
// runs the tests that use it. Tests run in the given suite should use
// `NewTestDatabase` to connect to the instance created here. This function
// also manages the lifecycle of the postgres instance by shutting it down
// after all tests run. To ensure that this works correctly, the test suite
// must be set up to use this function at least like this:
//
// ```
//
//	func TestMain(m *testing.M) {
//	  os.Exit(testdb.SetupDatabaseAndRunTests(m))
//	}
//
// ```
//
// NB: this function is designed this way because os.Exit terminates the
// program immediately, without waiting for calls to defer to complete.
// By putting the defer statements in the separate function, we ensure
// they are called before os.Exit is called.
func SetupDatabaseAndRunTests(m *testing.M) int {
	return m.Run()
}

func NewTestDatabase(ctx context.Context, t *testing.T) *db.Database {
	return newTestDatabase(ctx, t, true)
}

func NewEmptyTestDatabase(ctx context.Context, t *testing.T) *db.Database {
	return newTestDatabase(ctx, t, false)
}

// NewTestDatabase returns a DB instance configured to talk to the embedded
// postgres instance spun up as a part of this test suite. It terminates the
// test if there was an error connecting to the database and cleans up after
// itself, so the intended usage is fairly simple:
//
// ```
//
//	func Test(t *testing.T) {
//	  ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	  defer cancel()
//
//	  db := NewTestDatabase(ctx, t)
//	  // ... rest of the test
//	}
//
// ```
//
// This function should only be called in a test that runs in the context
// of `SetupDatabaseAndRunTests`. See above for details.
func newTestDatabase(ctx context.Context, t *testing.T, runMigrations bool) *db.Database {
	db, err := connectTestDatabase(ctx, t, runMigrations)
	if err != nil {
		t.Fatal(errors.Wrap(err, "error connecting to embedded postgres"))
	}

	return db
}
