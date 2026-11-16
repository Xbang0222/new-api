package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm/clause"
)

const ChannelSchedulingFaultReasonPrefix = "[intelligent-scheduling] "

// ChannelModelState stores durable model-specific scheduling faults. Whole
// channel faults continue to use channels.status so all existing channel
// management and cache invalidation behavior remains authoritative.
type ChannelModelState struct {
	ChannelId      int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false"`
	Model          string `json:"model" gorm:"type:varchar(191);primaryKey;autoIncrement:false"`
	Disabled       bool   `json:"disabled" gorm:"index"`
	ManualDisabled bool   `json:"manual_disabled" gorm:"index"`
	Reason         string `json:"reason" gorm:"type:text"`
	ErrorCode      string `json:"error_code" gorm:"size:128"`
	StatusCode     int    `json:"status_code"`
	DisabledAt     int64  `json:"disabled_at" gorm:"bigint;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint"`
}

func IsChannelSchedulingFault(channel *Channel) bool {
	if channel == nil {
		return false
	}
	reason, _ := channel.GetOtherInfo()["status_reason"].(string)
	return strings.HasPrefix(reason, ChannelSchedulingFaultReasonPrefix)
}

func GetChannelSchedulingFaultReason(channel *Channel) string {
	if channel == nil {
		return ""
	}
	reason, _ := channel.GetOtherInfo()["status_reason"].(string)
	return strings.TrimPrefix(reason, ChannelSchedulingFaultReasonPrefix)
}

func DisableChannelForScheduling(channelId int, reason string) error {
	markedReason := ChannelSchedulingFaultReasonPrefix + reason
	if UpdateChannelStatus(channelId, "", common.ChannelStatusAutoDisabled, markedReason) {
		return nil
	}

	pollingLock := GetChannelPollingLock(channelId)
	pollingLock.Lock()
	defer pollingLock.Unlock()

	channel, err := UpdateChannelLocked(channelId, func(channel *Channel) error {
		if channel.Status != common.ChannelStatusAutoDisabled {
			return fmt.Errorf("channel %d was not disabled", channelId)
		}
		info := channel.GetOtherInfo()
		info["status_reason"] = markedReason
		info["status_time"] = common.GetTimestamp()
		channel.SetOtherInfo(info)
		return nil
	})
	if err != nil {
		InitChannelCache()
		return err
	}
	CacheUpdateChannel(channel)
	return nil
}

func RestoreChannelForScheduling(channelId int) (bool, error) {
	pollingLock := GetChannelPollingLock(channelId)
	pollingLock.Lock()
	defer pollingLock.Unlock()

	channel, err := GetChannelById(channelId, true)
	if err != nil {
		return false, err
	}
	if channel.Status != common.ChannelStatusAutoDisabled || !IsChannelSchedulingFault(channel) {
		return false, nil
	}
	previousOtherInfo := channel.OtherInfo
	channel.Status = common.ChannelStatusEnabled
	info := channel.GetOtherInfo()
	info["status_reason"] = ""
	info["status_time"] = common.GetTimestamp()
	channel.SetOtherInfo(info)
	result := DB.Model(&Channel{}).
		Where("id = ? AND status = ? AND other_info = ?", channelId, common.ChannelStatusAutoDisabled, previousOtherInfo).
		Updates(map[string]interface{}{"status": channel.Status, "other_info": channel.OtherInfo})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	if err := UpdateAbilityStatus(channelId, true); err != nil {
		common.SysError(fmt.Sprintf("failed to restore channel abilities: channel_id=%d, error=%v", channelId, err))
	}
	CacheUpdateChannel(channel)
	BumpSyncTopic(SyncTopicChannels)
	return true, nil
}

func MarkChannelManualDisabledForScheduling(channelId int) (bool, error) {
	pollingLock := GetChannelPollingLock(channelId)
	pollingLock.Lock()
	defer pollingLock.Unlock()

	channel, err := GetChannelById(channelId, true)
	if err != nil {
		return false, err
	}
	if channel.Status != common.ChannelStatusAutoDisabled || !IsChannelSchedulingFault(channel) {
		return false, nil
	}
	previousOtherInfo := channel.OtherInfo
	channel.Status = common.ChannelStatusManuallyDisabled
	info := channel.GetOtherInfo()
	info["status_reason"] = "manual operation"
	info["status_time"] = common.GetTimestamp()
	channel.SetOtherInfo(info)
	result := DB.Model(&Channel{}).
		Where("id = ? AND status = ? AND other_info = ?", channelId, common.ChannelStatusAutoDisabled, previousOtherInfo).
		Updates(map[string]interface{}{"status": channel.Status, "other_info": channel.OtherInfo})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	CacheUpdateChannel(channel)
	BumpSyncTopic(SyncTopicChannels)
	return true, nil
}

func DisableChannelModel(channelId int, modelName string, reason string, errorCode string, statusCode int) error {
	now := time.Now().Unix()
	state := ChannelModelState{
		ChannelId:  channelId,
		Model:      modelName,
		Disabled:   true,
		Reason:     reason,
		ErrorCode:  errorCode,
		StatusCode: statusCode,
		DisabledAt: now,
		UpdatedAt:  now,
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "channel_id"}, {Name: "model"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"disabled":        true,
			"manual_disabled": false,
			"reason":          reason,
			"error_code":      errorCode,
			"status_code":     statusCode,
			"disabled_at":     now,
			"updated_at":      now,
		}),
	}).Create(&state).Error
}

// MarkChannelModelManualDisabled prevents scheduling recovery from probing this
// capability until an administrator explicitly restores it.
func MarkChannelModelManualDisabled(channelId int, modelName string) (bool, error) {
	result := DB.Model(&ChannelModelState{}).
		Where("channel_id = ? AND model = ? AND disabled = ?", channelId, modelName, true).
		Updates(map[string]interface{}{"manual_disabled": true, "updated_at": time.Now().Unix()})
	return result.RowsAffected > 0, result.Error
}

func RestoreChannelModel(channelId int, modelName string) (bool, error) {
	result := DB.Model(&ChannelModelState{}).
		Where("channel_id = ? AND model = ? AND disabled = ?", channelId, modelName, true).
		Updates(map[string]interface{}{
			"disabled":        false,
			"manual_disabled": false,
			"reason":          "",
			"error_code":      "",
			"status_code":     0,
			"updated_at":      time.Now().Unix(),
		})
	return result.RowsAffected > 0, result.Error
}

func ListChannelModelStates(disabledOnly bool) ([]ChannelModelState, error) {
	var states []ChannelModelState
	query := DB.Model(&ChannelModelState{})
	if disabledOnly {
		query = query.Where("disabled = ?", true)
	}
	err := query.Order("updated_at DESC").Find(&states).Error
	return states, err
}

func HasManualDisabledChannelModel(channelId int) (bool, error) {
	var count int64
	err := DB.Model(&ChannelModelState{}).
		Where("channel_id = ? AND disabled = ? AND manual_disabled = ?", channelId, true, true).
		Count(&count).Error
	return count > 0, err
}

func ListAutoDisabledChannels() ([]Channel, error) {
	var channels []Channel
	err := DB.Where("status = ?", common.ChannelStatusAutoDisabled).
		Order("id ASC").Find(&channels).Error
	return channels, err
}
