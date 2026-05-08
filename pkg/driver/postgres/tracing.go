package postgres

import (
	"context"
	"reflect"
	"strings"

	"github.com/hovanhoa/llmgateway/pkg/core/apm"
	"github.com/hovanhoa/llmgateway/pkg/driver"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type TracingRunner struct {
	config Config
	runner driver.SQLRunner
}

func NewTracingRunner(config Config, runner driver.SQLRunner) *TracingRunner {
	return &TracingRunner{config: config, runner: runner}
}

func (t *TracingRunner) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	span := t.setupSpan(ctx, query)
	defer span.Finish()

	return t.runner.Exec(ctx, query, convertArgs(args)...)
}

func (t *TracingRunner) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	span := t.setupSpan(ctx, query)
	defer span.Finish()

	return t.runner.Query(ctx, query, convertArgs(args)...)
}

func (t *TracingRunner) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	span := t.setupSpan(ctx, query)
	defer span.Finish()

	return t.runner.QueryRow(ctx, query, convertArgs(args)...)
}

// RawBytes wraps a []byte that should be sent as PostgreSQL bytea,
// bypassing the []byte→string conversion in convertByteSliceArgs.
type RawBytes []byte

// convertArgs normalizes query arguments for pgx/v5 compatibility.
//
// pgx/v5 maps Go types to PostgreSQL wire types strictly: []byte → bytea,
// named types → unrecognized. This function converts arguments that would
// otherwise fail
// To send actual bytea data, wrap the value in RawBytes.
func convertArgs(args []any) []any {
	for i, arg := range args {
		switch v := arg.(type) {
		case []byte:
			// pgx/v5 encodes []byte as bytea, but this codebase passes json.Marshal
			// output ([]byte) to JSONB columns. Converting to string makes pgx send
			// it as text, which PostgreSQL casts to JSONB.
			// A nil []byte must become untyped nil (SQL NULL), not "" (empty string),
			// because "" is not valid JSON. A typed nil like ([]byte)(nil) would
			// otherwise survive as non-nil in the interface and be converted to "".
			if v == nil {
				args[i] = nil
			} else {
				args[i] = string(v)
			}
		case RawBytes:
			// Escape hatch: unwrap back to plain []byte so pgx encodes it as bytea.
			// Used for columns that genuinely store binary data (none exist today).
			if v == nil {
				args[i] = nil
			} else {
				args[i] = []byte(v)
			}
		default:
			// Guard against untyped nil, which would panic in reflect.ValueOf.
			if arg == nil {
				continue
			}
			rv := reflect.ValueOf(arg)
			if rv.Kind() != reflect.Slice {
				continue
			}
			switch rv.Type().Elem().Kind() {
			case reflect.String:
				// Slices of named string types like []model.ServiceRequestUpdateType
				// are not recognized by pgx as text arrays. Convert to []string so
				// pgx sends them as PostgreSQL text[]. Plain []string is already
				// handled natively and skipped.
				if rv.Type() == reflect.TypeFor[[]string]() {
					continue
				}
				strs := make([]string, rv.Len())
				for j := range strs {
					strs[j] = rv.Index(j).String()
				}
				args[i] = strs
			case reflect.Uint8:
				// Named []byte types like json.RawMessage don't match the []byte
				// case above (Go type switches match exact types, not underlying
				// types). Apply the same []byte → string conversion via reflection.
				if rv.Type() == reflect.TypeFor[[]byte]() {
					continue
				}
				if rv.IsNil() {
					args[i] = nil
				} else {
					args[i] = string(rv.Bytes())
				}
			}
		}
	}
	return args
}

