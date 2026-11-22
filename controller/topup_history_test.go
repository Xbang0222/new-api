package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopUpHistorySearchAndExportDatabaseMatrix(t *testing.T) {
	for _, engine := range []struct{ name, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		t.Run(engine.name, func(t *testing.T) {
			if engine.env != "" && os.Getenv(engine.env) == "" {
				t.Skip("set " + engine.env + " to test this database")
			}
			db := modelManagementDB(t, engine.name, os.Getenv(engine.env))
			require.NoError(t, db.AutoMigrate(&model.TopUp{}))
			alice := model.User{Username: "充值用户", AffCode: "history-alice"}
			bob := model.User{Username: "42", AffCode: "history-bob"}
			require.NoError(t, db.Create(&alice).Error)
			require.NoError(t, db.Create(&bob).Error)
			now := common.GetTimestamp()
			orders := []model.TopUp{
				{UserId: alice.Id, TradeNo: "ALICE-CNY", Amount: 10, Money: 99, PaidAmountMinor: 250, PaidCurrency: "CNY", Status: common.TopUpStatusSuccess, CreateTime: now, CompleteTime: now},
				{UserId: alice.Id, TradeNo: "ALICE-JPY", Amount: 10, Money: 99, PaidAmountMinor: 4500, PaidCurrency: "JPY", Status: common.TopUpStatusSuccess, CreateTime: now},
				{UserId: alice.Id, TradeNo: "ALICE-KWD", Amount: 10, Money: 99, PaidAmountMinor: 12345, PaidCurrency: "KWD", Status: common.TopUpStatusSuccess, CreateTime: now},
				{UserId: alice.Id, TradeNo: "ALICE-LEGACY", Amount: 10, Money: 88, Status: common.TopUpStatusSuccess, CreateTime: now},
				{UserId: alice.Id, TradeNo: "ALICE-OLD", Amount: 10, Money: 88, Status: common.TopUpStatusSuccess, CreateTime: 1},
				{UserId: bob.Id, TradeNo: "BOB-PENDING", Amount: 10, Money: 99, Status: common.TopUpStatusPending, CreateTime: now},
			}
			require.NoError(t, db.Create(&orders).Error)
			request := func(handler gin.HandlerFunc, query string, userID int) (bool, []model.TopUpHistoryRecord, int) {
				t.Helper()
				response := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(response)
				c.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup?"+query, nil)
				c.Set("id", userID)
				handler(c)
				var result struct {
					Success bool `json:"success"`
					Data    struct {
						Items       []model.TopUpHistoryRecord `json:"items"`
						Total       int                        `json:"total"`
						GeneratedAt int64                      `json:"generated_at"`
					} `json:"data"`
				}
				require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
				if result.Success {
					assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
					if c.Query("export") == "true" {
						assert.Positive(t, result.Data.GeneratedAt)
					}
				}
				return result.Success, result.Data.Items, result.Data.Total
			}

			t.Run("account searches and complete export", func(t *testing.T) {
				query := "username=" + url.QueryEscape(alice.Username)
				ok, page, total := request(GetAllTopUps, query+"&p=2&page_size=1", 0)
				require.True(t, ok)
				assert.Len(t, page, 1)
				assert.Equal(t, 5, total)
				ok, exported, total := request(GetAllTopUps, query+"&p=2&page_size=1&export=true", 0)
				require.True(t, ok)
				require.Len(t, exported, 5, "export must include all matching pages")
				assert.Equal(t, 5, total)
				payments := map[string]string{}
				for _, order := range exported {
					assert.Equal(t, alice.Id, order.UserId)
					assert.Equal(t, alice.Username, order.Username)
					payments[order.TradeNo] = order.PaidAmount
				}
				assert.Equal(t, "2.50", payments["ALICE-CNY"])
				assert.Equal(t, "4500", payments["ALICE-JPY"])
				assert.Equal(t, "12.345", payments["ALICE-KWD"])
				assert.Empty(t, payments["ALICE-LEGACY"], "do not infer a currency or actual payment from legacy Money")
				ok, selected, _ := request(GetAllTopUps, fmt.Sprintf("user_id=%d&keyword=ALICE-CNY&export=true", alice.Id), 0)
				require.True(t, ok)
				require.Len(t, selected, 1)
				assert.Equal(t, "ALICE-CNY", selected[0].TradeNo)
				ok, numericName, _ := request(GetAllTopUps, "username=42", 0)
				require.True(t, ok)
				require.Len(t, numericName, 1)
				assert.Equal(t, bob.Id, numericName[0].UserId)
			})
			t.Run("self export cannot select another account or extend the history window", func(t *testing.T) {
				ok, personal, total := request(GetUserTopUps, fmt.Sprintf("user_id=%d&username=42&export=true", bob.Id), alice.Id)
				require.True(t, ok)
				assert.Equal(t, 4, total)
				for _, order := range personal {
					assert.Equal(t, alice.Id, order.UserId)
					assert.NotEqual(t, "ALICE-OLD", order.TradeNo)
				}
				ok, _, _ = request(GetUserTopUps, "export=true", 0)
				assert.False(t, ok)
			})
			t.Run("invalid and escaped filters", func(t *testing.T) {
				for _, query := range []string{"user_id=0", "user_id=-1", "user_id=abc", "user_id=999999999999999999999999", "page_size=-1", "p=-1"} {
					ok, _, _ := request(GetAllTopUps, query, 0)
					assert.False(t, ok, query)
				}
				ok, items, _ := request(GetAllTopUps, "username="+url.QueryEscape("' OR 1=1 --"), 0)
				require.True(t, ok)
				assert.Empty(t, items)
				ok, items, _ = request(GetAllTopUps, "keyword=ALICE_CNY", 0)
				require.True(t, ok)
				assert.Empty(t, items, "underscore must not become a wildcard")
			})
			t.Run("deleted account retains its named history", func(t *testing.T) {
				require.NoError(t, db.Delete(&bob).Error)
				ok, items, _ := request(GetAllTopUps, "username=42&export=true", 0)
				require.True(t, ok)
				require.Len(t, items, 1)
				assert.Equal(t, bob.Username, items[0].Username)
			})
			t.Run("export limit rejects truncation", func(t *testing.T) {
				more := make([]model.TopUp, model.MaxTopUpHistoryExport+1)
				for i := range more {
					more[i] = model.TopUp{UserId: alice.Id, TradeNo: fmt.Sprintf("LIMIT-%04d", i), CreateTime: now}
				}
				require.NoError(t, db.CreateInBatches(more, 100).Error)
				ok, _, _ := request(GetAllTopUps, "keyword=LIMIT-%25&export=true", 0)
				assert.False(t, ok, "a partial statement must never be presented as a complete export")
			})
		})
	}
}
