package migrations_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/internal/db/migrations"
	"github.com/hovanhoa/llmgateway/internal/db/migrations/manual_migrations"
	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/pkg/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var manualTestMigrations = map[string]migrations.MigrationEntry{
	"test_migration_only": {
		ID:        "test_migration_only",
		Name:      "This is a test only migration, do not use.",
		Migration: &manual_migrations.TestMigration{},
	},
	"broken_up_migration": {
		ID:        "broken_up_migration",
		Name:      "This is a broken migration for testing",
		Migration: &manual_migrations.BrokenUpMigration{},
	},
	"broken_down_migration": {
		ID:        "broken_down_migration",
		Name:      "This is a broken migration for testing",
		Migration: &manual_migrations.BrokenDownMigration{},
	},
}

func getTestMigrations() []migrations.MigrationEntry {
	return append(migrations.GetMigrations(), getTestMigration(migrations.GetMigrations()))
}

func TestMigrationsOrder(t *testing.T) {
	t.Parallel()

	m := migrations.GetMigrations()
	for i := 0; i < len(m)-1; i++ {
		assert.Lessf(t, m[i].Index, m[i+1].Index, "migrations %q and %q out of order", m[i].Name, m[i+1].Name)
	}
}

func TestMigratorUpSucceeds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Add the test migration to the existing list of migrations.
	m := migrations.GetMigrations()
	m = append(m, getTestMigration(m))

	// Run the migrations and verify that there was no error and the
	// data from the test migration is available.
	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(m))
	assert.NoError(t, migrator.Up(ctx))
	verifyTestMigrations(t, ctx, db.GetSQLClient(), "test_table")

	// check that all migrations were created correctly in order
	rows, err := db.GetSQLClient().Runner().Query(ctx, `
		SELECT
			index,
			created_at
		FROM
			migrations_state
		ORDER BY
			created_at ASC
	`)
	assert.NoError(t, err)

	defer rows.Close()

	var lastIndex = -1
	var lastCreatedAt time.Time

	for rows.Next() {
		var index int
		var createdAt time.Time

		assert.NoError(t, rows.Scan(&index, &createdAt))
		assert.Greater(t, index, lastIndex)
		assert.True(
			t,
			createdAt.After(lastCreatedAt),
			"%s (%d) should be after %s (%d)",
			createdAt,
			index,
			lastCreatedAt,
			lastIndex,
		)

		lastCreatedAt = createdAt
		lastIndex = index
	}

	assert.Equal(t, m[len(m)-1].Index, lastIndex)
}

func TestMigratorUpFails_OutOfOrder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(getTestMigrations()))
	// Reset DB
	assert.NoError(t, migrator.Down(ctx))
	assert.NoError(t, migrator.Up(ctx))

	// Update the timestamp of the first migration to be after the last migration.
	_, err := db.GetSQLClient().Runner().Exec(ctx, `
		UPDATE
			migrations_state
		SET
			created_at = $1
		WHERE
			index = $2
		`,
		time.Now().Add(1*time.Hour),
		0,
	)
	require.NoError(t, err)

	migrator = migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(getTestMigrations()))
	err = migrator.Up(ctx)

	// Migration should now fail to start because the migrations_state table
	// is in an invalid state.
	require.Error(t, err)
	assert.Equal(t, "error initializing the migrator: migration 1 out of order. got: 0", err.Error())
}

func TestMigratorUpFails_MissingMigration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(getTestMigrations()))
	// Reset DB
	assert.NoError(t, migrator.Down(ctx))
	assert.NoError(t, migrator.Up(ctx))

	// Delete the first migration from the migrations_state table.
	_, err := db.GetSQLClient().Runner().Exec(ctx, `DELETE FROM migrations_state WHERE index = $1`, 0)
	require.NoError(t, err)

	migrator = migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(getTestMigrations()))
	err = migrator.Up(ctx)

	// Migration should now fail to start because the migrations_state table
	// is in an invalid state.
	require.Error(t, err)
	assert.Equal(t, "error initializing the migrator: migration 1 out of order. got: 0", err.Error())
}