func (t *TracingRunner) setupSpan(ctx context.Context, query string) *apm.Span {
	hub := apm.GetHubFromContext(ctx)
	ctx = apm.SetHubOnContext(ctx, hub)

	span := apm.StartSpan(
		ctx,
		"db.query",
		apm.WithDescription(query),
	)

	operation, table := getSQLDescription(query)

	span.SetData("db.system", "postgresql")
	span.SetData("db.user", t.config.User)
	span.SetData("db.name", t.config.Name)
	span.SetData("db.statement", query)
	span.SetData("db.operation", operation)
	span.SetData("db.sql.table", table)

	span.SetData("server.port", t.config.Port)
	span.SetData("server.address", t.config.Host)

	return span
}

func getOperationName(query string) string {
	operations := []string{
		"ABORT",
		"ALTER AGGREGATE",
		"ALTER COLLATION",
		"ALTER CONVERSION",
		"ALTER DATABASE",
		"ALTER DEFAULT PRIVILEGES",
		"ALTER DOMAIN",
		"ALTER EVENT TRIGGER",
		"ALTER EXTENSION",
		"ALTER FOREIGN DATA WRAPPER",
		"ALTER FOREIGN TABLE",
		"ALTER FUNCTION",
		"ALTER GROUP",
		"ALTER INDEX",
		"ALTER LANGUAGE",
		"ALTER LARGE OBJECT",
		"ALTER MATERIALIZED VIEW",
		"ALTER OPERATOR",
		"ALTER OPERATOR CLASS",
		"ALTER OPERATOR FAMILY",
		"ALTER POLICY",
		"ALTER PROCEDURE",
		"ALTER PUBLICATION",
		"ALTER ROLE",
		"ALTER ROUTINE",
		"ALTER RULE",
		"ALTER SCHEMA",
		"ALTER SEQUENCE",
		"ALTER SERVER",
		"ALTER STATISTICS",
		"ALTER SUBSCRIPTION",
		"ALTER SYSTEM",
		"ALTER TABLE",
		"ALTER TABLESPACE",
		"ALTER TEXT SEARCH CONFIGURATION",
		"ALTER TEXT SEARCH DICTIONARY",
		"ALTER TEXT SEARCH PARSER",
		"ALTER TEXT SEARCH TEMPLATE",
		"ALTER TRIGGER",
		"ALTER TYPE",
		"ALTER USER",
		"ALTER USER MAPPING",
		"ALTER VIEW",
		"ANALYZE",
		"BEGIN",
		"CALL",
		"CHECKPOINT",
		"CLOSE",
		"CLUSTER",
		"COMMENT",
		"COMMIT",
		"COMMIT PREPARED",
		"COPY",
		"CREATE ACCESS METHOD",
		"CREATE AGGREGATE",
		"CREATE CAST",
		"CREATE COLLATION",
		"CREATE CONVERSION",
		"CREATE DATABASE",
		"CREATE DOMAIN",
		"CREATE EVENT TRIGGER",
		"CREATE EXTENSION",
		"CREATE FOREIGN DATA WRAPPER",
		"CREATE FOREIGN TABLE",
		"CREATE FUNCTION",
		"CREATE GROUP",
		"CREATE INDEX",
		"CREATE LANGUAGE",
		"CREATE MATERIALIZED VIEW",
		"CREATE OPERATOR",
		"CREATE OPERATOR CLASS",
		"CREATE OPERATOR FAMILY",
		"CREATE POLICY",
		"CREATE PROCEDURE",
		"CREATE PUBLICATION",
		"CREATE ROLE",
		"CREATE RULE",
		"CREATE SCHEMA",
		"CREATE SEQUENCE",
		"CREATE SERVER",
		"CREATE STATISTICS",
		"CREATE SUBSCRIPTION",
		"CREATE TABLE",
		"CREATE TABLE AS",
		"CREATE TABLESPACE",
		"CREATE TEXT SEARCH CONFIGURATION",
		"CREATE TEXT SEARCH DICTIONARY",
		"CREATE TEXT SEARCH PARSER",
		"CREATE TEXT SEARCH TEMPLATE",
		"CREATE TRANSFORM",
		"CREATE TRIGGER",
		"CREATE TYPE",
		"CREATE USER",
		"CREATE USER MAPPING",
		"CREATE VIEW",
		"DEALLOCATE",
		"DECLARE",
		"DELETE",
		"DISCARD",
		"DROP ACCESS METHOD",
		"DROP AGGREGATE",
		"DROP CAST",
		"DROP COLLATION",
		"DROP CONVERSION",
		"DROP DATABASE",
		"DROP DOMAIN",
		"DROP EVENT TRIGGER",
		"DROP EXTENSION",
		"DROP FOREIGN DATA WRAPPER",
		"DROP FOREIGN TABLE",
		"DROP FUNCTION",
		"DROP GROUP",
		"DROP INDEX",
		"DROP LANGUAGE",
		"DROP MATERIALIZED VIEW",
		"DROP OPERATOR",
		"DROP OPERATOR CLASS",
		"DROP OPERATOR FAMILY",
		"DROP OWNED",
		"DROP POLICY",
		"DROP PROCEDURE",
		"DROP PUBLICATION",
		"DROP ROLE",
		"DROP ROUTINE",
		"DROP RULE",
		"DROP SCHEMA",
		"DROP SEQUENCE",
		"DROP SERVER",
		"DROP STATISTICS",
		"DROP SUBSCRIPTION",
		"DROP TABLE",
		"DROP TABLESPACE",
		"DROP TEXT SEARCH CONFIGURATION",
		"DROP TEXT SEARCH DICTIONARY",
		"DROP TEXT SEARCH PARSER",
		"DROP TEXT SEARCH TEMPLATE",
		"DROP TRANSFORM",
		"DROP TRIGGER",
		"DROP TYPE",
		"DROP USER",
		"DROP USER MAPPING",
		"DROP VIEW",
		"EXECUTE",
		"EXPLAIN",
		"FETCH",
		"GRANT",
		"IMPORT FOREIGN SCHEMA",
		"INSERT",
		"LISTEN",
		"LOAD",
		"LOCK",
		"MERGE",
		"MOVE",
		"NOTIFY",
		"PREPARE",
		"PREPARE TRANSACTION",
		"REASSIGN OWNED",
		"REFRESH MATERIALIZED VIEW",
		"REINDEX",
		"RELEASE SAVEPOINT",
		"RESET",
		"REVOKE",
		"ROLLBACK",
		"ROLLBACK PREPARED",
		"ROLLBACK TO SAVEPOINT",
		"SAVEPOINT",
		"SECURITY LABEL",
		"SELECT",
		"SELECT INTO",
		"SET",
		"SET CONSTRAINTS",
		"SET ROLE",
		"SET SESSION AUTHORIZATION",
		"SET TRANSACTION",
		"SHOW",
		"START TRANSACTION",
		"TRUNCATE",
		"UNLISTEN",
		"UPDATE",
		"VACUUM",
		"VALUES",
	}

	for _, op := range operations {
		if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(query)), op) {
			return op
		}
	}

	return ""
}

func getSQLDescription(query string) (operation, table string) {
	tree, err := pg_query.Parse(query)
	if err != nil {
		return getOperationName(query), ""
	}

	if len(tree.GetStmts()) == 0 {
		return getOperationName(query), ""
	}

	if table = tree.GetStmts()[0].GetStmt().GetInsertStmt().GetRelation().GetRelname(); table != "" {
		operation = "INSERT"
		return
	}

	if table = tree.GetStmts()[0].GetStmt().GetUpdateStmt().GetRelation().GetRelname(); table != "" {
		operation = "UPDATE"
		return
	}

	if table = tree.GetStmts()[0].GetStmt().GetUpdateStmt().GetRelation().GetRelname(); table != "" {
		operation = "UPDATE"
		return
	}

	if clause := tree.GetStmts()[0].GetStmt().GetSelectStmt().GetFromClause(); len(clause) != 0 {
		if table = clause[0].GetRangeVar().GetRelname(); table != "" {
			operation = "SELECT"
			return
		}
	}

	// Try falling back if no matches
	return getOperationName(query), ""
}
