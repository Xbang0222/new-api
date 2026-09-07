package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// withInvoiceDatabases runs invoice contracts against isolated real engines.
// MySQL/PostgreSQL are opt-in and require an empty disposable *_test database.
// The callback uses model globals; do not run these tests in parallel.
func withInvoiceDatabases(t *testing.T, test func(*testing.T)) {
	t.Helper()
	for _, dialect := range []common.DatabaseType{common.DatabaseTypeSQLite, common.DatabaseTypeMySQL, common.DatabaseTypePostgreSQL} {
		t.Run(string(dialect), func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case common.DatabaseTypeSQLite:
				driver = sqlite.Open(filepath.Join(t.TempDir(), "invoices.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate")
			case common.DatabaseTypeMySQL:
				dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case common.DatabaseTypePostgreSQL:
				dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(8)
			t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })
			if dialect != common.DatabaseTypeSQLite {
				query := "SELECT DATABASE()"
				if dialect == common.DatabaseTypePostgreSQL {
					query = "SELECT current_database()"
				}
				var name string
				require.NoError(t, db.Raw(query).Scan(&name).Error)
				require.True(t, strings.HasSuffix(name, "_test"), "refusing a non-test database")
			}
			models := []any{&model.User{}, &model.Log{}, &model.TopUp{}, &model.SubscriptionOrder{}, &model.InvoiceApplication{}, &model.InvoiceOrder{}, &model.InvoicePaymentOrder{}}
			for _, table := range models {
				require.False(t, db.Migrator().HasTable(table), "refusing an existing invoice fixture table: %T", table)
			}
			t.Cleanup(func() {
				for i := len(models) - 1; i >= 0; i-- {
					assert.NoError(t, db.Migrator().DropTable(models[i]))
				}
			})
			require.NoError(t, db.AutoMigrate(models...))
			previousDB, previousLogDB := model.DB, model.LOG_DB
			previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
			previousRedis, previousMemory, previousBatch := common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled
			model.DB, model.LOG_DB = db, db
			common.SetDatabaseTypes(dialect, dialect)
			common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled = false, false, false
			t.Cleanup(func() {
				model.DB, model.LOG_DB = previousDB, previousLogDB
				common.SetDatabaseTypes(previousMainType, previousLogType)
				common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled = previousRedis, previousMemory, previousBatch
			})
			versionQuery := "SELECT version()"
			if dialect == common.DatabaseTypeSQLite {
				versionQuery = "SELECT sqlite_version()"
			}
			var version string
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("invoice database %s: %s", dialect, version)
			test(t)
		})
	}
}
