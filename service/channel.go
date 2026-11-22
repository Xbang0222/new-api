package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

func shouldCloseActiveWebSocketsAfterDisable(channelId int) (bool, *model.Channel) {
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.SysLog(common.LogText("failed to check channel status before closing active websockets: channel_id=%d, error=%v", channelId, err))
		return true, nil
	}
	return channel.Status != common.ChannelStatusEnabled, channel
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(common.LogText("channel %q (#%d) failed, disabling it, reason: %s", channelError.ChannelName, channelError.ChannelId, common.LocalLogPreview(reason)))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(common.LogText("channel %q (#%d) has automatic disabling turned off, skipping", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		closeWebSockets, channel := shouldCloseActiveWebSocketsAfterDisable(channelError.ChannelId)
		if closeWebSockets {
			CloseActiveWebSocketsForChannel(channelError.ChannelId, ChannelDisabledCloseReason)
		}
		params := map[string]any{"Name": channelError.ChannelName, "Id": channelError.ChannelId, "Reason": reason}
		NotifyChannelWatchers(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), func(lang string) (string, string) {
			return i18n.Translate(lang, i18n.MsgChannelNotifyDisabledSubject, params), i18n.Translate(lang, i18n.MsgChannelNotifyDisabledContent, params)
		})
		if common.IsMasterNode && channel != nil {
			scheduledPlugins.disabled(channel, channelError.UsingKey, time.Now())
		}
	}
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		params := map[string]any{"Name": channelName, "Id": channelId}
		NotifyChannelWatchers(formatNotifyType(channelId, common.ChannelStatusEnabled), func(lang string) (string, string) {
			return i18n.Translate(lang, i18n.MsgChannelNotifyEnabledSubject, params), i18n.Translate(lang, i18n.MsgChannelNotifyEnabledContent, params)
		})
	}
}

// EnableChannelIfEligible performs a fresh state check so scheduling recovery
// cannot overwrite an administrator's manual disable made during testing.
func EnableChannelIfEligible(channelId int, channelName string) bool {
	changed, err := model.RestoreChannelForScheduling(channelId)
	if err != nil || !changed {
		return false
	}
	model.InitChannelCache()
	ResetChannelSchedulingState(channelId, "")
	appendSchedulingEvent(ChannelSchedulingEvent{Ts: common.GetTimestamp(), Type: "restored_channel", ChannelId: channelId, ChannelName: channelName, Message: "channel restored automatically after all model tests passed"})
	return true
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	// Channel key and config errors fail for every request, so they disable.
	// Errors new-api raised itself never do: the request may not have reached
	// the upstream, and their message can carry client input that would
	// otherwise match a disable keyword.
	switch RelayErrorSource(err) {
	case "channel":
		return true
	case "local":
		return false
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	return search
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, channel *model.Channel) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil || channel == nil {
		return false
	}
	if channel.Status != common.ChannelStatusAutoDisabled || model.IsChannelSchedulingFault(channel) {
		return false
	}
	return true
}
