# 令牌多分组（Token Multi-Group）设计文档

- **日期**: 2026-05-23
- **分支**: ruoli（fork）
- **目标版本**: v0.13.2-ruoli-0.12
- **状态**: Draft → 待 PR

## 背景

当前一个 API 令牌（Token）只能绑定一个分组（`Token.Group`），意味着该令牌只能使用该分组下的渠道，并按该分组的倍率计费。这对于希望同一个令牌跨多个分组（例如"先用 vip 渠道，没有再 fallback 到 default 渠道"）的用户来说很不方便。

项目内已有一个名为 **auto** 的特殊分组模式：当 `Token.Group = "auto"` 且 `Token.CrossGroupRetry = true` 时，系统会按"用户全局可用分组列表"挨个尝试找渠道，命中哪个分组就按哪个分组的倍率计费。这套机制完整覆盖了多分组令牌所需要的全部能力，唯一的差别是 **auto 用的是用户的全局列表，而多分组令牌需要使用令牌自定义的列表**。

## 目标

允许令牌绑定**一个有序的分组列表**：
1. 请求到来时，按列表顺序找出第一个有可用渠道的分组。
2. 按实际命中的分组计费（每个分组各自的倍率）。
3. 跨分组重试逻辑沿用现有 auto 模式（按 `CrossGroupRetry` + 各分组优先级）。
4. 列表中允许出现 `"auto"` 作为占位项，被展开为用户全局 auto 列表。

## 非目标

- 不支持"客户端 header 显式指定分组"。
- 不支持"自动选最便宜的分组"。
- 不修改后台对全局 auto 列表的配置入口。
- 不删除或重命名 `Token.Group` 字段（保留作向后兼容）。
- 不重写 `quota.go` / `channel_select.go` 的现有计费/选渠道路径。

## 关键洞察

**多分组令牌 ≡ 自定义范围的 auto 模式。**

`service/channel_select.go::CacheGetRandomSatisfiedChannel`、`service/quota.go::PreConsumeQuota`、`middleware/distributor.go` 等热路径上的 auto 处理（按列表选渠道、命中后写入 `ContextKeyAutoGroup`、`quota.go` 按 `ContextKeyAutoGroup` 替换 `relayInfo.UsingGroup` 后取对应倍率）已经完整实现。**我们只需要把"用户全局 auto 列表"这一个数据源在特定条件下替换为"令牌自定义列表"**，其余路径完全复用。

## 设计

### 数据模型

`model/token.go`:

```go
type Token struct {
    // ... 其它字段不变 ...
    Group           string   `json:"group" gorm:"default:''"`                // legacy：单分组 / 多分组首项备份
    GroupsJSON      string   `json:"-" gorm:"column:groups;type:text"`        // 数据库列 `groups` 存 JSON 字符串
    Groups          []string `json:"groups,omitempty" gorm:"-"`               // API 字段（数组），GORM 忽略
    CrossGroupRetry bool     `json:"cross_group_retry"`                       // 沿用语义
}
```

**字段语义**:
- `GroupsJSON`：数据库实际存储列。空串 = 单分组模式；非空 = JSON 数组字符串如 `["vip","default"]`。
- `Groups`：仅用于 JSON IO（请求和响应都用 `groups: [...]`），GORM `gorm:"-"` 完全忽略。
- `Group`：保留作 legacy 字段，多分组时存首项（提供老的 list 渲染/admin 工具的可读 fallback）。

**Hook 与 helper**（建议放在新文件 `model/token_groups_custom.go` 以最小化 upstream diff）：

