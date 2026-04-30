package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCalcPurchaseWindowStart(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		plan         *SubscriptionPlan
		expectedZero bool
		expected     int64
	}{
		{
			name:         "nil plan returns 0",
			plan:         nil,
			expectedZero: true,
		},
		{
			name: "year duration",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationYear,
				DurationValue: 1,
			},
			expected: now.AddDate(-1, 0, 0).Unix(),
		},
		{
			name: "year duration multi-year",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationYear,
				DurationValue: 3,
			},
			expected: now.AddDate(-3, 0, 0).Unix(),
		},
		{
			name: "month duration",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationMonth,
				DurationValue: 1,
			},
			expected: now.AddDate(0, -1, 0).Unix(),
		},
		{
			name: "month duration multi-month",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationMonth,
				DurationValue: 6,
			},
			expected: now.AddDate(0, -6, 0).Unix(),
		},
		{
			name: "day duration",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationDay,
				DurationValue: 7,
			},
			expected: now.Add(-7 * 24 * time.Hour).Unix(),
		},
		{
			name: "hour duration",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationHour,
				DurationValue: 6,
			},
			expected: now.Add(-6 * time.Hour).Unix(),
		},
		{
			name: "custom duration with seconds",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationCustom,
				CustomSeconds: 3600,
			},
			expected: now.Add(-3600 * time.Second).Unix(),
		},
		{
			name: "custom with zero seconds returns 0",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationCustom,
				CustomSeconds: 0,
			},
			expectedZero: true,
		},
		{
			name: "custom with negative seconds returns 0",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationCustom,
				CustomSeconds: -100,
			},
			expectedZero: true,
		},
		{
			name: "month with zero value returns 0",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationMonth,
				DurationValue: 0,
			},
			expectedZero: true,
		},
		{
			name: "year with negative value returns 0",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationYear,
				DurationValue: -1,
			},
			expectedZero: true,
		},
		{
			name: "day with zero value returns 0",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationDay,
				DurationValue: 0,
			},
			expectedZero: true,
		},
		{
			name: "hour with zero value returns 0",
			plan: &SubscriptionPlan{
				DurationUnit:  SubscriptionDurationHour,
				DurationValue: 0,
			},
			expectedZero: true,
		},
		{
			name: "unknown duration unit returns 0",
			plan: &SubscriptionPlan{
				DurationUnit:  "fortnight",
				DurationValue: 1,
			},
			expectedZero: true,
		},
		{
			name: "empty duration unit returns 0",
			plan: &SubscriptionPlan{
				DurationUnit:  "",
				DurationValue: 1,
			},
			expectedZero: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := calcPurchaseWindowStart(now, tc.plan)
			if tc.expectedZero {
				assert.Equal(t, int64(0), got)
				return
			}
			assert.Equal(t, tc.expected, got)
		})
	}
}

func insertSubscriptionPlanForLimitTest(t *testing.T, id int, unit string, value int) *SubscriptionPlan {
	t.Helper()
	plan := &SubscriptionPlan{
		Id:                 id,
		Title:              "Limit Plan",
		PriceAmount:        1.00,
		Currency:           "USD",
		DurationUnit:       unit,
		DurationValue:      value,
		Enabled:            true,
		TotalAmount:        1000,
		MaxPurchasePerUser: 1,
	}
	require.NoError(t, DB.Create(plan).Error)
	return plan
}

