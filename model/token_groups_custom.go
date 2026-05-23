// custom: token multi-group
//
// 把 Token 的多分组列表存到 GroupsJSON（数据库列 groups），
// API 输入/输出用 Groups（[]string）。AfterFind hook 自动 sync GroupsJSON → Groups。
// Insert/Update 之前必须调 NormalizeGroups 把 Groups → GroupsJSON 并做归一化。
package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// AfterFind 是 GORM 查询返回后自动调用的 hook，把 GroupsJSON 反序列化到 Groups 字段。
// 容错：JSON 非法时仅记日志，把 Groups 置 nil，不返回 error（避免阻断 token 加载）。
func (t *Token) AfterFind(tx *gorm.DB) error {
	if t.GroupsJSON == "" {
		t.Groups = nil
		return nil
	}
	var arr []string
	if err := common.UnmarshalJsonStr(t.GroupsJSON, &arr); err != nil {
		common.SysError(fmt.Sprintf("token %d GroupsJSON malformed: %v", t.Id, err))
		t.Groups = nil
		return nil
	}
	t.Groups = arr
	return nil
}

// NormalizeGroups 把入参 Groups 清洗后写入 GroupsJSON / Group / CrossGroupRetry。
// 清洗规则：trim 空白 + 过滤空字符串 + 保序去重。
// 归一化语义：
//   - Groups == nil（客户端没传 groups 字段，纯老 API）: 完全不动，保持 Group / GroupsJSON 原值。
//   - len(cleaned) == 0（客户端显式传空数组）: Group = "" + GroupsJSON = ""（"用用户默认分组"）。
//   - len(cleaned) == 1: Group = 该项, GroupsJSON = ""（退化为单分组老格式）。
//   - len(cleaned) > 1: Group = 首项, GroupsJSON = JSON 串, CrossGroupRetry 强制 true。
func (t *Token) NormalizeGroups() error {
	// nil 表示"客户端没传 groups 字段"：保留 Group / GroupsJSON 原值，兼容老 API。
	// Controller 在调本函数前应处理好 nil → []string{} / [...] 的桥接。
	if t.Groups == nil {
		return nil
	}
	cleaned := make([]string, 0, len(t.Groups))
	seen := make(map[string]struct{}, len(t.Groups))
	for _, g := range t.Groups {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if _, dup := seen[g]; dup {
			continue
		}
		seen[g] = struct{}{}
		cleaned = append(cleaned, g)
	}
	t.Groups = cleaned

	switch len(cleaned) {
	case 0:
		t.Group = ""
		t.GroupsJSON = ""
	case 1:
		t.Group = cleaned[0]
		t.GroupsJSON = ""
	default:
		t.Group = cleaned[0]
		data, err := common.Marshal(cleaned)
		if err != nil {
			return err
		}
		t.GroupsJSON = string(data)
		t.CrossGroupRetry = true // 多分组天然就是跨分组语义，强制开启
	}
	return nil
}

// EffectiveGroups 返回业务判定要用的分组列表（鉴权/选渠道）。
// 对返回的每一项做 trim，防止历史脏数据（例如直写 DB 时遗留的 " vip "）
// 在 auth.go 走 GetUserUsableGroups 时因为空格 mismatch 被误拒。
//   - 已有 Groups 列表（多分组场景）→ 直接返回（trim 过）
//   - 否则若 Group 非空 → 单元素列表 [Group]（trim 过）
//   - 否则 nil（使用用户默认分组）
func (t *Token) EffectiveGroups() []string {
	if len(t.Groups) > 0 {
		out := make([]string, 0, len(t.Groups))
		for _, g := range t.Groups {
			g = strings.TrimSpace(g)
			if g != "" {
				out = append(out, g)
			}
		}
		return out
	}
	if trimmed := strings.TrimSpace(t.Group); trimmed != "" {
		return []string{trimmed}
	}
	return nil
}

// BridgeNilGroupsFromLegacy promotes the legacy single-Group field into Groups when
// the caller did not supply a groups list (Groups == nil). This ensures
// NormalizeGroups always runs the explicit-list path regardless of whether the
// client used the old API (group only) or the new API (groups array).
//   - Empty Group field → empty Groups slice (explicit "clear" intent)
//   - Non-empty Group field → single-element Groups list (trimmed)
//
// Idempotent: if Groups is already non-nil (even an empty slice), no change.
func (t *Token) BridgeNilGroupsFromLegacy() {
	if t.Groups != nil {
		return
	}
	trimmed := strings.TrimSpace(t.Group)
	if trimmed != "" {
		t.Groups = []string{trimmed}
	} else {
		t.Groups = []string{}
	}
}
