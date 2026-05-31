package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type RetryParam struct {
	Ctx          *gin.Context
	TokenGroup   string
	ModelName    string
	Retry        *int
	resetNextTry bool
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

// SetRetry / ResetRetryNextTry are upstream RetryParam helpers. custom: retry failover
// removed their only callers (the per-group budget reset in the multi-group auto path),
// so they are currently unused here, but are kept intact — and NOT deleted — to minimize
// the upstream merge diff. Retry is now a pure global budget counter driven solely by
// IncreaseRetry; the excluded set drives tier/group progression instead of resetting retry.
func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

// CacheGetRandomSatisfiedChannel selects a channel for the request under the
// excluded-set failover model (custom: retry failover).
// 在「排除集合」故障转移模型下为请求选择渠道。
//
// Tier selection is driven by the excluded set — the channels already tried this
// request, read once from gin.Context "use_channel". The model-layer selector picks
// the highest priority tier that still has a non-excluded channel and weighted-randoms
// within it; retry stays a pure global budget counter in the relay loop.
// 档位由排除集合驱动（本次请求已尝试的渠道，从 use_channel 读一次）：model 层选「还有
// 未排除渠道的最高优先级档」并在档内加权随机；retry 仅作 relay 循环的全局预算计数。
//
// For an "auto" tokenGroup it walks the effective auto groups from the remembered
// ContextKeyAutoGroupIndex:
// 对于 "auto" tokenGroup，从记录的 ContextKeyAutoGroupIndex 起遍历有效分组：
//
//   - A group returns nil when it has no non-excluded channel for the model.
//     某分组对该模型没有未排除渠道时返回 nil。
//
//   - On nil, advance to the next group only when cross-group retry is on (multi-group
//     token) or still in the initial walk (excluded empty — the group simply lacks the
//     model). Plain "auto" stays within one group and stops when it is exhausted.
//     返回 nil 时，仅在开启跨分组重试（多分组 token）或仍处初始走查（excluded 为空，即
//     该分组本就没有此模型）时切下一分组；plain "auto" 固定单分组内，耗尽即停止。
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	var channel *model.Channel
	var err error
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)

	// custom: retry failover — build the already-tried channel set once from
	// use_channel and pass it (unchanged) to every group. The selector picks the
	// highest tier that still has a non-excluded channel, so a failed channel is
	// never retried. Channel.Id is globally unique, so preserving the set across
	// groups is correct (same physical upstream stays excluded everywhere).
	excluded := buildExcludedChannelSet(param.Ctx)

	if param.TokenGroup == "auto" {
		// custom: token multi-group — 多分组 token 在 ctx 里自带分组列表，
		// 不依赖管理员配置的全局 auto 列表。先取 effective auto groups，
		// 为空再判 "auto 未启用"，避免误伤多分组 token。
		autoGroups := GetTokenEffectiveAutoGroups(param.Ctx, userGroup)
		if len(autoGroups) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}

		// custom: retry failover
		// Original: priorityRetry = Retry - startRetryIndex drove the tier inside each
		//   group, and SetRetry(0)/ResetRetryNextTry reset the budget per group, so total
		//   attempts became groupCount*(RetryTimes+1); the tier was indexed by retry count.
		// Changed: the excluded set drives the tier inside each group; retry stays a single
		//   global budget counter in the relay loop (never reset here). We walk groups from
		//   the remembered AutoGroupIndex; each group's GetRandomSatisfiedChannel returns a
		//   non-excluded channel from its highest available tier, or nil when that group has
		//   no usable channel. We advance to the next group on nil only when cross-group retry
		//   is on (multi-group token) OR we are still in the initial walk (excluded empty),
		//   where nil means the group simply lacks the model — preserving plain "auto" staying
		//   within a single group and stopping when that group's channels are exhausted.
		// Why: user chose "global total budget RetryTimes+1"; tier/group progression is now
		//   state-driven by the excluded set, not by mutating retry.
		// Revert: restore priorityRetry + SetRetry(0) + ResetRetryNextTry + AutoGroupRetryIndex.
		crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)
		startGroupIndex := 0
		if lastGroupIndex, exists := common.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
			if idx, ok := lastGroupIndex.(int); ok {
				startGroupIndex = idx
			}
		}

		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			channel, _ = model.GetRandomSatisfiedChannel(autoGroup, param.ModelName, excluded)
			if channel == nil {
				logger.LogDebug(param.Ctx, "No usable channel in group %s for model %s (excluded=%d)", autoGroup, param.ModelName, len(excluded))
				if !crossGroupRetry && len(excluded) > 0 {
					// plain auto: the chosen group is exhausted (all its channels tried).
					// Do not fall across groups — stop and report no-available.
					break
				}
				// cross-group token, or initial walk skipping a group that lacks the model.
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				continue
			}
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
			selectGroup = autoGroup
			logger.LogDebug(param.Ctx, "Auto selected group: %s", autoGroup)
			break
		}
	} else {
		channel, err = model.GetRandomSatisfiedChannel(param.TokenGroup, param.ModelName, excluded)
		if err != nil {
			return nil, param.TokenGroup, err
		}
	}
	return channel, selectGroup, nil
}
