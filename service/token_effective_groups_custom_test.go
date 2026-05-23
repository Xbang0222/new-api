// custom: token multi-group
package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// ctxWithTokenGroupList 构造一个写入 ContextKeyTokenGroupList 的 gin.Context。
func ctxWithTokenGroupList(list []string) *gin.Context {
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyTokenGroupList, list)
	return c
}

// withSeededAutoAndUsableGroups 把 auto / usable 全局设置切到指定值，并通过
// t.Cleanup 在测试结束时恢复原值。返回值供调用方在断言后调试用。
func withSeededAutoAndUsableGroups(t *testing.T, autoJSON, usableJSON string) {
	t.Helper()

	origAutoJSON := setting.AutoGroups2JsonString()
	origUsableJSON := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		_ = setting.UpdateAutoGroupsByJsonString(origAutoJSON)
		_ = setting.UpdateUserUsableGroupsByJSONString(origUsableJSON)
	})

	if err := setting.UpdateAutoGroupsByJsonString(autoJSON); err != nil {
		t.Fatalf("seed auto groups failed: %v", err)
	}
	if err := setting.UpdateUserUsableGroupsByJSONString(usableJSON); err != nil {
		t.Fatalf("seed usable groups failed: %v", err)
	}
}

func TestExpandAutoEntries_NoAutoNoDuplicate(t *testing.T) {
	got := expandAutoEntries([]string{"vip", "default"}, "")
	assert.Equal(t, []string{"vip", "default"}, got)
}

func TestExpandAutoEntries_DedupeOnly(t *testing.T) {
	got := expandAutoEntries([]string{"vip", "vip", "default", "vip"}, "")
	assert.Equal(t, []string{"vip", "default"}, got)
}

func TestExpandAutoEntries_SingleEntryNoExpansion(t *testing.T) {
	// 单个非 auto 元素 / list 中无 "auto" 占位符时，expandAutoEntries
	// 仅做去重并保留顺序，不应触发 GetUserAutoGroup 展开。
	got := expandAutoEntries([]string{"vip"}, "")
	assert.Equal(t, []string{"vip"}, got)
}

func TestExpandAutoEntries_AutoExpansion(t *testing.T) {
	// 种入 auto=[x,y] + usable={x,y}，让 GetUserAutoGroup("") 返回 [x,y]
	withSeededAutoAndUsableGroups(t, `["x","y"]`, `{"x":"X group","y":"Y group"}`)

	// ["a", "auto", "b"] + auto=[x,y] → ["a","x","y","b"]
	got := expandAutoEntries([]string{"a", "auto", "b"}, "")
	assert.Equal(t, []string{"a", "x", "y", "b"}, got)

	// ["auto", "x"] + auto=[x,y] → ["x","y"]
	// （auto 先展开为 [x,y]，随后 list 里的 "x" 被去重跳过）
	got2 := expandAutoEntries([]string{"auto", "x"}, "")
	assert.Equal(t, []string{"x", "y"}, got2)
}

func TestGetTokenEffectiveAutoGroups_EmptyCtxFallback(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	got := GetTokenEffectiveAutoGroups(c, "default") // 没写 ctx key → fallback
	// fallback 取 GetUserAutoGroup(userGroup)，在测试环境通常为空 slice
	// 这里我们只断言 "没有 panic 且返回类型正确"
	assert.NotNil(t, got)
}

func TestGetTokenEffectiveAutoGroups_NilCtxDoesNotPanic(t *testing.T) {
	// gin.Context 为 nil 时不应 panic，应当回退到 GetUserAutoGroup(userGroup)。
	assert.NotPanics(t, func() {
		_ = GetTokenEffectiveAutoGroups(nil, "default")
	})
	got := GetTokenEffectiveAutoGroups(nil, "default")
	assert.NotNil(t, got)
}

func TestGetTokenEffectiveAutoGroups_UsesTokenList(t *testing.T) {
	c := ctxWithTokenGroupList([]string{"vip", "default"})
	got := GetTokenEffectiveAutoGroups(c, "default")
	assert.Equal(t, []string{"vip", "default"}, got)
}

func TestGetTokenEffectiveAutoGroups_DedupePreservesOrder(t *testing.T) {
	c := ctxWithTokenGroupList([]string{"a", "b", "a", "c", "b"})
	got := GetTokenEffectiveAutoGroups(c, "default")
	assert.Equal(t, []string{"a", "b", "c"}, got)
}
