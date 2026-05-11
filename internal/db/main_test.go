package db_test

import (
	"os"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/db/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.SetupDatabaseAndRunTests(m))
}
