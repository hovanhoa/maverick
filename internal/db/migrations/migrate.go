package migrations

import (
	"context"
	"embed"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/core/log"
	"github.com/hovanhoa/llmgateway/pkg/core/retries"
	"github.com/hovanhoa/llmgateway/pkg/driver"
)

// Migration is an interface defining the operations a migration needs to be able to perform,
// regardless of the form it is in (SQL, Go, etc.). All migrations need a roll-forward and a
// rollback that completely do and undo the migration.
type Migration interface {
	// Up performs the migration in the forward direction (upgrade), i.e. it actually does the
	// migration. This operation should be entirely performed in the given transaction.
	Up(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error

	// Down perform the migration in the backward direction (downgrade), i.e. it undoes the
	// migration. This operation should be entirely performed in the given transaction.
	Down(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error
}

// MigrationEntry describes a single migration.
type MigrationEntry struct {
	// Index of the migration is a unique integer essentially describing its relative order.
	// In a []MigrationEntry, this value must be unique and monotonically increase as the
	// list goes on.
	Index int

	// Unique ID that identifies a specific manual migration entry.
	ID string

	// Name is a user-facing friendly name that describes this migration.
	Name string

	// Migration implements the actual migration (upgrade and downgrade) logic.
	Migration Migration
}

type MigrationState struct {
	Index     int
	ID        string
	Completed bool
}

// Migrator runs migrations on a database.
type Migrator struct {
	logger           *log.Logger
	database         driver.SQLStore
	kvStore          driver.KVStore
	migrations       []MigrationEntry
	targetEnv        *env.Environment
	manualMigrations map[string]MigrationEntry

	completed       map[int]bool
	manualCompleted map[string]bool
}

// MigrationOption specifies an option that modifies the behavior of the Migrator.
type MigrationOption func(migration *Migrator)

// NewMigrator creates a Migrator that performs migrations on the given database.
// Options can be used to override certain behaviors, but by default the migrator
// is configured to run the default migrations returned by `GetMigrations()` filtered
// by the current environment.
func NewMigrator(database driver.SQLStore, kvStore driver.KVStore, opts ...MigrationOption) *Migrator {
	// Create a new migrator with the current environment
	m := &Migrator{
		logger:           log.New(),
		database:         database,
		kvStore:          kvStore,
		migrations:       GetMigrations(),
		targetEnv:        env.GetEnvironment(),
		manualMigrations: GetManualMigrations(),
	}

	// Apply any specified options
	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (m *Migrator) SetLogger(logger *log.Logger) {
	m.logger = logger
}

// WithMigrations sets the migrator to use the given migrations, subject to any other
// option that impacts which migrations are run.
func WithMigrations(migrations []MigrationEntry) MigrationOption {
	return func(m *Migrator) {
		m.migrations = migrations
	}
}

func WithManualMigrations(migrations map[string]MigrationEntry) MigrationOption {
	return func(m *Migrator) {
		m.manualMigrations = migrations
	}
}

// RunUpMigrations runs migration up in the given environment. Based on the values passed in,
// we either run all migrations, migrations up to <index> or manually only run
// migration <id>.
func (m *Migrator) RunUpMigrations(ctx context.Context, index int, id string) error {
	var err error
	if index != -1 {
		err = m.UpTo(ctx, index)
	} else if id != "" {
		err = m.ManualUp(ctx, id)
	} else {
		err = m.Up(ctx)
	}

	if err != nil {
		return err
	}

	return nil
}

// RunDownMigrations runs migration down in the given environment. Based on the values passed in,
// we either run all migrations, migrations up to <index> or manually only run
// migration <id>.
func (m *Migrator) RunDownMigrations(ctx context.Context, index int, id string) error {
	var err error
	if index != -1 {
		err = m.DownTo(ctx, index)
	} else if id != "" {
		err = m.ManualDown(ctx, id)
	} else {
		err = m.Down(ctx)
	}

	if err != nil {
		return err
	}

	return nil
}

// GetMigrations returns the default list of migrations that should be run.
func GetMigrations() []MigrationEntry {
	return []MigrationEntry{
		{
			Index:     0,
			Name:      "Add account tables",
			Migration: NewSQLFileMigration("00000_account_tables"),
		},
		{
			Index:     1,
			Name:      "Add api_key table",
			Migration: NewSQLFileMigration("00001_api_key_table"),
		},
		{
			Index:     2,
			Name:      "Add usage_event table",
			Migration: NewSQLFileMigration("00002_usage_event_table"),
		},
	}
}

func GetManualMigrations() map[string]MigrationEntry {
	return map[string]MigrationEntry{}
}

func (m *Migrator) GetMigrationStates(ctx context.Context) ([]MigrationState, error) {
	var migrationStates []MigrationState

	// populate completion arrays
	err := m.Init(ctx)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(m.migrations); i++ {
		var migrationState MigrationState
		if m.completed[m.migrations[i].Index] {
			migrationState = MigrationState{
				Index:     i,
				Completed: true,
			}
		} else {
			migrationState = MigrationState{
				Index:     i,
				Completed: false,
			}
		}
		migrationStates = append(migrationStates, migrationState)
	}

	for key := range m.manualMigrations {
		var migrationState MigrationState
		if m.manualCompleted[key] {
			migrationState = MigrationState{
				ID:        key,
				Completed: true,
			}
		} else {
			migrationState = MigrationState{
				ID:        key,
				Completed: false,
			}
		}
		migrationStates = append(migrationStates, migrationState)
	}

	return migrationStates, nil
}

// Init checks some invariants and ensures that the migration table is created.
func (m *Migrator) Init(ctx context.Context) error {
	// reset map of migration state
	m.completed = make(map[int]bool)
	m.manualCompleted = make(map[string]bool)

	// preflight checks on the migrations
	for i := 0; i < len(m.migrations)-1; i++ {
		// check that migrations are in order and not duplicated
		if m.migrations[i].Index >= m.migrations[i+1].Index {
			return errors.New(
				"migrations %q (%d) and %q (%d) out of order",
				m.migrations[i].Name,
				m.migrations[i].Index,
				m.migrations[i+1].Name,
				m.migrations[i+1].Index,
			)
		}
	}

	return m.database.WithTransaction(ctx, func(ctx context.Context, s driver.SQLStore, r *retries.RetryControl) error {
		// check that the migrations state table is created
		table := `
			CREATE TABLE IF NOT EXISTS migrations_state(
				index      INTEGER PRIMARY KEY,
				created_at TIMESTAMP DEFAULT NOW()
			);
		`
		if _, err := s.Runner().Exec(ctx, table); err != nil {
			return errors.Wrap(err, "error creating migrations_state table")
		}

		// check that the manual migrations state table is created
		table = `
			CREATE TABLE IF NOT EXISTS manual_migrations_state(
				id         TEXT PRIMARY KEY,
				created_at TIMESTAMP DEFAULT NOW()
			);
		`
		if _, err := s.Runner().Exec(ctx, table); err != nil {
			return errors.Wrap(err, "error creating manual_migrations_state table")
		}

		// grab all executed migrations
		stmt := `SELECT index FROM migrations_state ORDER BY created_at ASC`
		migrationRows, err := s.Runner().Query(ctx, stmt)
		if err != nil {
			return errors.Wrap(err, "error running %s", stmt)
		}

		defer migrationRows.Close()

		i := 0
		for migrationRows.Next() {
			var index int
			if err := migrationRows.Scan(&index); err != nil {
				return errors.Wrap(err, "rows.Scan")
			}
			if index != m.migrations[i].Index {
				return errors.New("migration %d out of order. got: %d", index, m.migrations[i].Index)
			}

			m.completed[m.migrations[i].Index] = true
			i++
		}

		// grab all executed manual migrations
		stmt = `SELECT id FROM manual_migrations_state ORDER BY created_at ASC`
		manualMigrationRows, err := s.Runner().Query(ctx, stmt)
		if err != nil {
			return errors.Wrap(err, "error running %s", stmt)
		}

		defer manualMigrationRows.Close()

		for manualMigrationRows.Next() {
			var id string
			if err := manualMigrationRows.Scan(&id); err != nil {
				return errors.Wrap(err, "rows.Scan")
			}

			m.manualCompleted[id] = true
		}

		return nil
	})
}

// Up runs all pending migrations, starting from the last successfully completed one.
func (m *Migrator) Up(ctx context.Context) error {
	return m.UpTo(ctx, len(m.migrations)-1)
}

// Down undos all migrations starting from the one before the first uncompleted one.
func (m *Migrator) Down(ctx context.Context) error {
	return m.DownTo(ctx, -1)
}

// UpTo runs migrations up to the specified index
func (m *Migrator) UpTo(ctx context.Context, index int) error {
	if index >= len(m.migrations) || index < 0 {
		return errors.New("Index %d out of range", index)
	}

	// ensure that the migrator was initialized
	if err := m.Init(ctx); err != nil {
		return errors.Wrap(err, "error initializing the migrator")
	}

	// check that there are no gaps in migrations. this is just defensive programming.
	// if Init runs successfully, it should be impossible to error here.
	lastCompleted := -1
	for i := 0; i < len(m.migrations); i++ {
		if m.completed[m.migrations[i].Index] {
			lastCompleted = i
		} else {
			break
		}
	}

	if lastCompleted >= index {
		return nil
	}

	for i := lastCompleted + 1; i <= index; i++ {
		if m.completed[m.migrations[i].Index] {
			return errors.New("migration gap exists between %d and %d", lastCompleted, i)
		}
	}

	return m.database.WithTransaction(ctx, func(ctx context.Context, s driver.SQLStore, r *retries.RetryControl) error {
		// perform all migrations after the last completed one
		for i := lastCompleted + 1; i <= index; i++ {
			// run the migration
			if err := m.migrations[i].Migration.Up(ctx, s, m.kvStore); err != nil {
				return errors.Wrap(err, "error running up for migration %q", m.migrations[i].Name)
			}

			// insert the row into the migrations table
			sql, args, _ := s.Builder().
				Insert("migrations_state").
				Columns("index", "created_at").
				Values(m.migrations[i].Index, time.Now()).
				ToSql()

			if _, err := s.Runner().Exec(ctx, sql, args...); err != nil {
				return errors.Wrap(err, "error running %s [args: %v]", sql, args)
			}
		}

		return nil
	})
}

// DownTo runs migrations down to the specified index
func (m *Migrator) DownTo(ctx context.Context, index int) error {
	if index > len(m.migrations) || index < -1 {
		return errors.New("Index %d out of range", index)
	}

	// ensure that the migrator was initialized
	if err := m.Init(ctx); err != nil {
		return errors.Wrap(err, "error initializing the migrator")
	}

	// check that there are no gaps in migrations
	lastUncompleted := len(m.migrations)
	for i := len(m.migrations) - 1; i >= 0; i-- {
		if !m.completed[m.migrations[i].Index] {
			lastUncompleted = i
		} else {
			break
		}
	}

	if index >= lastUncompleted {
		return errors.New("migrations up to/ past %d have not been run", index)
	}

	for i := lastUncompleted - 1; i > index; i-- {
		if !m.completed[m.migrations[i].Index] {
			return errors.New("migration gap exists between %d and %d", lastUncompleted, i)
		}
	}

	m.logger.Debug("migration state", log.Int("lastUncompleted", lastUncompleted), log.Int("total", len(m.migrations)))

	return m.database.WithTransaction(ctx, func(ctx context.Context, s driver.SQLStore, r *retries.RetryControl) error {
		// undo all migrations before the first uncompleted one
		for i := lastUncompleted - 1; i > index; i-- {
			m.logger.Debug(
				"undoing migration",
				log.String("name", m.migrations[i].Name),
				log.Int("index", m.migrations[i].Index),
				log.Int("total", len(m.migrations)),
			)

			// undo the migration
			if err := m.migrations[i].Migration.Down(ctx, s, m.kvStore); err != nil {
				return errors.Wrap(err, "error running down for migration %q", m.migrations[i].Name)
			}

			// remove the row from the migrations table
			sql, args, _ := s.Builder().
				Delete("migrations_state").
				Where(sq.Eq{"index": m.migrations[i].Index}).
				ToSql()

			if _, err := s.Runner().Exec(ctx, sql, args...); err != nil {
				return errors.Wrap(err, "error running %s [args: %v]", sql, args)
			}
		}

		return nil
	})
}

// ManualUp runs up on a singular migration of id migrationID
func (m *Migrator) ManualUp(ctx context.Context, migrationID string) error {
	// check validity of the migrationID
	migration, ok := m.manualMigrations[migrationID]
	if !ok {
		return errors.New("Got invalid migration ID (%q)", migrationID)
	}

	// ensure that the migrator was initialized
	if err := m.Init(ctx); err != nil {
		return errors.Wrap(err, "error initializing the migrator")
	}

	if m.manualCompleted[migrationID] {
		return errors.New("This migration has already been run (%q)", migrationID)
	}

	return m.database.WithTransaction(ctx, func(ctx context.Context, s driver.SQLStore, r *retries.RetryControl) error {
		// perform single manual migration
		m.logger.Debug(
			"running migration",
			log.String("name", migration.Name),
			log.String("id", migrationID),
		)

		// run the migration
		if err := migration.Migration.Up(ctx, s, m.kvStore); err != nil {
			return errors.Wrap(err, "error running up for migration %q", migration.Name)
		}

		// insert the row into the migrations table
		sql, args, _ := s.Builder().
			Insert("manual_migrations_state").
			Columns("id", "created_at").
			Values(migrationID, time.Now()).
			ToSql()

		if _, err := s.Runner().Exec(ctx, sql, args...); err != nil {
			return errors.Wrap(err, "error running %s [args: %v]", sql, args)
		}

		return nil
	})
}

// ManualDown runs down on a singular migration of id migrationID
func (m *Migrator) ManualDown(ctx context.Context, migrationID string) error {
	// check validity of the migrationID
	migration, ok := m.manualMigrations[migrationID]
	if !ok {
		return errors.New("Got invalid migration ID (%q)", migrationID)
	}

	// ensure that the migrator was initialized
	if err := m.Init(ctx); err != nil {
		return errors.Wrap(err, "error initializing the migrator")
	}

	return m.database.WithTransaction(ctx, func(ctx context.Context, s driver.SQLStore, r *retries.RetryControl) error {
		// undo specified manual migration
		m.logger.Debug(
			"undoing migration",
			log.String("name", migration.Name),
			log.String("id", migrationID),
		)

		// undo the migration
		if err := migration.Migration.Down(ctx, s, m.kvStore); err != nil {
			return errors.Wrap(err, "error running down for migration %q", migration.Name)
		}

		// remove the row from the migrations table
		sql, args, _ := s.Builder().
			Delete("manual_migrations_state").
			Where(sq.Eq{"id": migrationID}).
			ToSql()

		if _, err := s.Runner().Exec(ctx, sql, args...); err != nil {
			return errors.Wrap(err, "error running %s [args: %v]", sql, args)
		}

		return nil
	})
}

//go:embed *.sql
var sqlMigrationFS embed.FS

type SQLFileMigration struct {
	filePrefix string
}

func NewSQLFileMigration(filePrefix string) *SQLFileMigration {
	return &SQLFileMigration{filePrefix: filePrefix}
}

func (m *SQLFileMigration) Up(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
	path := fmt.Sprintf("%s.up.sql", m.filePrefix)
	f, err := sqlMigrationFS.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "error reading path %q in embedded fs", path)
	}

	_, err = s.Runner().Exec(ctx, string(f))
	return err
}

func (m *SQLFileMigration) Down(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
	path := fmt.Sprintf("%s.down.sql", m.filePrefix)
	f, err := sqlMigrationFS.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "error reading path %q in embedded fs", path)
	}

	_, err = s.Runner().Exec(ctx, string(f))
	return err
}

type NoopMigration struct{}

func (m *NoopMigration) Up(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
	return nil
}

func (m *NoopMigration) Down(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
	return nil
}
