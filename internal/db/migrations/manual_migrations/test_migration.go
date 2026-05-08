package manual_migrations

import (
	"context"

	"github.com/hovanhoa/llmgateway/pkg/driver"
)

type TestMigration struct{}

func (m *TestMigration) Up(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
	query := `
	CREATE TABLE IF NOT EXISTS test_manual_table(
		index INTEGER PRIMARY KEY,
		data  TEXT
	);

	INSERT INTO test_manual_table VALUES(0, 'a');
	INSERT INTO test_manual_table VALUES(1, 'b');
`
	_, err := s.Runner().Exec(ctx, query)
	return err
}

func (m *TestMigration) Down(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
	query := `DROP TABLE IF EXISTS test_manual_table;`
	_, err := s.Runner().Exec(ctx, query)
	return err
}

type BrokenUpMigration struct{}

func (m *BrokenUpMigration) Up(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
	// Should fail due to table not existing
	query := `INSERT INTO test_manual_table_2 VALUES(0, 'a');`
	_, err := s.Runner().Exec(ctx, query)
	return err
}

func (m *BrokenUpMigration) Down(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
	// This should not run at all
	return nil
}

type BrokenDownMigration struct{}

func (m *BrokenDownMigration) Up(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
	query := `
	CREATE TABLE IF NOT EXISTS test_broken_manual_table(
		index INTEGER PRIMARY KEY,
		data  TEXT
	);
	
	INSERT INTO test_broken_manual_table VALUES(0, 'a');`
	_, err := s.Runner().Exec(ctx, query)
	return err
}

func (m *BrokenDownMigration) Down(ctx context.Context, s driver.SQLStore, kv driver.KVStore) error {
	// This one should fail
	query := `
		INTENTIONAL SYNTAX ERROR;
		DELETE FROM test_broken_manual_table WHERE index = 0 AND data = 'a';
	`
	_, err := s.Runner().Exec(ctx, query)
	return err
}