```go
// AfterFind 在 GORM 查询返回后把 GroupsJSON 反序列化到 Groups 字段。
func (t *Token) AfterFind(tx *gorm.DB) error {
    if t.GroupsJSON == "" {
        t.Groups = nil
        return nil
    }
    var arr []string
    if err := common.UnmarshalJsonStr(t.GroupsJSON, &arr); err != nil {
        // 防数据库被外部污染：降级到 [Group]，仅记日志
        common.SysError(fmt.Sprintf("token %d GroupsJSON malformed: %v", t.Id, err))
        t.Groups = nil
        return nil
    }
    t.Groups = arr
    return nil
}

// NormalizeGroups 在 Insert/Update 前调用，把 Groups([]string) 归一化到 Group+GroupsJSON。
// 同时处理：trim 空白 / 过滤空字符串 / 多分组强制 CrossGroupRetry=true。
//   - Groups=[]/nil   → Group 不变（前端可单独传 group）；GroupsJSON=""
//   - Groups=[single] → Group=single; GroupsJSON=""（退化为老格式）
//   - Groups=[multi…] → Group=first; GroupsJSON=JSON; CrossGroupRetry=true（强制）
func (t *Token) NormalizeGroups() error {
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
        t.CrossGroupRetry = true // 强制：详见 §跨分组重试
    }
    return nil
}

// EffectiveGroups 返回用于业务判定的分组列表（鉴权/选渠道）：
//   - GroupsJSON 非空 → 解析后的列表
//   - 否则若 Group != "" → [Group]
//   - 否则 nil（使用用户默认分组）
func (t *Token) EffectiveGroups() []string {
    if len(t.Groups) > 0 {
        return t.Groups
    }
    if t.Group != "" {
        return []string{t.Group}
    }
    return nil
}
```

### 鉴权与 Effective Group

`middleware/auth.go::TokenAuth` 现有的 line 382-399 段落，从两分支扩展为三分支：

```go
userGroup := userCache.Group
tokenGroups := token.EffectiveGroups()

switch {
case len(tokenGroups) > 1:
    // —— 多分组模式 ——
    for _, g := range tokenGroups {
        // 复用既有校验：用户可用 + 分组未弃用（auto 例外）
        if _, ok := service.GetUserUsableGroups(userGroup)[g]; !ok {
            abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("无权访问 %s 分组", g))
            return
        }
        if !ratio_setting.ContainsGroupRatio(g) && g != "auto" {
            abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("分组 %s 已被弃用", g))
            return
        }
    }
    common.SetContextKey(c, constant.ContextKeyTokenGroupList, tokenGroups)
    common.SetContextKey(c, constant.ContextKeyUsingGroup, "auto") // 复用 auto 分支

case len(tokenGroups) == 1:
    // —— 老的单分组逻辑，完全保留（直接复用原 line 384-398 校验流程） ——
    tokenGroup := tokenGroups[0]
    if _, ok := service.GetUserUsableGroups(userGroup)[tokenGroup]; !ok { /* abort */ }
    if !ratio_setting.ContainsGroupRatio(tokenGroup) && tokenGroup != "auto" { /* abort */ }
    common.SetContextKey(c, constant.ContextKeyUsingGroup, tokenGroup)

default:
    common.SetContextKey(c, constant.ContextKeyUsingGroup, userGroup)
}
```

新增 ctx key：

```go
// constant/context_key.go
ContextKeyTokenGroupList ContextKey = "token_group_list" // []string，仅多分组 token 写入
```

`ContextKeyTokenGroup` 沿用现状写入 `token.Group`（首项），保持下游兼容。

### 渠道选择（核心改动点）

在 `service/group.go` 新增：