func TestMigratorUpToSucceeds_MigrationAlreadyRan(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Run the migrations and verify that there was no error and the
	// data from the test migration is available.
	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(getTestMigrations()))
	// Reset DB
	assert.NoError(t, migrator.Down(ctx))
	assert.NoError(t, migrator.Up(ctx))

	// We should not run an error if we have already migrated to the latest version
	assert.NoError(t, migrator.UpTo(ctx, 1))
}

func TestMigratorUpToFails_OutOfRange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Run the migrations and verify that there was no error and the
	// data from the test migration is available.
	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(getTestMigrations()))
	err := migrator.UpTo(ctx, 100000)
	assert.Error(t, err)
	assert.Equal(t, "Index 100000 out of range", err.Error())
}

func TestMigratorUpFails_FailedMigration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Add a migration to test a successful `Up` call should complete successfully
	m := getTestMigrations()

	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(m))
	assert.NoError(t, migrator.Up(ctx))
	verifyTestMigrations(t, ctx, db.GetSQLClient(), "test_table")

	// Add two migrations, one that should fail and one that should succeed
	m = append(m,
		NewGoMigrationEntry(
			len(m),
			func(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
				// This one should fail due to primary key constraint
				query := `INSERT INTO test_table VALUES(0, 'c');`
				_, err := s.Runner().Exec(ctx, query)
				return err
			},
			func(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
				// This should not run at all
				query := `DELETE FROM test_table WHERE index = 0 AND data = 'c';`
				_, err := s.Runner().Exec(ctx, query)
				return err
			},
		),
		NewGoMigrationEntry(
			len(m)+1,
			func(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
				// This one is fine, but should not run because the previous one failed
				query := `INSERT INTO test_table VALUES(2, 'c');`
				_, err := s.Runner().Exec(ctx, query)
				return err
			},
			func(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
				// This should not run at all
				query := `DELETE FROM test_table WHERE index = 2 AND data = 'c';`
				_, err := s.Runner().Exec(ctx, query)
				return err
			},
		),
	)

	// The migrator should now fail due to the bad migration.
	migrator = migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(m))
	assert.Error(t, migrator.Up(ctx))

	// Test test migration should still be saved because that ran successfully
	// in a separate invocation of the migrator.
	verifyTestMigrations(t, ctx, db.GetSQLClient(), "test_table")
}

func TestMigratorDownSucceeds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Add a migration to test a successful `Down` call removes this migration.
	m := getTestMigrations()

	// Run the migration
	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(m))
	assert.NoError(t, migrator.Up(ctx))

	// The test migration should be available now.
	verifyTestMigrations(t, ctx, db.GetSQLClient(), "test_table")

	// Running down should have no error
	assert.NoError(t, migrator.Down(ctx))

	// The test migration should be undone.
	verifyNoTestMigrations(t, ctx, db.GetSQLClient(), "test_table")

	// All migrations should be removed.
	rows, err := db.GetSQLClient().Runner().Query(ctx, `
		SELECT
			index,
			created_at
		FROM
			migrations_state
		ORDER BY
			created_at ASC
	`)
	assert.NoError(t, err)

	defer rows.Close()

	for rows.Next() {
		assert.Fail(t, "There should be no migrations")
	}
}

