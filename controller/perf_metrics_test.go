package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPerformanceSummaryExplicitDashboardRange(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.PerfMetric{}))
	for _, row := range []model.PerfMetric{
		{ModelName: "dashboard-range", Group: "auto", BucketTs: 3600, RequestCount: 4, SuccessCount: 3, TotalLatencyMs: 4000},
		{ModelName: "dashboard-range", Group: "auto", BucketTs: 7200, RequestCount: 1, TotalLatencyMs: 2000},
		{ModelName: "dashboard-range", Group: "auto", BucketTs: 10800, RequestCount: 10, SuccessCount: 10},
	} {
		require.NoError(t, db.Create(&row).Error)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/perf-metrics/summary?hours=1&start_timestamp=3661&end_timestamp=7300", nil)
	GetPerfMetricsSummary(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool                         `json:"success"`
		Data    perfmetrics.SummaryAllResult `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.NotNil(t, payload.Data.Summary)
	assert.Equal(t, 60.0, payload.Data.Summary.SuccessRate)
	assert.Equal(t, int64(1200), payload.Data.Summary.AvgLatencyMs)
	assert.Equal(t, int64(3600), payload.Data.WindowStart)
	assert.Equal(t, int64(7300), payload.Data.WindowEnd)
	for _, query := range []string{
		"start_timestamp=3661", "end_timestamp=7300", "start_timestamp=&end_timestamp=7300", "start_timestamp=bad&end_timestamp=7300", "start_timestamp=0&end_timestamp=7300", "start_timestamp=7300&end_timestamp=3661", "start_timestamp=3661&end_timestamp=7300&recent_model=dashboard-range",
	} {
		t.Run(query, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/perf-metrics/summary?"+query, nil)
			GetPerfMetricsSummary(ctx)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}