```go
// GetTokenEffectiveAutoGroups 返回 token 在 auto 分支下应该遍历的分组列表。
// 当 ctx 携带 ContextKeyTokenGroupList（多分组 token）时，返回 token 自定义列表
// （其中 "auto" 项会被展开为用户全局 auto 列表，结果去重保序）；
// 否则回退到 GetUserAutoGroup(userGroup)（兼容 legacy auto token）。
func GetTokenEffectiveAutoGroups(ctx *gin.Context, userGroup string) []string {
    if v, ok := common.GetContextKey(ctx, constant.ContextKeyTokenGroupList); ok {
        if list, ok2 := v.([]string); ok2 && len(list) > 0 {
            return expandAutoEntries(list, userGroup)
        }
    }
    return GetUserAutoGroup(userGroup)
}

// expandAutoEntries 把 list 中的 "auto" 占位项替换为 GetUserAutoGroup(userGroup)，
// 去重并保持首次出现顺序。
//
// 算法：
//   result := []string{}
//   seen   := map[string]struct{}{}
//   for _, item := range list {
//       if item == "auto" {
//           for _, g := range GetUserAutoGroup(userGroup) {
//               appendIfUnseen(g)
//           }
//       } else {
//           appendIfUnseen(item)
//       }
//   }
//   return result
//
// 边界：
//   - list 中没有 "auto" → 仅去重，原样返回。
//   - list 全为 "auto" → 等价于 GetUserAutoGroup(userGroup)。
//   - GetUserAutoGroup 为空且 list 全为 "auto" → 返回 []，channel_select 自然 fallback 到 "auto groups is not enabled" 错误（与现有 legacy auto 行为一致）。
func expandAutoEntries(list []string, userGroup string) []string
```

**调用方替换（仅 2 处，逐行 1 行修改）**:

1. `service/channel_select.go:93`
   ```diff
   - autoGroups := GetUserAutoGroup(userGroup)
   + autoGroups := GetTokenEffectiveAutoGroups(param.Ctx, userGroup)
   ```

2. `middleware/distributor.go:117`
   ```diff
   - autoGroups := service.GetUserAutoGroup(userGroup)
   + autoGroups := service.GetTokenEffectiveAutoGroups(c, userGroup)
   ```

### 计费

**零代码改动**。`service/quota.go:112-117` 现有逻辑：

```go
autoGroup, exists := common.GetContextKey(ctx, constant.ContextKeyAutoGroup)
if exists {
    groupRatio = ratio_setting.GetGroupRatio(autoGroup.(string))
    relayInfo.UsingGroup = autoGroup.(string)
}
```

`ContextKeyAutoGroup` 已由 `channel_select.go` / `distributor.go` 在选定渠道时写入实际命中的分组名，多分组令牌完全继承这套行为。

### 跨分组重试（CrossGroupRetry）

字段含义不变。**多分组 token 在 `NormalizeGroups()` 里强制 `CrossGroupRetry = true`** —— 只要清洗后列表长度 > 1，无论前端/API 传什么值都覆盖为 true。

理由：列表显式写多个就是用户明确表达了"跨分组"意图，若不强制 true 则会出现"第一次请求命中第一个分组、重试时困在当前分组内"这种非预期行为。

前端 UX：

| 分组列表 | 开关可见性 | 后端最终值 |
|---------|-----------|-----------|
| `[]` | 隐藏 | - |
| `["vip"]` 等单具体分组 | 隐藏 | 沿用前端提交（实际无作用） |
| `["auto"]`（老 auto 等价） | 显示 | 用户配置 |
| 列表含 `"auto"` 或 长度 > 1 | 隐藏 | **后端强制 true** |

### 控制器

`controller/token.go` 的 `AddToken` / `UpdateToken` 现状是直接 `c.ShouldBindJSON(&token model.Token)` 并把字段抄到 `cleanToken`。改动：

```go
// 抄字段时把 Groups 也带上（已被 ShouldBindJSON 填充为 []string）
cleanToken.Groups = token.Groups

// 在 Insert/Update 之前调用归一化，自动处理：
//   - trim 空白 + 过滤空串 + 去重
//   - 写入 GroupsJSON / Group
//   - len > 1 时强制 CrossGroupRetry = true
if err := cleanToken.NormalizeGroups(); err != nil {
    common.ApiError(c, err)
    return
}
```

`token.Update()` 的 `Select` 子句新增 `"groups"`（数据库列名）。

