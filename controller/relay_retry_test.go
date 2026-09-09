package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configureIntelligentRetryGroupForTest(t *testing.T, group string) {
	t.Helper()
	original := operation_setting.GetChannelSchedulingSetting()
	originalGroups, err := common.Marshal(original.GroupStrategies)
	require.NoError(t, err)
	strategies := original.GroupStrategies
	strategies[group] = operation_setting.ChannelSchedulingStrategyIntelligent
	strategiesJSON, err := common.Marshal(strategies)
	require.NoError(t, err)
	require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
		"group_strategies": string(strategiesJSON),
	}))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
			"group_strategies": string(originalGroups),
		}))
	})
}

func TestIntelligentSchedulingRetriesAnyUpstreamStatusBeforeResponseStarts(t *testing.T) {
	const group = "controller-intelligent-retry"
	configureIntelligentRetryGroupForTest(t, group)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
	info := &relaycommon.RelayInfo{UpstreamStartTime: time.Now()}

	for _, statusCode := range []int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests, 524} {
		relayErr := types.NewOpenAIError(errors.New("upstream failed"), types.ErrorCodeBadResponseStatusCode, statusCode)
		assert.True(t, shouldRetry(c, info, relayErr, 1), "status code %d should fail over", statusCode)
	}
}

func TestIntelligentSchedulingDoesNotRetryLocalAbortOrStartedResponse(t *testing.T) {
	const group = "controller-intelligent-retry-boundary"
	configureIntelligentRetryGroupForTest(t, group)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
	startedAt := time.Now()
	info := &relaycommon.RelayInfo{StartTime: startedAt.Add(-time.Second), UpstreamStartTime: startedAt}

	clientCancel := types.NewOpenAIError(context.Canceled, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	assert.False(t, shouldRetry(c, info, clientCancel, 1))

	localError := types.NewErrorWithStatusCode(errors.New("invalid local request"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	assert.False(t, shouldRetry(c, &relaycommon.RelayInfo{}, localError, 1))

	_, err := c.Writer.Write([]byte("started"))
	require.NoError(t, err)
	assert.False(t, shouldRetry(c, info, types.NewOpenAIError(errors.New("late failure"), types.ErrorCodeBadResponse, 500), 1))

	streamContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(streamContext, constant.ContextKeyUsingGroup, group)
	streamInfo := &relaycommon.RelayInfo{
		StartTime:                startedAt.Add(-time.Second),
		UpstreamStartTime:        startedAt,
		UpstreamFirstContentTime: startedAt.Add(time.Second),
		IsStream:                 true,
	}
	assert.False(t, shouldRetry(streamContext, streamInfo, types.NewOpenAIError(errors.New("stream interrupted"), types.ErrorCodeBadResponse, 500), 1))
}

func TestRelayRetriesStopAtRequestAndResponseBoundaries(t *testing.T) {
	for _, intelligent := range []bool{false, true} {
		group := "controller-retry-boundary-legacy"
		if intelligent {
			group = "controller-retry-boundary-intelligent"
			configureIntelligentRetryGroupForTest(t, group)
		}
		for _, test := range []struct {
			name  string
			setup func(*gin.Context, *relaycommon.RelayInfo)
		}{
			{name: "request cancelled", setup: func(c *gin.Context, _ *relaycommon.RelayInfo) {
				ctx, cancel := context.WithCancel(c.Request.Context())
				cancel()
				c.Request = c.Request.WithContext(ctx)
			}},
			{name: "response body sent", setup: func(c *gin.Context, _ *relaycommon.RelayInfo) {
				_, err := c.Writer.Write([]byte(`{"partial":true}`))
				require.NoError(t, err)
			}},
			{name: "stream content sent", setup: func(_ *gin.Context, info *relaycommon.RelayInfo) {
				info.IsStream = true
				info.FirstResponseTime = time.Now()
			}},
			{name: "stream raw write without provider timing", setup: func(c *gin.Context, info *relaycommon.RelayInfo) {
				info.IsStream = true
				_, err := c.Writer.Write([]byte("data: partial\n\n"))
				require.NoError(t, err)
			}},
			{name: "stream meaningful content received", setup: func(_ *gin.Context, info *relaycommon.RelayInfo) {
				info.IsStream = true
				info.MarkUpstreamFirstContent()
			}},
			{name: "upgraded connection", setup: func(c *gin.Context, _ *relaycommon.RelayInfo) {
				c.Status(http.StatusSwitchingProtocols)
			}},
		} {
			t.Run(fmt.Sprintf("%s/%s", group, test.name), func(t *testing.T) {
				c := newPinRetryContext()
				common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
				info := &relaycommon.RelayInfo{StartTime: time.Now().Add(-time.Second), UpstreamStartTime: time.Now()}
				test.setup(c, info)
				relayErr := types.NewOpenAIError(errors.New("upstream failed"), types.ErrorCodeBadResponseStatusCode, 500)
				assert.False(t, shouldRetry(c, info, relayErr, 1))
				assert.False(t, shouldRetryTaskRelay(c, info, 1, &dto.TaskError{StatusCode: 500, Error: errors.New("upstream failed")}, 1))
			})
		}
	}
}

func TestRelayCanRetryAfterOnlyKeepaliveButNotAfterADataFrame(t *testing.T) {
	for _, intelligent := range []bool{false, true} {
		group := "controller-keepalive-legacy"
		if intelligent {
			group = "controller-keepalive-intelligent"
			configureIntelligentRetryGroupForTest(t, group)
		}
		c := newPinRetryContext()
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
		info := &relaycommon.RelayInfo{IsStream: true, UpstreamStartTime: time.Now()}
		relayErr := types.NewOpenAIError(errors.New("upstream failed"), types.ErrorCodeBadResponseStatusCode, 500)
		require.NoError(t, helper.PingData(c))
		require.NoError(t, helper.PingData(c))
		assert.True(t, shouldRetry(c, info, relayErr, 1), "keepalive alone permits failover")
		c.Writer.Header().Set("Content-Encoding", "gzip")
		assert.False(t, shouldRetry(c, info, relayErr, 1), "encoded response sizes must not be compared with keepalive input bytes")
		c.Writer.Header().Del("Content-Encoding")
		require.NoError(t, helper.StringData(c, `{"content":"hello"}`))
		assert.False(t, shouldRetry(c, info, relayErr, 1), "any data frame ends failover even without provider timing markers")
	}
}

func TestTaskRelayDoesNotRetryLocalErrorsOrPinnedChannels(t *testing.T) {
	for _, intelligent := range []bool{false, true} {
		group := "controller-task-retry-legacy"
		if intelligent {
			group = "controller-task-retry-intelligent"
			configureIntelligentRetryGroupForTest(t, group)
		}
		for _, status := range []int{http.StatusTooManyRequests, http.StatusTemporaryRedirect, http.StatusInternalServerError} {
			c := newPinRetryContext()
			common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
			info := &relaycommon.RelayInfo{UpstreamStartTime: time.Now()}
			assert.False(t, shouldRetryTaskRelay(c, info, 1, &dto.TaskError{StatusCode: status, LocalError: true, Error: errors.New("local failure")}, 1), "%s local status %d", group, status)
		}
		c := newPinRetryContext()
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
		info := &relaycommon.RelayInfo{UpstreamStartTime: time.Now()}
		service.GetChannelConstraints(c).AddPin(dto.ChannelPin{ChannelId: 1, Source: dto.PinSourceToken, Rank: dto.PinRankToken, RetryMode: dto.PinRetrySingleAttempt})
		assert.False(t, shouldRetryTaskRelay(c, info, 1, &dto.TaskError{StatusCode: 500, Error: errors.New("upstream failed")}, 1), "%s respects token pin", group)
	}
}

func TestLegacyRelayDoesNotRetryCancellationOrAnExhaustedChannelError(t *testing.T) {
	c := newPinRetryContext()
	info := &relaycommon.RelayInfo{}
	assert.False(t, shouldRetry(c, info, types.NewOpenAIError(context.Canceled, types.ErrorCodeDoRequestFailed, 500), 1))
	channelErr := types.NewOpenAIError(errors.New("invalid channel key"), types.ErrorCodeChannelInvalidKey, 500)
	assert.False(t, shouldRetry(c, info, channelErr, 0))
}