func TestMigratorDownFails_FailedMigration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Add a migration to test that this data is still here if `Down` fails.
	m := getTestMigrations()

	// When we run the migrator, we should have two rows in the test migrations.
	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(m))
	assert.NoError(t, migrator.Up(ctx))
	verifyTestMigrationsN(t, ctx, db.GetSQLClient(), 2, "test_table")

	// Add a migrations whose `Down` fails.
	m = append(m,
		NewGoMigrationEntry(
			len(m),
			func(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
				// This row should still persist after `Down` fails
				query := `INSERT INTO test_table VALUES(2, 'c');`
				_, err := s.Runner().Exec(ctx, query)
				return err
			},
			func(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
				// This one should fail
				query := `
					INTENTIONAL SYNTAX ERROR;
					DELETE FROM test_table WHERE index = 2 AND data = 'c';
				`
				_, err := s.Runner().Exec(ctx, query)
				return err
			},
		),
		NewGoMigrationEntry(
			len(m)+1,
			func(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
				// This row should also still persist after `Down` fails
				query := `INSERT INTO test_table VALUES(3, 'd');`
				_, err := s.Runner().Exec(ctx, query)
				return err
			},
			func(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
				// This one should succeed
				query := `DELETE FROM test_table WHERE index = 3 AND data = 'd';`
				_, err := s.Runner().Exec(ctx, query)
				return err
			},
		),
	)

	// Run the two newly added migrations. This should succeed and create 2 more rows.
	migrator = migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(m))
	assert.NoError(t, migrator.Up(ctx))
	verifyTestMigrationsN(t, ctx, db.GetSQLClient(), 4, "test_table")

	// Undoing all of the migrations should fail.
	assert.Error(t, migrator.Down(ctx))

	// There should be 4 rows of data still.
	verifyTestMigrationsN(t, ctx, db.GetSQLClient(), 4, "test_table")
}

func TestMigratorDownToFails_MigrationNotRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Run the migrations and verify that there was no error and the
	// data from the test migration is available.
	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithMigrations(getTestMigrations()))

	// Start with fresh migrations
	assert.NoError(t, migrator.Down(ctx))
	assert.NoError(t, migrator.UpTo(ctx, 1))

	err := migrator.DownTo(ctx, 2)
	assert.Error(t, err)
	assert.Equal(t, "migrations up to/ past 2 have not been run", err.Error())
}

func TestMigratorManualUpSucceeds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Run the migrations and verify that there was no error and the
	// data from the test migration is available.
	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithManualMigrations(manualTestMigrations))
	assert.NoError(t, migrator.ManualUp(ctx, "test_migration_only"))
	verifyTestMigrations(t, ctx, db.GetSQLClient(), "test_manual_table")

	// check that the migration was created
	rows, err := db.GetSQLClient().Runner().Query(ctx, `
		SELECT
			id,
			created_at
		FROM
			manual_migrations_state
		ORDER BY
			created_at ASC
	`)
	assert.NoError(t, err)

	defer rows.Close()

	for rows.Next() {
		var id string
		var createdAt time.Time

		assert.NoError(t, rows.Scan(&id, &createdAt))
		assert.Equal(t, "test_migration_only", id)
	}

	// Does not run migration again if it has already been run
	err = migrator.ManualUp(ctx, "test_migration_only")
	assert.Error(t, err)
	assert.Equal(t, "This migration has already been run (\"test_migration_only\")", err.Error())
}

func TestMigratorManualUpFail_MigrationNotExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Run the migrations and verify that there was no error and the
	// data from the test migration is available.
	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithManualMigrations(manualTestMigrations))
	err := migrator.ManualUp(ctx, "fake_migration")
	assert.Error(t, err)
	assert.Equal(t, "Got invalid migration ID (\"fake_migration\")", err.Error())
}

func TestMigratorManualUpFail_MigrationFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Run the migrations and verify that there was no error and the
	// data from the test migration is available.
	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithManualMigrations(manualTestMigrations))
	assert.Error(t, migrator.ManualUp(ctx, "broken_up_migration"))
}

func TestMigratorManualDownSucceeds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Run the migration
	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithManualMigrations(manualTestMigrations))
	assert.NoError(t, migrator.ManualUp(ctx, "test_migration_only"))

	// The test migration should be available now.
	verifyTestMigrations(t, ctx, db.GetSQLClient(), "test_manual_table")

	// Running down should have no error
	assert.NoError(t, migrator.ManualDown(ctx, "test_migration_only"))

	// The test migration should be undone.
	verifyNoTestMigrations(t, ctx, db.GetSQLClient(), "test_manual_table")

	// All migrations should be removed.
	rows, err := db.GetSQLClient().Runner().Query(ctx, `
		SELECT
			id,
			created_at
		FROM
			manual_migrations_state
		ORDER BY
			created_at ASC
	`)
	assert.NoError(t, err)

	defer rows.Close()

	for rows.Next() {
		assert.Fail(t, "There should be no migrations")
	}

	// Does not do anything if down was already run, or we call down before up is called
	assert.NoError(t, migrator.ManualDown(ctx, "test_migration_only"))
}