注意：因为 `Groups` 字段是 `gorm:"-"`，所以 `Select("groups", ...)` 这里的 "groups" 指的是 `GroupsJSON` 的 `column:groups`。GORM 会按 column name 匹配，所以写 `"groups"` 正确。

### DTO / 请求结构

**无需新增独立 DTO**。`controller/token.go` 现状是直接 `c.ShouldBindJSON(&token model.Token)`，扩展 `model.Token` 上的 `Groups []string` 字段后，前端 / API 调用方直接传：

```jsonc
{
    "name": "my token",
    "group": "vip",            // legacy 字段，可选（多分组时被 Groups 覆盖）
    "groups": ["vip", "default"],  // 新字段
    "cross_group_retry": true
}
```

响应同样形式（`Groups` 字段自动通过 `AfterFind` hook 填充，标准 JSON marshal 即可输出 `"groups": [...]`）。

### 前端

#### EditTokenModal.jsx

替换原单 `Form.Select field='group'` 为可增减的列表：

```
令牌分组 ⓘ
┌──────────────────────────────────┐
│ vip                          ▾ │ ✕
└──────────────────────────────────┘
┌──────────────────────────────────┐
│ default                      ▾ │ ✕
└──────────────────────────────────┘
[➕ 添加分组]
```

约束：
- 列表至少 1 行，第一行不可删除（但允许清空选择，代表"使用用户默认分组"）。
- 每个 Select 的 options 复用现有 `loadGroups()`，**包括下拉里出现的 "auto" 项**（如果 admin 在用户可用分组里允许了 auto）—— "auto" 在多分组列表里是合法占位项。
- 客户端过滤已选项防止重复（同一个 group 不能选两次）。
- 仅当列表长度=1 且选中值为 "auto" 时显示 `cross_group_retry` 开关；其它情况隐藏。

提交语义（前端 onSubmit 处理）：

```js
// 1. 把 N 个 Select 的值收集到数组并过滤空值
const groups = formValues.groupsList
    .map(s => (s || '').trim())
    .filter(s => s !== '');

// 2. 直接提交（后端 NormalizeGroups 会再做一次去重 + 归一化）
payload.groups = groups;        // []  或  ["vip"]  或  ["vip","default"]
delete payload.groupsList;       // 仅前端用的临时字段不传
```

约定：
- 全部行清空 → `groups: []` → 后端等价"使用用户分组"
- 仅 1 行有值 → `groups: ["vip"]` → 后端归一化为 `Group="vip", GroupsJSON=""`，**完全等价单分组老行为**
- 多行 → `groups: ["vip","default"]` → 后端走多分组路径

#### TokensColumnDefs.jsx

`renderGroupColumn` 增加多分组分支：当 `token.groups.length > 1` 时渲染一组 Tag（按顺序），每个 Tag 显示 `<分组名> · <倍率>`；单分组保持现有渲染。

### 数据库迁移

GORM `db.AutoMigrate(&Token{})`（已存在于 `model/main.go`）会自动新增 `groups TEXT` 列（GORM 按 `column:groups;type:text` tag 创建）。SQLite / MySQL ≥ 5.7.8 / PostgreSQL ≥ 9.6 均原生支持 TEXT。

老数据：`groups` 列为空，`AfterFind` hook 把 `Groups` 设为 `nil`，`EffectiveGroups()` 自动回退到 `[Group]`，**行为完全等价于改造前**。

### 国际化

`web/src/i18n/locales/zh.json`（及 `zh-CN.json`、`zh-TW.json`、`en.json`、`fr.json`、`ja.json`、`ru.json`、`vi.json`）新增 key：

- `添加分组` — "Add Group" / "Ajouter un groupe" / "グループを追加" / "Добавить группу" / "Thêm nhóm"
- `不能选择重复分组` — "Cannot select duplicate groups" / ...
- （可选）`已达分组上限` — 若做 UI 上限校验（建议 ≤ 10）

