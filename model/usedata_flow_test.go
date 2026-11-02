package model

import (
	"errors"
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

func seedFlowQuotaData(t *testing.T, quotaData QuotaData) {
	t.Helper()
	require.NoError(t, DB.Create(&quotaData).Error)
}

func seedFlowLookupData(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Create(&Channel{Id: 1, Name: "east"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 2, Name: "west"}).Error)
	require.NoError(t, DB.Create(&Token{Id: 11, UserId: 1, Key: "sk-primary", Name: "primary"}).Error)
	require.NoError(t, DB.Create(&Token{Id: 22, UserId: 2, Key: "sk-backup", Name: "backup"}).Error)
	require.NoError(t, DB.Delete(&Token{Id: 11}).Error)
}

func TestGetFlowQuotaDataUsesQuotaDataRoleSpecificDimensions(t *testing.T) {
	truncateTables(t)
	seedFlowLookupData(t)

	seedFlowQuotaData(t, QuotaData{
		UserID:    1,
		Username:  "alice",
		NodeName:  "node-a",
		TokenID:   11,
		UseGroup:  "vip",
		ModelName: "gpt-a",
		ChannelID: 1,
		CreatedAt: 1000,
		Count:     2,
		Quota:     100,
		TokenUsed: 40,
	})
	seedFlowQuotaData(t, QuotaData{
		UserID:    1,
		Username:  "alice",
		NodeName:  "node-a",
		TokenID:   11,
		UseGroup:  "vip",
		ModelName: "gpt-a",
		ChannelID: 1,
		CreatedAt: 1100,
		Count:     1,
		Quota:     50,
		TokenUsed: 20,
	})
	seedFlowQuotaData(t, QuotaData{
		UserID:    1,
		Username:  "alice",
		NodeName:  "node-a",
		TokenID:   11,
		UseGroup:  "vip",
		ModelName: "gpt-a",
		ChannelID: 2,
		CreatedAt: 1200,
		Count:     1,
		Quota:     25,
		TokenUsed: 10,
	})
	seedFlowQuotaData(t, QuotaData{
		UserID:    2,
		Username:  "bob",
		NodeName:  "node-b",
		TokenID:   22,
		UseGroup:  "default",
		ModelName: "gpt-b",
		ChannelID: 1,
		CreatedAt: 1300,
		Count:     3,
		Quota:     70,
		TokenUsed: 30,
	})
	seedFlowQuotaData(t, QuotaData{
		UserID:    1,
		Username:  "alice",
		ModelName: "legacy",
		CreatedAt: 1400,
		Count:     99,
		Quota:     999,
		TokenUsed: 999,
	})

	rootRows, err := GetFlowQuotaData(900, 2000, "", 0, common.RoleRootUser)
	require.NoError(t, err)
	require.Len(t, rootRows, 3)
	// Token 11 was soft-deleted, so its name is intentionally left empty for the
	// frontend to render a localized "deleted (id)" label instead.
	require.Equal(t, FlowQuotaData{
		UserID:      1,
		Username:    "alice",
		NodeName:    "node-a",
		TokenID:     11,
		TokenName:   "",
		UseGroup:    "vip",
		ChannelID:   1,
		ChannelName: "east",
		ModelName:   "gpt-a",
		TokenUsed:   60,
		Count:       3,
		Quota:       150,
	}, *rootRows[0])
	// A token that still exists resolves to its current name.
	require.Equal(t, 22, rootRows[1].TokenID)
	require.Equal(t, "backup", rootRows[1].TokenName)

	adminRows, err := GetFlowQuotaData(900, 2000, "alice", 0, common.RoleAdminUser)
	require.NoError(t, err)
	require.Len(t, adminRows, 2)
	require.Equal(t, 0, adminRows[0].TokenID)
	require.Empty(t, adminRows[0].TokenName)
	require.Empty(t, adminRows[0].NodeName)
	require.Equal(t, "alice", adminRows[0].Username)
	require.Equal(t, "vip", adminRows[0].UseGroup)
	require.Equal(t, "east", adminRows[0].ChannelName)
	require.Equal(t, 150, adminRows[0].Quota)

	selfRows, err := GetFlowQuotaData(900, 2000, "", 1, common.RoleCommonUser)
	require.NoError(t, err)
	require.Len(t, selfRows, 1)
	require.Empty(t, selfRows[0].Username)
	require.Equal(t, 0, selfRows[0].ChannelID)
	require.Empty(t, selfRows[0].ChannelName)
	require.Empty(t, selfRows[0].TokenName)
	require.Equal(t, "vip", selfRows[0].UseGroup)
	require.Equal(t, 175, selfRows[0].Quota)
}