func TestMigratorManualDownFail_MigrationNotExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Run the migrations and verify that there was no error and the
	// data from the test migration is available.
	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithManualMigrations(manualTestMigrations))
	err := migrator.ManualDown(ctx, "fake_migration")
	assert.Error(t, err)
	assert.Equal(t, "Got invalid migration ID (\"fake_migration\")", err.Error())
}

func TestMigratorManualDownFail_MigrationFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testdb.NewTestDatabase(ctx, t)

	// Run the migrations and verify that there was no error and the
	// data from the test migration is available.
	migrator := migrations.NewMigrator(db.GetSQLClient(), db.GetKVClient(), migrations.WithManualMigrations(manualTestMigrations))
	assert.NoError(t, migrator.ManualUp(ctx, "broken_down_migration"))
	verifyTestMigrationsN(t, ctx, db.GetSQLClient(), 1, "test_broken_manual_table")

	assert.Error(t, migrator.ManualDown(ctx, "broken_down_migration"))
	// manual migration should still exist
	verifyTestMigrationsN(t, ctx, db.GetSQLClient(), 1, "test_broken_manual_table")
}

type migrationFunc func(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error

type GoMigration struct{ up, down migrationFunc }

func (g *GoMigration) Up(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
	return g.up(ctx, s, kv)
}
func (g *GoMigration) Down(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
	return g.down(ctx, s, kv)
}

func NewGoMigrationEntry(i int, up, down migrationFunc) migrations.MigrationEntry {
	return migrations.MigrationEntry{
		Index:     i,
		Migration: &GoMigration{up, down},
	}
}

func getTestMigration(m []migrations.MigrationEntry) migrations.MigrationEntry {
	return migrations.MigrationEntry{
		Index: len(m),
		Name:  "Test migration",
		Migration: &GoMigration{
			up: func(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
				query := `
					CREATE TABLE IF NOT EXISTS test_table(
						index INTEGER PRIMARY KEY,
						data  TEXT
					);

					INSERT INTO test_table VALUES(0, 'a');
					INSERT INTO test_table VALUES(1, 'b');
				`
				_, err := s.Runner().Exec(ctx, query)
				return err
			},
			down: func(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
				query := `DROP TABLE IF EXISTS test_table;`
				_, err := s.Runner().Exec(ctx, query)
				return err
			},
		},
	}
}

func verifyTestMigrations(t *testing.T, ctx context.Context, sqlClient driver.SQLStore, tableName string) {
	verifyTestMigrationsN(t, ctx, sqlClient, 2, tableName)
}

func verifyTestMigrationsN(t *testing.T, ctx context.Context, sqlClient driver.SQLStore, n int, tableName string) {
	rows, err := sqlClient.Runner().Query(ctx, fmt.Sprintf(`SELECT index, data FROM %s ORDER BY index ASC `, tableName))
	assert.NoError(t, err)

	defer rows.Close()

	var i = 0
	for rows.Next() {
		var index int
		var data string
		assert.NoError(t, rows.Scan(&index, &data))
		assert.Equal(t, i, index)
		assert.Equal(t, string(rune('a'+i)), data)
		i++
	}

	assert.Equal(t, n, i)
}

func verifyNoTestMigrations(t *testing.T, ctx context.Context, sqlClient driver.SQLStore, tableName string) {
	_, err := sqlClient.Runner().Query(ctx, fmt.Sprintf(`SELECT index, data FROM %s`, tableName))
	assert.Error(t, err)
}

func TestMain(m *testing.M) {
	os.Exit(testdb.SetupDatabaseAndRunTests(m))
}