i18n 文件按 CLAUDE.md Rule 7.3 做 deep merge 维护（避免与 upstream 合并冲突）。

## Fork 维护

按 CLAUDE.md Rule 7，标记一律使用 `// custom: token multi-group`。

### 新增文件（零冲突风险）

- `model/token_groups_custom.go` — `AfterFind` hook + `NormalizeGroups` + `EffectiveGroups`
- `model/token_groups_custom_test.go` — 单测
- `service/token_effective_groups_custom.go` — `GetTokenEffectiveAutoGroups` + `expandAutoEntries`
- `service/token_effective_groups_custom_test.go` — 单测

### 修改上游文件（带 `custom: token multi-group` 标记）

| 文件 | 改动行数 | 风险 |
|------|---------|------|
| `model/token.go` | +2 字段（`GroupsJSON string`、`Groups []string`） | Low |
| `controller/token.go` | AddToken/UpdateToken 各 +3 行（抄 Groups + 调 NormalizeGroups）+ Update Select 增 "groups" | Low |
| `middleware/auth.go` | 重构 line 382-399 段落为三分支（+15 行） | **Medium** |
| `constant/context_key.go` | +1 ctx key | Low |
| `service/channel_select.go` | 1 行替换 | Low |
| `middleware/distributor.go` | 1 行替换 | Low |
| `web/src/components/table/tokens/modals/EditTokenModal.jsx` | UI 重构 group 字段为多 Select 列表 + onSubmit 收集 | **Medium** |
| `web/src/components/table/tokens/TokensColumnDefs.jsx` | renderGroupColumn 增多分组分支 | Low |
| `web/src/i18n/locales/{zh,zh-CN,zh-TW,en,fr,ja,ru,vi}.json` | +2~3 i18n keys per locale | Low |
| `VERSION` | bump 到 `v0.13.2-ruoli-0.12` | Low |

`middleware/auth.go` 是行为性改动（不仅是新增），需要按 Rule 7.2 加块注释：

```go
// custom: token multi-group
// Original: if token.Group != "" { ... 单分组校验 ... } else { userGroup = ... }
// Changed: 增加 len(tokenGroups) > 1 分支，预先校验每项 + 写入 ContextKeyTokenGroupList，
//   ContextKeyUsingGroup 设为 "auto" 以复用现有 auto 路径。
// Revert: 删除 case len(tokenGroups) > 1 整段即可恢复上游行为。
```

## 测试

### 单元测试

- `model/token_groups_custom_test.go`
  - `NormalizeGroups`：空 / 1 项 / 多项 / 含空白 / 含重复 / 含纯空字符串 的归一化
  - `NormalizeGroups`：len > 1 时强制 `CrossGroupRetry = true`（即使入参 false）
  - `AfterFind`：模拟 GroupsJSON 各种状态（空 / 合法 JSON / 非法 JSON），验证 `Groups` 字段填充
  - `AfterFind`：非法 JSON 不返回 error，但 Groups 应为 nil + 有 SysError 日志
  - `EffectiveGroups`：三态（多项 / 单 Group / 全空）

- `service/token_effective_groups_custom_test.go`
  - 空 ctx → `GetUserAutoGroup(userGroup)`
  - ctx 带 `[vip, default]` 且 userGroup 全部可访问 → `[vip, default]`
  - ctx 带 `[auto, vip]`，userAutoGroups=[A, B]，→ `[A, B, vip]`（auto 展开 + 去重）
  - ctx 带 `[vip, auto, vip]` → `[vip, A, B]`（顺序保持 + 重复去除）
  - ctx 带 `[]` → fallback 到 `GetUserAutoGroup`

### 集成 / 手工 E2E

- 创建一个 token，分组列表 `[vipA, default]`。
- 用 vipA 没渠道的模型发请求，确认：
  1. 渠道命中的是 default
  2. 日志里 `group` 列显示 `default`
  3. 扣费按 default 分组倍率计算
