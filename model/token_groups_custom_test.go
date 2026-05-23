// custom: token multi-group
package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeGroups_Empty(t *testing.T) {
	tok := &Token{Group: "vip", Groups: []string{}, CrossGroupRetry: true}
	err := tok.NormalizeGroups()
	assert.NoError(t, err)
	assert.Equal(t, "", tok.GroupsJSON)
	assert.Equal(t, "", tok.Group, "case 0 应清空 Group（语义：用用户默认分组）")
	assert.Equal(t, []string{}, tok.Groups)
}

func TestNormalizeGroups_SingleItem(t *testing.T) {
	tok := &Token{Group: "old", Groups: []string{"vip"}, CrossGroupRetry: true}
	err := tok.NormalizeGroups()
	assert.NoError(t, err)
	assert.Equal(t, "vip", tok.Group)
	assert.Equal(t, "", tok.GroupsJSON) // 退化为单分组
}

func TestNormalizeGroups_MultiItems(t *testing.T) {
	tok := &Token{Groups: []string{"vip", "default"}, CrossGroupRetry: false}
	err := tok.NormalizeGroups()
	assert.NoError(t, err)
	assert.Equal(t, "vip", tok.Group)
	assert.Equal(t, `["vip","default"]`, tok.GroupsJSON)
	assert.True(t, tok.CrossGroupRetry, "多分组应强制启用 CrossGroupRetry")
}

func TestNormalizeGroups_TrimAndDedupe(t *testing.T) {
	tok := &Token{Groups: []string{"  vip ", "", "default", "vip", "  "}}
	err := tok.NormalizeGroups()
	assert.NoError(t, err)
	assert.Equal(t, []string{"vip", "default"}, tok.Groups)
	assert.Equal(t, `["vip","default"]`, tok.GroupsJSON)
}

func TestNormalizeGroups_OnlyWhitespace(t *testing.T) {
	tok := &Token{Group: "kept", Groups: []string{"   ", "", "\t"}}
	err := tok.NormalizeGroups()
	assert.NoError(t, err)
	assert.Equal(t, []string{}, tok.Groups)
	assert.Equal(t, "", tok.GroupsJSON)
	assert.Equal(t, "", tok.Group, "纯空白等价于显式清空，Group 也清空")
}

func TestNormalizeGroups_NilGroupsPreservesLegacyGroup(t *testing.T) {
	// 客户端用老 API：完全没传 groups 字段（=== nil），仅传 group。
	// Controller 负责在 nil 时把 Group → Groups 桥接，但 NormalizeGroups 本身
	// 在收到 nil 时不应清空 Group（兼容 controller 没桥接的极端 case）。
	tok := &Token{Group: "vip", Groups: nil}
	err := tok.NormalizeGroups()
	assert.NoError(t, err)
	assert.Equal(t, "vip", tok.Group, "Groups=nil 时不清空 Group（兼容 legacy）")
	assert.Equal(t, "", tok.GroupsJSON)
}

func TestEffectiveGroups_FromGroups(t *testing.T) {
	tok := &Token{Group: "first", Groups: []string{"vip", "default"}}
	assert.Equal(t, []string{"vip", "default"}, tok.EffectiveGroups())
}

func TestEffectiveGroups_FromGroupField(t *testing.T) {
	tok := &Token{Group: "vip", Groups: nil}
	assert.Equal(t, []string{"vip"}, tok.EffectiveGroups())
}

func TestEffectiveGroups_Empty(t *testing.T) {
	tok := &Token{Group: "", Groups: nil}
	assert.Nil(t, tok.EffectiveGroups())
}

func TestAfterFind_EmptyGroupsJSON(t *testing.T) {
	tok := &Token{GroupsJSON: "", Groups: []string{"stale"}}
	err := tok.AfterFind(nil)
	assert.NoError(t, err)
	assert.Nil(t, tok.Groups)
}

func TestAfterFind_ValidGroupsJSON(t *testing.T) {
	tok := &Token{GroupsJSON: `["vip","default"]`}
	err := tok.AfterFind(nil)
	assert.NoError(t, err)
	assert.Equal(t, []string{"vip", "default"}, tok.Groups)
}

func TestAfterFind_MalformedJSONDoesNotError(t *testing.T) {
	tok := &Token{Id: 42, GroupsJSON: "not json"}
	err := tok.AfterFind(nil)
	// 不返回 error 避免阻断 token 加载，但 Groups 应为 nil
	assert.NoError(t, err)
	assert.Nil(t, tok.Groups)
}

func TestBridgeNilGroupsFromLegacy_NilWithNonEmptyGroup(t *testing.T) {
	tok := &Token{Group: "vip", Groups: nil}
	tok.BridgeNilGroupsFromLegacy()
	assert.Equal(t, []string{"vip"}, tok.Groups)
	assert.Equal(t, "vip", tok.Group, "Group 字段不应被修改")
}

func TestBridgeNilGroupsFromLegacy_NilWithEmptyGroup(t *testing.T) {
	tok := &Token{Group: "", Groups: nil}
	tok.BridgeNilGroupsFromLegacy()
	assert.Equal(t, []string{}, tok.Groups)
}

func TestBridgeNilGroupsFromLegacy_NonNilGroupsUnchanged(t *testing.T) {
	tok := &Token{Group: "ignored", Groups: []string{"vip", "default"}}
	tok.BridgeNilGroupsFromLegacy()
	assert.Equal(t, []string{"vip", "default"}, tok.Groups, "已有 Groups 时不动")
}

func TestBridgeNilGroupsFromLegacy_NonNilEmptyGroupsUnchanged(t *testing.T) {
	tok := &Token{Group: "vip", Groups: []string{}}
	tok.BridgeNilGroupsFromLegacy()
	assert.Equal(t, []string{}, tok.Groups, "非 nil 空 slice 表示客户端显式清空，不应被 bridge 覆盖")
}
