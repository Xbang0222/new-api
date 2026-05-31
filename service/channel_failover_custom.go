package service

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// buildExcludedChannelSet (custom: retry failover) computes the set of channel IDs that
// must NOT be selected again for the current request, derived from the gin-context
// "use_channel" slice (appended by addUsedChannel in controller/relay.go after every
// attempt).
//
// A channel is excluded once it can no longer usefully serve the request (see
// channelExhausted):
//   - single-key channel: one failure excludes it (never retry a dead channel);
//   - multi-key channel: excluded once it has been tried as many times as it has keys
//     (a STATIC upper bound — see channelExhausted for why the live enabled-key count is
//     deliberately NOT used) OR once every key is disabled. Each retry rotates to a fresh
//     key (the spent key is retired by DisableChannel on auth/quota errors); failover moves
//     to another channel only after every key has had a turn.
//
// The selector then picks the highest priority tier that still has a non-excluded channel,
// giving "same-tier failover -> drop a tier when exhausted -> error only when every tier is
// exhausted".
//
// Lifetime: gin.Context is request-scoped, so the set is rebuilt fresh on every
// CacheGetRandomSatisfiedChannel call and never leaks across requests. Excluding by
// channel_id is correct across groups too — Channel.Id is a globally unique primary key,
// so the same physical upstream that is used up must not be picked again regardless of
// which group it appears in.
func buildExcludedChannelSet(c *gin.Context) map[int]bool {
	if c == nil {
		return nil
	}
	used := c.GetStringSlice("use_channel")
	if len(used) == 0 {
		return nil
	}
	attempts := make(map[int]int, len(used))
	for _, idStr := range used {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			// Do not fail silently: a malformed entry means we cannot account for that
			// channel, which could let a used-up channel be retried. Log and skip.
			logger.LogWarn(c, fmt.Sprintf("buildExcludedChannelSet: invalid channel id in use_channel: %q", idStr))
			continue
		}
		attempts[id]++
	}
	if len(attempts) == 0 {
		return nil
	}
	excluded := make(map[int]bool, len(attempts))
	for id, tried := range attempts {
		if channelExhausted(id, tried) {
			excluded[id] = true
		}
	}
	if len(excluded) == 0 {
		// Some channel still has an untried key — return nil so downstream NOT IN / map
		// lookups treat the request as "nothing excluded yet" (consistent with first attempt).
		return nil
	}
	return excluded
}

// channelExhausted (custom: retry failover) reports whether a channel has used up its
// per-request attempt budget and must be excluded from further failover.
//
//   - non-multi-key / single-key channel: exhausted after one failure (never retry a dead
//     channel).
//   - multi-key channel: exhausted once it has been tried as many times as it has keys
//     (`tried >= total`) OR once every key is disabled (`CountEnabledKeys() == 0`, nothing
//     left to try).
//
// Why the STATIC total-key count and not the live enabled-key count: DisableChannel runs
// asynchronously (gopool.Go in controller/relay.go) and retires a key on auth/quota errors,
// so the enabled count SHRINKS during a request. Comparing a monotonically growing attempt
// count against a shrinking budget makes them meet one key too early — e.g. a 2-key channel
// whose first key gets disabled would compute budget 1 against attempts 1 and be excluded
// before its second key is ever tried. The total key count never shrinks, so it is a
// race-free upper bound that still lets every key have a turn (GetNextEnabledKey skips the
// disabled ones), while the `== 0` short-circuit lets failover move on promptly once the
// channel is truly dead. If the channel cannot be looked up (cache miss / deleted), treat it
// as single-key — the conservative choice that excludes it after one failure.
func channelExhausted(channelId, tried int) bool {
	channel, err := model.CacheGetChannel(channelId)
	if err != nil || channel == nil || !channel.ChannelInfo.IsMultiKey {
		return tried >= 1
	}
	total := len(channel.GetKeys())
	if total <= 1 {
		return tried >= 1
	}
	if tried >= total {
		return true
	}
	return channel.CountEnabledKeys() == 0
}