- 用 vipA 有渠道的模型发请求，确认命中 vipA 且按 vipA 倍率计费。
- 创建 1 项列表 `[vip]`，确认数据库里 `Group="vip" Groups=""`，行为与改造前一致。
- 创建空列表，确认行为等价于"使用用户分组"。

## 风险与回退

### 风险

- **`middleware/auth.go` 行为分歧**：三分支化是行为性改动，需要严密的 case 覆盖。多分组中混入弃用分组（除 `auto`）会被拒，符合现有单分组语义。

- **`ContextKeyUsingGroup = "auto"` 的语义稀释**：复用 "auto" 作为虚拟值意味着仅看 `relayInfo.UsingGroup` 已无法区分"全局 auto"和"多分组 token 的 auto"。代码内已验证项目里没有 `UsingGroup == "auto"` 的特判逻辑（grep 结果只在 `quota.go:116` 和 `relay/helper/price.go:49` 把 UsingGroup 重置为实际命中分组），所以这是一个"语义稀释但实际无影响"的代价。如有第三方监控/统计依赖该值做分类，应改用 `ContextKeyTokenGroupList` 区分。

- **Channel Affinity 缓存共享与命中弱化**：
  `service/channel_affinity.go::GetPreferredChannelByAffinity` 的 cache key 由 `(rule, modelName, usingGroup, affinityValue)` 组成。多分组 token 都使用 `usingGroup = "auto"`，意味着：
  1. **所有多分组 token 与 legacy auto token 共享同一组亲和性缓存键**。token A (`[vipA, default]`) 写入的亲和性 channel X，会被 token B (`[vipB, default]`) 在查 affinity 时看到。
  2. token B 命中 X 后，会在 `distributor.go:115-126` 走"反查 channel 属于哪个 group"逻辑（改造后用 `GetTokenEffectiveAutoGroups`），如果 X 属于 vipA、不在 B 的列表里 → 反查失败 → `channel = nil` → fall through 到 `CacheGetRandomSatisfiedChannel` 重新选 → **行为正确，但每次都重选，亲和性优化对 B 失效**。
  3. token B 命中并 `RecordChannelAffinity` 自己的 channel Y 后，覆盖缓存里 A 的 X → **A 下次也会重选**。

  **影响范围**：仅当用户对多分组 token 配置了亲和性规则、且不同 token 的列表交集很少时显著。对 legacy auto token 单独使用的场景无影响。

  **接受 trade-off**：暂不引入 token 维度的 affinity key（会显著扩大缓存空间且需修改 upstream `channel_affinity.go`）。后续若有强需求，可在 affinity rule 配置里加 "include_token_id" 维度作为升级路径。

- **前端 `TokensColumnDefs.jsx` 兼容性**：列表 API 返回新字段 `groups` 后，老前端缓存可能看不到。属于浏览器刷新即可解决的小问题。

- **`AfterFind` hook 覆盖面**：GORM hooks 仅在 GORM 方法（`First/Find/Where/...`）触发，不在 raw SQL 查询时触发。项目内 token 相关查询基本都走 GORM API，未发现 raw SQL 查询 token 的位置，但合并 upstream 时需要 spot-check 新增的查询路径。

### 回退路径

- 删除新增文件 + 在所有 `// custom: token multi-group` 标记处恢复原代码 → 行为完全回到改造前。
- 数据库中 `groups` 列保留也无害（仅多分组令牌的列表数据，不影响其它逻辑）。
- 前端按 git revert 即可。

## CLAUDE.md / CLAUDE.local.md 更新清单（实现后写）

- CLAUDE.md §7.2 表格新增以上 9 行修改文件
- CLAUDE.md §7.2 `自定义 Feature 标签` 表新增 `custom: token multi-group`
- CLAUDE.local.md `Custom Features` 章节新增"Token Multi-Group (令牌多分组)"小节
