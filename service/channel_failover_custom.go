package service

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/logger"

	"github.com/gin-gonic/gin"
)

// buildExcludedChannelSet (custom: retry failover) collects the channel IDs already
// tried during the current request from the gin context "use_channel" slice
// (appended by addUsedChannel in controller/relay.go after every attempt).
//
// The channel selector excludes these IDs so a failed channel is never retried:
// it picks the highest priority tier that still has a non-excluded channel,
// which naturally yields "same-tier failover -> drop one tier when that tier is
// exhausted -> error only when every tier is exhausted".
//
// Lifetime: gin.Context is request-scoped, so the set is rebuilt fresh on every
// CacheGetRandomSatisfiedChannel call and never leaks across requests. Excluding
// by channel_id is correct across groups too — Channel.Id is a globally unique
// primary key, so the same physical upstream that just failed must not be picked
// again regardless of which group it appears in.
func buildExcludedChannelSet(c *gin.Context) map[int]bool {
	if c == nil {
		return nil
	}
	used := c.GetStringSlice("use_channel")
	if len(used) == 0 {
		return nil
	}
	excluded := make(map[int]bool, len(used))
	for _, idStr := range used {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			// Do not fail silently: a malformed entry means we cannot exclude that
			// channel, which could let a failed channel be retried. Log and skip.
			logger.LogWarn(c, fmt.Sprintf("buildExcludedChannelSet: invalid channel id in use_channel: %q", idStr))
			continue
		}
		excluded[id] = true
	}
	return excluded
}