func TestLogQuotaDataSplitsRowsByUseGroupTokenChannelAndNode(t *testing.T) {
	truncateTables(t)
	CacheQuotaDataLock.Lock()
	CacheQuotaData = make(map[string]*QuotaData)
	CacheQuotaDataLock.Unlock()

	LogQuotaData(QuotaDataLogParams{
		UserID:    1,
		Username:  "alice",
		ModelName: "gpt-a",
		CreatedAt: 3661,
		UseGroup:  "vip",
		TokenID:   11,
		ChannelID: 1,
		NodeName:  "node-a",
		Quota:     100,
		TokenUsed: 40,
	})
	LogQuotaData(QuotaDataLogParams{
		UserID:    1,
		Username:  "alice",
		ModelName: "gpt-a",
		CreatedAt: 3700,
		UseGroup:  "vip",
		TokenID:   11,
		ChannelID: 1,
		NodeName:  "node-a",
		Quota:     50,
		TokenUsed: 20,
	})
	LogQuotaData(QuotaDataLogParams{
		UserID:    1,
		Username:  "alice",
		ModelName: "gpt-a",
		CreatedAt: 3700,
		UseGroup:  "default",
		TokenID:   11,
		ChannelID: 1,
		NodeName:  "node-a",
		Quota:     25,
		TokenUsed: 10,
	})

	SaveQuotaDataCache()

	var rows []QuotaData
	require.NoError(t, DB.Order("quota DESC").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Equal(t, int64(3600), rows[0].CreatedAt)
	require.Equal(t, "vip", rows[0].UseGroup)
	require.Equal(t, 11, rows[0].TokenID)
	require.Equal(t, 1, rows[0].ChannelID)
	require.Equal(t, "node-a", rows[0].NodeName)
	require.Equal(t, 2, rows[0].Count)
	require.Equal(t, 150, rows[0].Quota)
	require.Equal(t, 60, rows[0].TokenUsed)
	require.Equal(t, "default", rows[1].UseGroup)
	require.Equal(t, 25, rows[1].Quota)
}

func TestDashboardQuotaQueriesAndRetry(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(filepath.Join(t.TempDir(), "dashboard.db"))
			case "mysql":
				dsn := os.Getenv("TEST_DASHBOARD_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_DASHBOARD_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_DASHBOARD_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_DASHBOARD_POSTGRES_DSN is not configured")
				}
				driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })
			if dialect != "sqlite" {
				query := "SELECT DATABASE()"
				if dialect == "postgres" {
					query = "SELECT current_database()"
				}
				var name string
				require.NoError(t, db.Raw(query).Scan(&name).Error)
				require.True(t, strings.HasSuffix(name, "_test"), "requires isolated dashboard test database")
			}
			require.False(t, db.Migrator().HasTable(&QuotaData{}), "refusing to overwrite existing quota_data")
			require.NoError(t, db.AutoMigrate(&QuotaData{}))
			oldDB, oldCache := DB, CacheQuotaData
			DB, CacheQuotaData = db, make(map[string]*QuotaData)
			t.Cleanup(func() { DB, CacheQuotaData = oldDB, oldCache; assert.NoError(t, db.Migrator().DropTable(&QuotaData{})) })
			versionQuery := "SELECT version()"
			if dialect == "sqlite" {
				versionQuery = "SELECT sqlite_version()"
			}
			var version string
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("database version: %s", version)

			t.Run("all dashboard views include the overlapping first hour", func(t *testing.T) {
				for _, stamp := range []int64{3600, 7200, 10800} {
					require.NoError(t, db.Create(&QuotaData{UserID: 1, Username: "alice", ModelName: "a", UseGroup: "default", CreatedAt: stamp, Count: 1, Quota: 50, TokenUsed: 100}).Error)
				}
				for _, query := range []struct {
					name string
					run  func() ([]*QuotaData, error)
				}{
					{"all", func() ([]*QuotaData, error) { return GetAllQuotaDates(3661, 7300, "") }},
					{"username", func() ([]*QuotaData, error) { return GetQuotaDataByUsername("alice", 3661, 7300) }},
					{"self", func() ([]*QuotaData, error) { return GetQuotaDataByUserId(1, 3661, 7300) }},
					{"users", func() ([]*QuotaData, error) { return GetQuotaDataGroupByUser(3661, 7300) }},
				} {
					rows, err := query.run()
					require.NoError(t, err)
					assert.Len(t, rows, 2, query.name)
				}
				for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser} {
					rows, err := GetFlowQuotaData(3661, 7300, "alice", 1, role)
					require.NoError(t, err)
					require.Len(t, rows, 1)
					assert.Equal(t, 2, rows[0].Count)
				}
				ranking, total, err := GetTokenRanking(TokenRankingQuery{StartTime: 3661, EndTime: 7300})
				require.NoError(t, err)
				require.Len(t, ranking, 1)
				assert.Equal(t, int64(1), total)
				assert.Equal(t, int64(200), ranking[0].TokenUsed)
				rank, tokens, err := GetUserTokenRanking(TokenRankingQuery{StartTime: 3661, EndTime: 7300, UserID: 1})
				require.NoError(t, err)
				assert.Equal(t, 1, rank)
				assert.Equal(t, int64(200), tokens)
				require.NoError(t, db.Where("1 = 1").Delete(&QuotaData{}).Error)
			})

			for _, operation := range []string{"query", "create", "create-rollback", "update"} {
				t.Run(operation+" failure keeps only unpersisted usage for retry", func(t *testing.T) {
					require.NoError(t, db.Where("1 = 1").Delete(&QuotaData{}).Error)
					CacheQuotaData = make(map[string]*QuotaData)
					if operation == "update" || operation == "query" {
						require.NoError(t, db.Create(&QuotaData{UserID: 1, Username: "alice", ModelName: "retry", CreatedAt: 3600, Count: 1, Quota: 10, TokenUsed: 20}).Error)
					}
					LogQuotaData(QuotaDataLogParams{UserID: 1, Username: "alice", ModelName: "retry", CreatedAt: 3661, Quota: 30, TokenUsed: 40})
					LogQuotaData(QuotaDataLogParams{UserID: 1, Username: "alice", ModelName: "saved", CreatedAt: 3661, Quota: 50, TokenUsed: 60})
					fail := func(tx *gorm.DB) {
						if tx.Statement.Table != "quota_data" {
							return
						}
						if operation == "create" || operation == "create-rollback" {
							row, ok := tx.Statement.Dest.(*QuotaData)
							if !ok || row.ModelName != "retry" {
								return
							}
						}
						tx.AddError(errors.New("injected database failure"))
					}
					var remove func() error
					switch operation {
					case "query":
						require.NoError(t, db.Callback().Query().Before("gorm:query").Register("dashboard_failure", fail))
						remove = func() error { return db.Callback().Query().Remove("dashboard_failure") }
					case "create":
						require.NoError(t, db.Callback().Create().Before("gorm:create").Register("dashboard_failure", fail))
						remove = func() error { return db.Callback().Create().Remove("dashboard_failure") }
					case "create-rollback":
						require.NoError(t, db.Callback().Create().After("gorm:create").Before("gorm:commit_or_rollback_transaction").Register("dashboard_failure", fail))
						remove = func() error { return db.Callback().Create().Remove("dashboard_failure") }
					case "update":
						require.NoError(t, db.Callback().Update().Before("gorm:update").Register("dashboard_failure", fail))
						remove = func() error { return db.Callback().Update().Remove("dashboard_failure") }
					}
					t.Cleanup(func() {
						if remove != nil {
							assert.NoError(t, remove())
						}
					})
					SaveQuotaDataCache()
					require.NoError(t, remove())
					remove = nil
					if operation == "query" {
						beforeRetry, err := GetAllQuotaDates(3600, 7199, "")
						require.NoError(t, err)
						require.Len(t, beforeRetry, 1)
						assert.Equal(t, 1, beforeRetry[0].Count)
					}
					// Retry after the database recovers, then flush once more to detect repeats.
					SaveQuotaDataCache()
					SaveQuotaDataCache()
					rows, err := GetAllQuotaDates(3600, 7199, "")
					require.NoError(t, err)
					require.Len(t, rows, 2)
					totals := map[string]*QuotaData{}
					for _, row := range rows {
						totals[row.ModelName] = row
					}
					require.NotNil(t, totals["retry"])
					require.NotNil(t, totals["saved"])
					wantCount, wantQuota, wantTokens := 1, 30, 40
					if operation == "update" || operation == "query" {
						wantCount, wantQuota, wantTokens = 2, 40, 60
					}
					assert.Equal(t, wantCount, totals["retry"].Count)
					assert.Equal(t, wantQuota, totals["retry"].Quota)
					assert.Equal(t, wantTokens, totals["retry"].TokenUsed)
					assert.Equal(t, 1, totals["saved"].Count)
					assert.Equal(t, 50, totals["saved"].Quota)
					assert.Equal(t, 60, totals["saved"].TokenUsed)
				})
			}
		})
	}
}
