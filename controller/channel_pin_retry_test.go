package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldRetryHonorsPinRetryMode(t *testing.T) {
	openaiErr := types.NewOpenAIError(errors.New("upstream"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)

	c := newPinRetryContext()
	assert.True(t, shouldRetry(c, nil, openaiErr, 1))

	origin := newPinRetryContext()
	service.GetChannelConstraints(origin).AddPin(dto.ChannelPin{
		ChannelId: 2,
		Source:    dto.PinSourceOriginTask,
		Rank:      dto.PinRankOriginTask,
		RetryMode: dto.PinRetrySameChannel,
	})
	assert.True(t, shouldRetry(origin, nil, openaiErr, 1), "origin pin retries on the same channel")

	token := newPinRetryContext()
	service.GetChannelConstraints(token).AddPin(dto.ChannelPin{
		ChannelId: 1,
		Source:    dto.PinSourceToken,
		Rank:      dto.PinRankToken,
		RetryMode: dto.PinRetrySingleAttempt,
	})
	assert.False(t, shouldRetry(token, nil, openaiErr, 1), "token pin suppresses retry")
}

func TestShouldRetryTaskRelayHonorsPinRetryMode(t *testing.T) {
	taskErr := &dto.TaskError{StatusCode: http.StatusInternalServerError}

	c := newPinRetryContext()
	assert.True(t, shouldRetryTaskRelay(c, nil, 1, taskErr, 1))

	origin := newPinRetryContext()
	service.GetChannelConstraints(origin).AddPin(dto.ChannelPin{
		ChannelId: 2,
		Source:    dto.PinSourceOriginTask,
		Rank:      dto.PinRankOriginTask,
		RetryMode: dto.PinRetrySameChannel,
	})
	assert.True(t, shouldRetryTaskRelay(origin, nil, 2, taskErr, 1))

	token := newPinRetryContext()
	service.GetChannelConstraints(token).AddPin(dto.ChannelPin{
		ChannelId: 1,
		Source:    dto.PinSourceToken,
		Rank:      dto.PinRankToken,
		RetryMode: dto.PinRetrySingleAttempt,
	})
	assert.False(t, shouldRetryTaskRelay(token, nil, 1, taskErr, 1))
}

func TestSameChannelPinsMergeToStricterRetryMode(t *testing.T) {
	c := newPinRetryContext()
	constraints := service.GetChannelConstraints(c)
	constraints.AddPin(dto.ChannelPin{
		ChannelId: 7,
		Source:    dto.PinSourceOriginTask,
		Rank:      dto.PinRankOriginTask,
		RetryMode: dto.PinRetrySameChannel,
	})
	constraints.AddPin(dto.ChannelPin{
		ChannelId: 7,
		Source:    dto.PinSourceToken,
		Rank:      dto.PinRankToken,
		RetryMode: dto.PinRetrySingleAttempt,
	})
	pin, found, overridden := constraints.ResolvedPin()
	require.True(t, found)
	assert.Equal(t, 7, pin.ChannelId)
	assert.Equal(t, dto.PinRetrySingleAttempt, pin.RetryMode)
	assert.Empty(t, overridden)
	assert.False(t, shouldRetry(c, nil, types.NewOpenAIError(errors.New("upstream"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError), 1))
}

func newPinRetryContext() *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c
}

func TestChannelAffinityRetryPriorities(t *testing.T) {
	for _, dialect := range []struct{ kind, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		t.Run(dialect.kind, func(t *testing.T) {
			dsn := ""
			if dialect.env != "" {
				dsn = os.Getenv(dialect.env)
				if dsn == "" {
					t.Skipf("%s not configured", dialect.env)
				}
			}
			db := modelManagementDB(t, dialect.kind, dsn)
			previousRetryTimes := common.RetryTimes
			previousGroups := setting.UserUsableGroups2JSONString()
			previousMaxGroups := setting.GetMaxTokenAutoGroups()
			affinity := operation_setting.GetChannelAffinitySetting()
			previousAffinity := *affinity
			t.Cleanup(func() {
				common.RetryTimes = previousRetryTimes
				*affinity = previousAffinity
				require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
				require.NoError(t, setting.UpdateMaxTokenAutoGroups(fmt.Sprint(previousMaxGroups)))
			})
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP","empty":"Empty"}`))
			require.NoError(t, setting.UpdateMaxTokenAutoGroups("2"))
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"empty":1}`))
			const modelName = "affinity-retry-test"
			for _, spec := range []struct {
				id       int
				priority int64
				group    string
			}{{24, 4, "default"}, {27, 3, "default"}, {30, 2, "default"}, {34, 4, "vip"}, {37, 3, "vip"}} {
				channel := model.Channel{
					Id: spec.id, Name: fmt.Sprintf("channel-%d", spec.id), Type: constant.ChannelTypeOpenAI,
					Key: "test-only", Status: common.ChannelStatusEnabled, Models: modelName, Group: spec.group,
					Priority: common.GetPointer(spec.priority), Weight: common.GetPointer(uint(1)),
				}
				require.NoError(t, db.Create(&channel).Error)
				require.NoError(t, channel.AddAbilities(db))
			}
			for _, cached := range []bool{false, true} {
				common.MemoryCacheEnabled = cached
				model.InitChannelCache()
				for _, tc := range []struct {
					name, mode string
					binding    int
					retries    int
					autoGroups []string
					crossGroup bool
					want       []int
				}{
					{name: "ordinary routing", mode: "off", retries: 2, want: []int{24, 27, 30}},
					{name: "affinity miss", mode: "prefer", retries: 2, want: []int{24, 27, 30}},
					{name: "affinity hit", mode: "prefer", binding: 27, retries: 2, want: []int{27, 24, 30}},
					{name: "no retry budget", mode: "prefer", binding: 27, want: []int{27}},
					{name: "one retry", mode: "prefer", binding: 27, retries: 1, want: []int{27, 24}},
					{name: "strict session", mode: "strict", binding: 27, retries: 2, want: []int{27}},
					{name: "auto group", mode: "prefer", binding: 27, retries: 1, autoGroups: []string{"default", "vip"}, want: []int{27, 24}},
					{name: "cross group", mode: "prefer", binding: 27, retries: 1, autoGroups: []string{"default", "vip"}, crossGroup: true, want: []int{27, 24, 34, 37}},
					{name: "skip empty group", mode: "prefer", binding: 27, retries: 1, autoGroups: []string{"empty", "default"}, want: []int{27, 24}},
				} {
					t.Run(fmt.Sprintf("cache=%t/%s", cached, tc.name), func(t *testing.T) {
						common.RetryTimes = tc.retries
						*affinity = operation_setting.ChannelAffinitySetting{
							Enabled: true, DefaultTTLSeconds: 60, MaxEntries: 100,
							Rules: []operation_setting.ChannelAffinityRule{{
								Name: t.Name(), ModelRegex: []string{"^" + modelName + "$"}, SessionMode: tc.mode,
								KeySources: []operation_setting.ChannelAffinityKeySource{{Type: "request_header", Key: "X-Session"}},
							}},
						}
						ctx := newPinRetryContext()
						ctx.Request.Header.Set("X-Session", t.Name())
						group := "default"
						common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
						if tc.autoGroups != nil {
							group = "auto"
							common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, tc.autoGroups)
							common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, tc.crossGroup)
						}
						if tc.binding != 0 {
							_, found := service.GetPreferredChannelByAffinity(ctx, modelName, group)
							require.False(t, found)
							service.RecordChannelAffinity(ctx, tc.binding)
							t.Cleanup(func() { service.ClearCurrentChannelAffinityCache(ctx) })
						}
						// HTTP distribution and the relay loop use separate RetryParam values.
						channel, _, selectionErr := service.SelectChannelForRequest(ctx, modelName, &service.RetryParam{Ctx: ctx, TokenGroup: group, ModelName: modelName})
						require.Nil(t, selectionErr)
						require.NotNil(t, channel)
						retry := &service.RetryParam{Ctx: ctx, TokenGroup: group, ModelName: modelName}
						upstreamErr := types.NewOpenAIError(errors.New("upstream failure"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
						var attempts []int
						attemptLimit := service.GetRelayAttemptLimit(retry)
						for attempt := 0; attempt < attemptLimit; attempt++ {
							if len(attempts) > 0 {
								retry.IncreaseRetry()
								var err error
								channel, _, err = service.CacheGetRandomSatisfiedChannel(retry)
								require.NoError(t, err)
								if channel == nil {
									break
								}
							}
							service.AppendUsedChannel(ctx, channel.Id)
							retry.MarkChannelAttempted(channel)
							attempts = append(attempts, channel.Id)
							require.LessOrEqual(t, len(attempts), len(tc.want), "retry budget must not increase")
							if !shouldRetry(ctx, nil, upstreamErr, attemptLimit-attempt-1) {
								break
							}
						}
						assert.Equal(t, tc.want, attempts)
					})
				}
			}
		})
	}
}
