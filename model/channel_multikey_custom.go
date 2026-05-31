package model

import "github.com/QuantumNous/new-api/common"

// CountEnabledKeys (custom: retry failover) returns how many of a channel's keys are
// still usable, mirroring GetNextEnabledKey's status semantics (a key counts as enabled
// unless MultiKeyStatusList marks it otherwise; a missing entry defaults to enabled).
// A non-multi-key channel returns 1 — its single credential.
//
// The failover layer (service.buildExcludedChannelSet) uses this as a per-request
// attempt budget: a multi-key channel stays eligible for re-selection until each of its
// live keys has had a turn, so one failed key does not retire the whole channel — the
// other pooled credentials are effectively independent upstreams. Once every live key is
// used up the channel is excluded and failover moves on, which still honours "never
// retry a dead channel".
//
// This is a pure read with NO side effects (unlike GetNextEnabledKey, which advances the
// polling index). The MultiKeyStatusList read is guarded by the same per-channel polling
// lock that handlerMultiKeyUpdate takes when disabling a key, so a concurrent key-disable
// (DisableChannel runs async) cannot cause a "concurrent map read and map write" panic.
func (channel *Channel) CountEnabledKeys() int {
	if !channel.ChannelInfo.IsMultiKey {
		return 1
	}
	keys := channel.GetKeys()
	if len(keys) == 0 {
		return 0
	}
	lock := GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()
	statusList := channel.ChannelInfo.MultiKeyStatusList
	if statusList == nil {
		return len(keys)
	}
	enabled := 0
	for i := range keys {
		if status, ok := statusList[i]; !ok || status == common.ChannelStatusEnabled {
			enabled++
		}
	}
	return enabled
}
