// custom: token multi-group
//
// 多分组令牌在 distributor 走 auto 分支时，使用 token 自定义列表替代用户全局 auto 列表。
// 入口函数 GetTokenEffectiveAutoGroups 从 ctx 读 ContextKeyTokenGroupList（鉴权阶段写入），
// 没有则 fallback 到 GetUserAutoGroup(userGroup)（兼容 legacy auto token）。
package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

// GetTokenEffectiveAutoGroups 在 auto 分支下返回 token 应当遍历的分组列表。
//   - 多分组 token: 使用 ctx 里的 ContextKeyTokenGroupList，把 "auto" 占位项展开为用户全局 auto 列表，去重保序。
//   - legacy auto token: 回退到 GetUserAutoGroup(userGroup)。
//   - ctx 为 nil 时直接走 fallback，避免 gin 内部 c.mu 解引用 panic。
func GetTokenEffectiveAutoGroups(ctx *gin.Context, userGroup string) []string {
	if ctx == nil {
		return GetUserAutoGroup(userGroup)
	}
	if v, ok := common.GetContextKey(ctx, constant.ContextKeyTokenGroupList); ok {
		if list, ok2 := v.([]string); ok2 && len(list) > 0 {
			return expandAutoEntries(list, userGroup)
		}
	}
	return GetUserAutoGroup(userGroup)
}

// expandAutoEntries 把 list 中的 "auto" 占位项替换为 GetUserAutoGroup(userGroup) 展开，
// 然后整体去重保序。
//   - list 中没有 "auto": 仅去重
//   - list 中含 "auto": "auto" 位置展开成用户全局 auto 分组（依次）
//   - GetUserAutoGroup 为空时 "auto" 项被静默丢弃
func expandAutoEntries(list []string, userGroup string) []string {
	result := make([]string, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	appendIfUnseen := func(g string) {
		if _, ok := seen[g]; ok {
			return
		}
		seen[g] = struct{}{}
		result = append(result, g)
	}
	for _, item := range list {
		if item == "auto" {
			for _, g := range GetUserAutoGroup(userGroup) {
				appendIfUnseen(g)
			}
			continue
		}
		appendIfUnseen(item)
	}
	return result
}