func insertUserSubscriptionForLimitTest(t *testing.T, userId, planId int, createdAt int64) {
	t.Helper()
	sub := &UserSubscription{
		UserId:    userId,
		PlanId:    planId,
		Status:    "active",
		StartTime: createdAt,
		EndTime:   createdAt + 30*86400,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	// BeforeCreate hook overwrites CreatedAt to now(); skip hooks so the
	// historic timestamp survives for window-filter testing.
	require.NoError(t, DB.Session(&gorm.Session{SkipHooks: true}).Create(sub).Error)
}

func TestCountPurchasesInWindow(t *testing.T) {
	common.UsingSQLite = true
	truncateTables(t)

	plan := insertSubscriptionPlanForLimitTest(t, 9001, SubscriptionDurationMonth, 1)
	userId := 7001

	t.Run("no purchases returns 0", func(t *testing.T) {
		count, err := CountPurchasesInWindow(userId, plan)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	now := time.Now().Unix()

	t.Run("purchase inside window counted", func(t *testing.T) {
		// 5 days ago, well within 30-day window
		insertUserSubscriptionForLimitTest(t, userId, plan.Id, now-5*86400)
		count, err := CountPurchasesInWindow(userId, plan)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("purchase outside window not counted", func(t *testing.T) {
		// 60 days ago — well outside 30-day window for a monthly plan
		insertUserSubscriptionForLimitTest(t, userId, plan.Id, now-60*86400)
		count, err := CountPurchasesInWindow(userId, plan)
		require.NoError(t, err)
		// still 1 — the 5-day-old one inside, the 60-day-old one outside
		assert.Equal(t, int64(1), count)
	})

	t.Run("purchase by different user not counted", func(t *testing.T) {
		insertUserSubscriptionForLimitTest(t, userId+1, plan.Id, now-1*86400)
		count, err := CountPurchasesInWindow(userId, plan)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("purchase for different plan not counted", func(t *testing.T) {
		otherPlan := insertSubscriptionPlanForLimitTest(t, 9002, SubscriptionDurationMonth, 1)
		insertUserSubscriptionForLimitTest(t, userId, otherPlan.Id, now-1*86400)
		count, err := CountPurchasesInWindow(userId, plan)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("invalid args return error", func(t *testing.T) {
		_, err := CountPurchasesInWindow(0, plan)
		require.Error(t, err)
		_, err = CountPurchasesInWindow(userId, nil)
		require.Error(t, err)
	})
}

func TestCountPurchasesInWindow_NoDurationFallsBackToLifetime(t *testing.T) {
	common.UsingSQLite = true
	truncateTables(t)

	// Plan with unknown duration_unit → window=0 → fallback to lifetime count
	plan := insertSubscriptionPlanForLimitTest(t, 9100, "fortnight", 1)
	userId := 7100

	now := time.Now().Unix()
	// Insert one very old purchase (10 years ago) — should still count under lifetime fallback
	insertUserSubscriptionForLimitTest(t, userId, plan.Id, now-10*365*86400)

	count, err := CountPurchasesInWindow(userId, plan)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "unknown duration_unit should fall back to lifetime count")
}

func TestCountPurchasesInWindow_DayWindow(t *testing.T) {
	common.UsingSQLite = true
	truncateTables(t)

	// 7-day window
	plan := insertSubscriptionPlanForLimitTest(t, 9200, SubscriptionDurationDay, 7)
	userId := 7200
	now := time.Now().Unix()

	insertUserSubscriptionForLimitTest(t, userId, plan.Id, now-3*86400) // 3 days ago: inside
	insertUserSubscriptionForLimitTest(t, userId, plan.Id, now-8*86400) // 8 days ago: outside

	count, err := CountPurchasesInWindow(userId, plan)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "only the 3-day-old purchase should be inside a 7-day window")
}

func TestCountPurchasesInWindow_HourWindow(t *testing.T) {
	common.UsingSQLite = true
	truncateTables(t)

	// 24-hour window
	plan := insertSubscriptionPlanForLimitTest(t, 9300, SubscriptionDurationHour, 24)
	userId := 7300
	now := time.Now().Unix()

	insertUserSubscriptionForLimitTest(t, userId, plan.Id, now-3600)        // 1h ago: inside
	insertUserSubscriptionForLimitTest(t, userId, plan.Id, now-25*3600)     // 25h ago: outside
	insertUserSubscriptionForLimitTest(t, userId, plan.Id, now-23*3600-100) // ~23h ago: inside

	count, err := CountPurchasesInWindow(userId, plan)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestCountPurchasesInWindow_CustomSecondsWindow(t *testing.T) {
	common.UsingSQLite = true
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:                 9400,
		Title:              "Custom Plan",
		PriceAmount:        1.00,
		Currency:           "USD",
		DurationUnit:       SubscriptionDurationCustom,
		CustomSeconds:      300, // 5 minutes
		Enabled:            true,
		TotalAmount:        1000,
		MaxPurchasePerUser: 1,
	}
	require.NoError(t, DB.Create(plan).Error)

	userId := 7400
	now := time.Now().Unix()

	insertUserSubscriptionForLimitTest(t, userId, plan.Id, now-100) // 100s ago: inside
	insertUserSubscriptionForLimitTest(t, userId, plan.Id, now-400) // 400s ago: outside

	count, err := CountPurchasesInWindow(userId, plan)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

// Regression guard documenting the old vs new semantics. Under the original
// upstream behavior, MaxPurchasePerUser=1 became a lifetime cap, so even
// purchases from many days ago would block a new buy. The new behavior
// counts only within a rolling window equal to the plan's duration.
//
// Note: this test exercises countPurchasesInWindowTx (the function used
// internally by every enforce site). A full CreateUserSubscriptionFromPlanTx
// integration test is intentionally omitted because its body calls
// GetDBTimestamp(), which opens a fresh DB.Raw query — this deadlocks
// under the test harness's single-connection SQLite setup, even though
// it works fine in production (which has multiple connections). The
// actual enforce branch (the simple `if count >= limit` after calling
// the helper) is trivially correct and covered by the helper tests.
func TestPurchaseLimitWindow_LifetimeRegressionGuard(t *testing.T) {
	common.UsingSQLite = true
	truncateTables(t)

	// Weekly plan, max 1 per cycle.
	plan := insertSubscriptionPlanForLimitTest(t, 9600, SubscriptionDurationDay, 7)
	userId := 7600

	// Two old purchases from 10 days ago — both outside the 7-day window.
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour).Unix()
	insertUserSubscriptionForLimitTest(t, userId, plan.Id, tenDaysAgo)
	insertUserSubscriptionForLimitTest(t, userId, plan.Id, tenDaysAgo)

	// Under OLD lifetime semantics: count would be 2 ≥ 1, blocking new purchase.
	// Under NEW cycle semantics: both old purchases fall outside the 7-day
	// window, so window-count is 0 < 1 and a new purchase is allowed.
	count, err := CountPurchasesInWindow(userId, plan)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "old purchases outside window must not count toward limit")
	assert.True(t, count < int64(plan.MaxPurchasePerUser), "user must not be blocked from buying after window slides")
}
