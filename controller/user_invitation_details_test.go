package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInvitationDetailsLegacyDB(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, db.Exec(`
		CREATE TABLE top_ups (
			id integer PRIMARY KEY,
			user_id integer,
			amount integer,
			money real,
			trade_no text,
			payment_method text,
			payment_provider text,
			create_time integer,
			complete_time integer,
			status text
		)
	`).Error)

	t.Cleanup(func() {
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestGetInvitationDetailsSucceedsWithLegacyTables(t *testing.T) {
	setupInvitationDetailsLegacyDB(t)

	require.NoError(t, model.DB.Create(&model.User{
		Id:              41,
		Username:        "details_inviter",
		DisplayName:     "Inviter",
		Status:          common.UserStatusEnabled,
		AffCount:        1,
		AffQuota:        500,
		AffHistoryQuota: 500,
		AffCode:         "INV41",
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:          42,
		Username:    "details_invitee",
		DisplayName: "Invitee",
		Status:      common.UserStatusEnabled,
		InviterId:   41,
		AffCode:     "INV42",
		CreatedAt:   1779783729,
	}).Error)
	require.NoError(t, model.DB.Exec(
		"INSERT INTO top_ups (id, user_id, amount, money, trade_no, payment_method, payment_provider, create_time, complete_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		43, 42, 2, 2, "reward-details-legacy", model.PaymentMethodStripe, model.PaymentProviderStripe, int64(100), int64(200), common.TopUpStatusSuccess,
	).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/aff/details", nil)
	ctx.Set("id", 41)

	GetInvitationDetails(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			InvitedUsers  []model.InvitedUserDetail   `json:"invited_users"`
			InvitedTotal  int64                       `json:"invited_total"`
			RebateDetails []model.InviterRebateDetail `json:"rebate_details"`
			RebateTotal   int64                       `json:"rebate_total"`
			AffCount      int                         `json:"aff_count"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.EqualValues(t, 1, payload.Data.InvitedTotal)
	require.Len(t, payload.Data.InvitedUsers, 1)
	require.Equal(t, 42, payload.Data.InvitedUsers[0].Id)
	require.EqualValues(t, 1779783729, payload.Data.InvitedUsers[0].CreatedAt)
	require.EqualValues(t, 1, payload.Data.RebateTotal)
	require.Len(t, payload.Data.RebateDetails, 1)
	require.Equal(t, "reward-details-legacy", payload.Data.RebateDetails[0].TradeNo)
	require.Equal(t, 1, payload.Data.AffCount)
}

func TestGetAffCodeCanReturnInvitationDetails(t *testing.T) {
	setupInvitationDetailsLegacyDB(t)

	require.NoError(t, model.DB.Create(&model.User{
		Id:          51,
		Username:    "details_query_inviter",
		DisplayName: "Inviter",
		Status:      common.UserStatusEnabled,
		AffCount:    1,
		AffCode:     "INV51",
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:          52,
		Username:    "details_query_invitee",
		DisplayName: "Invitee",
		Status:      common.UserStatusEnabled,
		InviterId:   51,
		AffCode:     "INV52",
		CreatedAt:   1779783729,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/aff?details=1", nil)
	ctx.Set("id", 51)

	GetAffCode(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			InvitedUsers []model.InvitedUserDetail `json:"invited_users"`
			InvitedTotal int64                     `json:"invited_total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.EqualValues(t, 1, payload.Data.InvitedTotal)
	require.Len(t, payload.Data.InvitedUsers, 1)
	require.Equal(t, 52, payload.Data.InvitedUsers[0].Id)
	require.EqualValues(t, 1779783729, payload.Data.InvitedUsers[0].CreatedAt)
}

func TestInvitationRankingDatabaseMatrix(t *testing.T) {
	for _, database := range []struct{ kind, env string }{
		{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"},
	} {
		t.Run(database.kind, func(t *testing.T) {
			dsn := os.Getenv(database.env)
			if database.env != "" && dsn == "" {
				t.Skip("database DSN not configured")
			}
			db, _ := newAuditTestDatabase(t, database.kind, dsn)
			previous := model.DB
			model.DB = db
			t.Cleanup(func() { model.DB = previous })
			require.NoError(t, db.AutoMigrate(&model.User{}))
			for i, quota := range []int{900, 800, 800, 600, 500, 400, 0, 1000} {
				user := model.User{Id: i + 1, Username: fmt.Sprintf("rank_user_%d", i+1), AffCode: fmt.Sprintf("rank_%d", i+1), AffHistoryQuota: quota}
				require.NoError(t, db.Create(&user).Error)
			}
			require.NoError(t, db.Delete(&model.User{}, 8).Error)
			for _, viewer := range []struct{ id, role int }{
				{3, common.RoleCommonUser}, {99, common.RoleAdminUser}, {99, common.RoleRootUser},
			} {
				recorder := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(recorder)
				ctx.Request = httptest.NewRequest(http.MethodGet, "/api/data/invitation-ranking?role=100&id=1&limit=100", nil)
				ctx.Set("id", viewer.id)
				ctx.Set("role", viewer.role)
				GetInvitationRanking(ctx)
				assert.Equal(t, http.StatusOK, recorder.Code)
				assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
				var payload struct {
					Success bool `json:"success"`
					Data    []struct {
						Username string `json:"username"`
						Quota    int    `json:"aff_history_quota"`
						Rank     int    `json:"rank"`
						IsSelf   bool   `json:"is_self"`
					} `json:"data"`
				}
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
				require.True(t, payload.Success)
				require.Len(t, payload.Data, 5)
				assert.NotContains(t, recorder.Body.String(), `"id":`)
				for i, entry := range payload.Data {
					assert.Equal(t, i+1, entry.Rank)
					assert.Equal(t, []int{900, 800, 800, 600, 500}[i], entry.Quota)
					assert.Equal(t, viewer.id == i+1, entry.IsSelf)
					username := fmt.Sprintf("rank_user_%d", i+1)
					if viewer.role >= common.RoleAdminUser || entry.IsSelf {
						assert.Equal(t, username, entry.Username)
					} else {
						assert.NotEqual(t, username, entry.Username)
						assert.Contains(t, entry.Username, "*")
					}
				}
			}
		})
	}
}
