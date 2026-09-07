package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// withCommerceDatabases runs the same behavior against SQLite and optional real
// MySQL/PostgreSQL databases. TEST_MYSQL_DSN and TEST_POSTGRES_DSN must point to
// disposable databases ending in "_test" with none of the fixture tables present.
// These tests replace package globals and must not use t.Parallel.
func withCommerceDatabases(t *testing.T, test func(*testing.T), extraModels ...any) {
	t.Helper()
	for _, databaseType := range []common.DatabaseType{
		common.DatabaseTypeSQLite,
		common.DatabaseTypeMySQL,
		common.DatabaseTypePostgreSQL,
	} {
		t.Run(string(databaseType), func(t *testing.T) {
			setupCommerceTestDB(t, databaseType, extraModels...)
			test(t)
		})
	}
}

func setupCommerceTestDB(t *testing.T, databaseType common.DatabaseType, extraModels ...any) {
	t.Helper()
	var dialector gorm.Dialector
	switch databaseType {
	case common.DatabaseTypeSQLite:
		dsn := filepath.Join(t.TempDir(), "commerce.db") +
			"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate"
		dialector = sqlite.Open(dsn)
	case common.DatabaseTypeMySQL:
		dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
		if dsn == "" {
			t.Skip("TEST_MYSQL_DSN is not configured")
		}
		dialector = mysql.Open(dsn)
	case common.DatabaseTypePostgreSQL:
		dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
		if dsn == "" {
			t.Skip("TEST_POSTGRES_DSN is not configured")
		}
		dialector = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
	default:
		t.Fatalf("unsupported commerce test database: %s", databaseType)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// Subscription transactions call GetDBTimestamp through the global DB. Keep
	// spare connections for that query and for concurrent purchase attempts.
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })

	if databaseType != common.DatabaseTypeSQLite {
		query := "SELECT DATABASE()"
		if databaseType == common.DatabaseTypePostgreSQL {
			query = "SELECT current_database()"
		}
		var databaseName string
		require.NoError(t, db.Raw(query).Scan(&databaseName).Error)
		require.True(t, strings.HasSuffix(databaseName, "_test"),
			"commerce fixtures require an isolated database with a name ending in _test")
	}

	models := append([]any{
		&User{}, &Log{}, &TopUp{}, &SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{},
	}, extraModels...)
	for _, model := range models {
		require.False(t, db.Migrator().HasTable(model),
			"refusing to use existing table for commerce fixture: %T", model)
	}
	t.Cleanup(func() {
		for i := len(models) - 1; i >= 0; i-- {
			assert.NoError(t, db.Migrator().DropTable(models[i]))
		}
	})
	require.NoError(t, db.AutoMigrate(models...))

	previousDB, previousLogDB := DB, LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis, previousMemory := common.RedisEnabled, common.MemoryCacheEnabled
	previousBatch, previousLogConsume := common.BatchUpdateEnabled, common.LogConsumeEnabled
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(databaseType, databaseType)
	common.RedisEnabled, common.MemoryCacheEnabled = false, false
	common.BatchUpdateEnabled, common.LogConsumeEnabled = false, true
	initCol()
	t.Cleanup(func() {
		assert.NoError(t, getSubscriptionPlanCache().Purge())
		assert.NoError(t, getSubscriptionPlanInfoCache().Purge())
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled, common.MemoryCacheEnabled = previousRedis, previousMemory
		common.BatchUpdateEnabled, common.LogConsumeEnabled = previousBatch, previousLogConsume
		initCol()
	})
	require.NoError(t, getSubscriptionPlanCache().Purge())
	require.NoError(t, getSubscriptionPlanInfoCache().Purge())

	versionQuery := "SELECT version()"
	if databaseType == common.DatabaseTypeSQLite {
		versionQuery = "SELECT sqlite_version()"
	}
	var version string
	require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
	t.Logf("commerce database %s: %s", databaseType, version)
}
