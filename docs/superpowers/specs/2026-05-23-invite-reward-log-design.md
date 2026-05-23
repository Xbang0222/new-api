# 邀请返利明细（Invite Reward Log）设计文档

- **日期**: 2026-05-23
- **分支**: ruoli（fork）
- **目标版本**: v0.13.2-ruoli-0.14
- **状态**: Draft → 待 PR

## 背景

现有"邀请返利"功能（custom: invite rebate, PR #3495）只在 `User` 表上维护三个累加字段：

- `AffCount`：邀请人数
- `AffQuota`：待划转返利
- `AffHistoryQuota`：历史返利总额

**对用户透明的信息是聚合数字**，没有"我邀请了哪些人 / 谁给我带来了多少返利"的明细视图。运行时的 `model/user.go::ProcessInviterReward` 仅在 `RecordLog` 系统日志写一条文本消息，无结构化记录，也无法回查。

用户希望在自己页面看到两个视图：
1. **按人汇总**：每个被邀请人的累计充值额、累计返利、最后返利时间
2. **返利流水**：每笔返利发生时的时间、来源、金额

## 目标

1. 新增持久化明细表 `invite_reward_logs`，每发一次返利写一条
2. 把 `ProcessInviterReward` 三步操作（CAS 标记 / 加 aff_quota / 写 log）**包成单事务**，顺手治掉现有"非事务一致性"隐患
3. 启动时一次性补录历史返利（基于 `TopUp.InviterRewardSent = true` + 当前规则反推）
4. 提供两个 `UserAuth` endpoint 给用户自己看
5. 前端在现有 `InvitationCard` 上加按钮 → 弹 Modal → Tabs 切换两视图，复用现有 `TopupHistoryModal` 风格

## 非目标

- 不做管理员视图（用户只看自己的，YAGNI）
- 不做返利"明细脱敏"（用户明确要求不脱敏）
- 不做返利明细的修改 / 撤销 / 复算 endpoint
- 不解析 `RecordLog` 系统日志做历史补录（脆弱，只扫 `TopUp` 表）
- 不缓存到 Redis（DB 查询已经够快）
- 不改 `model/subscription.go` 续费返利逻辑（透明继承）

## 关键洞察

**"明细" = `ProcessInviterReward` 每次发放时的一次性快照**。包括返利规则当时的 type/value，这样**未来改规则不会让历史返利数字变值**。

补录脚本会有**精度上限**：如果你历史上改过返利规则，补录值会和真实发出值不符。务实方案是按当前规则反推 + UI 顶部 banner 提示。完美补录需要解析非结构化日志，性价比太低。

## 设计

### 数据模型

新表 `invite_reward_logs`（GORM 自动复数化命名）：

```go
// model/invite_reward_log_custom.go
// custom: invite reward log
type InviteRewardLog struct {
    Id            int    `json:"id" gorm:"primaryKey;autoIncrement"`
    InviterId     int    `json:"inviter_id" gorm:"index:idx_invite_reward_inviter_created,priority:1"`
    InviteeId     int    `json:"invitee_id" gorm:"index"`
    TopUpId       int    `json:"topup_id" gorm:"index"`               // 0 表示手动加额触发
    RechargeQuota int    `json:"recharge_quota"`                       // 触发返利的充值额（与 User.Quota 同单位）
    RewardQuota   int    `json:"reward_quota"`                         // 本次返利额
    RewardType    string `json:"reward_type" gorm:"type:varchar(16)"`  // "fixed" / "percentage" 规则快照
    RewardValue   int    `json:"reward_value"`                         // 规则数值快照
    CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime;index:idx_invite_reward_inviter_created,priority:2"`
}
```

**字段说明**：
- 全部类型与 `TopUp` / `User` 对齐（`int` / `int64`），跨 SQLite / MySQL 5.7.8+ / PostgreSQL 9.6+ 无差异
- 共享名复合索引 `(inviter_id, created_at)` 服务"我邀请的人的流水"主查询
- 单独 `invitee_id` 索引服务"补录脚本查重"
- `topup_id` 索引服务"按订单回查"（少用，但廉价）
- **不加硬唯一约束**：原因是 MySQL 5.7 不支持条件 partial unique（`WHERE topup_id > 0`）。靠应用层防重 + `TopUp.InviterRewardSent` 双重保险

### 后端逻辑：`ProcessInviterReward` 改造

**现状**（`model/user.go:348`）：

```go
// 三步在不同的 DB 调用里，整体非事务
DB.Model(&TopUp{}).Where(...).Update("inviter_reward_sent", true)  // CAS
DB.Model(&User{}).Where(...).Updates(map[string]interface{}{...})  // 加 aff_quota
RecordLog(...)                                                      // 写系统日志
```

中间崩可能"标记已发但 quota 没加"或"加了 quota 但日志没写"。

**改造后**：

```go
// custom: invite reward log
err := DB.Transaction(func(tx *gorm.DB) error {
    // 1) CAS：tx 内更新 topup.inviter_reward_sent
    if topUpId > 0 {
        result := tx.Model(&TopUp{}).
            Where("id = ? AND inviter_reward_sent = ?", topUpId, false).
            Update("inviter_reward_sent", true)
        if result.Error != nil { return fmt.Errorf("...: %w", result.Error) }
        if result.RowsAffected == 0 { return nil } // 已发，跳过
    }

    // 2) 算 rewardQuota（沿用现有 decimal 逻辑）
    var rewardQuota int
    // ... 同现状 ...
    if rewardQuota <= 0 { return nil }

    // 3) tx 内更新 aff_quota / aff_history
    if err := tx.Model(&User{}).Where("id = ?", user.InviterId).
        Updates(map[string]interface{}{
            "aff_quota":   gorm.Expr("aff_quota + ?", rewardQuota),
            "aff_history": gorm.Expr("aff_history + ?", rewardQuota),
        }).Error; err != nil {
        return err
    }

    // 4) tx 内 INSERT invite_reward_logs
    return tx.Create(&InviteRewardLog{
        InviterId:     user.InviterId,
        InviteeId:     userId,
        TopUpId:       topUpId,
        RechargeQuota: rechargeQuota,
        RewardQuota:   rewardQuota,
        RewardType:    common.InviterRewardType,
        RewardValue:   common.InviterRewardValue,
    }).Error
})
if err != nil { return err }

// 5) 事务外 RecordLog（与现状一致：日志失败不影响返利已发的事实）
RecordLog(user.InviterId, LogTypeSystem, logMessage)
return nil
```

**改动量**：`model/user.go::ProcessInviterReward` 约 **-10/+15** 行，全部带 `// custom: invite reward log` 标记。

**事务回滚的影响**：如果第 4 步 INSERT log 失败，整事务回滚（CAS、aff_quota 都不动），返利**完全没发**。这比现状"半发"安全。返利没发是调用方可观察的（caller 已有 SysLog warning 模式），不引入新行为。

### 后端逻辑：一次性补录脚本

**触发**：`model/main.go` 在 `AutoMigrate(&InviteRewardLog{})` 之后调一次。

**幂等**：用 Option key `InviteRewardLogBackfilled` 标记。值为 `"true"` 时跳过；否则执行。

```go
// model/invite_reward_log_custom.go
// custom: invite reward log
func BackfillInviteRewardLogs() error {
    if common.OptionMap["InviteRewardLogBackfilled"] == "true" {
        return nil
    }
    if common.InviterRewardType == "" || common.InviterRewardValue == 0 {
        // 规则关闭：不补录，也不设标记（等规则开启后下次启动重跑）
        return nil
    }

    var stats struct{ Processed, Skipped, Failed int }
    var lastId int = 0
    const batch = 200

    for {
        var topups []TopUp
        err := DB.Where("inviter_reward_sent = ? AND status = ? AND id > ?",
            true, common.TopUpStatusSuccess, lastId).
            Order("id ASC").Limit(batch).Find(&topups).Error
        if err != nil { return err }
        if len(topups) == 0 { break }
        lastId = topups[len(topups)-1].Id

        for _, t := range topups {
            // 1) 已有记录跳过
            var count int64
            DB.Model(&InviteRewardLog{}).
                Where("invitee_id = ? AND topup_id = ?", t.UserId, t.Id).
                Count(&count)
            if count > 0 { stats.Skipped++; continue }

            // 2) 找邀请者
            user, err := GetUserById(t.UserId, false)
            if err != nil || user.InviterId == 0 || user.InviterId == t.UserId {
                stats.Skipped++; continue
            }

            // 3) 计算 recharge_quota（按 PaymentProvider 重算，复用 ManualCompleteTopUp 的逻辑）
            rechargeQuota := computeQuotaFromTopUp(t)

            // 4) 按当前规则反推 reward_quota
            rewardQuota := computeRewardQuota(rechargeQuota)
            if rewardQuota <= 0 { stats.Skipped++; continue }

            // 5) 插入，CreatedAt 取 TopUp.CompleteTime（保留历史时间）
            log := InviteRewardLog{
                InviterId: user.InviterId, InviteeId: t.UserId, TopUpId: t.Id,
                RechargeQuota: rechargeQuota, RewardQuota: rewardQuota,
                RewardType: common.InviterRewardType, RewardValue: common.InviterRewardValue,
                CreatedAt: t.CompleteTime,
            }
            if err := DB.Create(&log).Error; err != nil {
                stats.Failed++
                common.SysError(fmt.Sprintf("backfill failed for topup %d: %v", t.Id, err))
                continue
            }
            stats.Processed++
        }
    }

    common.SysLog(fmt.Sprintf(
        "邀请返利明细补录完成: 处理 %d 条, 跳过 %d 条, 失败 %d 条",
        stats.Processed, stats.Skipped, stats.Failed))

    if stats.Failed == 0 {
        UpdateOption("InviteRewardLogBackfilled", "true")
    }
    return nil
}
```

**覆盖面**：
- ✓ 所有 `TopUp.InviterRewardSent = true AND status = Success` 的记录
- ✗ 手动加额触发的返利（不在 TopUp 表，无锚点）
- ✗ 订阅续费触发的且 `topUpId = 0` 的（同上）

**已知不精确**：如果历史上改过返利规则，补录值会按**当前规则**反推。UI 流水视图首行 banner 提示。

### API

两个 endpoint，注册到 `router/api-router.go` 紧挨现有 `/aff` 路由：

#### `GET /api/user/invites/summary?page=1&page_size=20`

按人汇总，按返利倒序、未充值垫底。SQL：

```sql
SELECT
  u.id AS invitee_id, u.username, u.display_name, u.created_at AS invited_at,
  COALESCE(SUM(log.recharge_quota), 0) AS total_recharge,
  COALESCE(SUM(log.reward_quota), 0)   AS total_reward,
  COALESCE(MAX(log.created_at), 0)     AS last_reward_at
FROM users u
LEFT JOIN invite_reward_logs log
  ON log.invitee_id = u.id AND log.inviter_id = :me
WHERE u.inviter_id = :me
GROUP BY u.id, u.username, u.display_name, u.created_at
ORDER BY total_reward DESC, u.created_at DESC
LIMIT ? OFFSET ?
```

Response：

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "invitee_id": 123, "username": "alice", "display_name": "Alice",
        "invited_at": 1700000000, "total_recharge": 1000000,
        "total_reward": 100000, "last_reward_at": 1701000000
      }
    ],
    "total": 25,
    "stats": {
      "total_invitees": 25,      // 直接读 User.AffCount
      "active_invitees": 10,      // SELECT COUNT(DISTINCT invitee_id) FROM invite_reward_logs WHERE inviter_id = ?
      "total_reward_all": 500000  // 直接读 User.AffHistoryQuota
    }
  }
}
```

#### `GET /api/user/invites/logs?page=1&page_size=20`

返利流水。SQL：

```sql
SELECT log.*, u.username, u.display_name
FROM invite_reward_logs log
LEFT JOIN users u ON u.id = log.invitee_id  -- LEFT JOIN: 软删除的 invitee 仍出现
WHERE log.inviter_id = :me
ORDER BY log.created_at DESC, log.id DESC
LIMIT ? OFFSET ?
```

Response 各字段同 InviteRewardLog 字段 + `username` / `display_name`（可为 NULL，前端处理为"已注销用户 #invitee_id"）。

**鉴权**：复用 `UserAuth` 中间件，`inviter_id` 强制取自 JWT，用户**不能**伪造 inviter_id 查别人的明细。

**字段裸露**：只暴露 `username` + `display_name`。**不返回 email/phone**，即使用户说不脱敏，邮箱手机仍属敏感，前端也用不上。

**分页上限**：`page_size <= 50`（与现有 TopUp 表格一致），防 DoS。

### 前端 UI

#### 入口

在 `web/src/components/topup/InvitationCard.jsx` 现有"划转额度"按钮**旁边**加一个 `[查看邀请明细]` 按钮：

```
┌──────────────────────────────────────────────────┐
│ 邀请奖励                                          │
│ 待使用收益: ¥12.34    总收益: ¥56.78             │
│ 邀请人数: 8                                       │
│ 邀请链接: https://.../register?aff=ABCD [复制]   │
│ [划转额度]  [查看邀请明细]  ← 新增              │
└──────────────────────────────────────────────────┘
```

改动量：`InvitationCard.jsx` +1 按钮 + 1 `useState`（开关 Modal）≈ **10 行**，全部 `{/* custom: invite reward log */}` 标记。

#### Modal 结构

```
┌────────────────────────────────────────────────────────────┐
│ 邀请明细                              [紧凑/自适应切换]  ✕ │
├────────────────────────────────────────────────────────────┤
│ 总览: 邀请 8 人 | 7 人产生过返利 | 累计返利 ¥56.78         │
├────────────────────────────────────────────────────────────┤
│  [ 按人汇总 ]  [ 返利流水 ]   ← Semi Tabs                  │
├────────────────────────────────────────────────────────────┤
│  (Tab 1) | 用户 | 注册时间 | 累计充值 | 累计返利 | 最后返利│
│  (Tab 2) ⓘ 功能上线前的数据按当前规则估算                  │
│         | 时间 | 用户 | 类型 | 充值额 | 返利额 | 规则      │
│  [分页器]                                                  │
└────────────────────────────────────────────────────────────┘
```

#### 复用

| 复用对象 | 来自 | 用法 |
|---|---|---|
| Modal 外壳 | `topup/modals/TopupHistoryModal.jsx` | 复用 Modal + 表格容器布局 |
| 紧凑/自适应双模式 | `useTableCompactMode` + `CompactModeToggle` | 直接 import |
| 时间列不换行 | `renderTimestampNoWrap` | 直接复用 |
| 数字格式化 | `renderQuota` | 项目惯例 |
| 表格 + 分页 | Semi `Table` + `Pagination` | 项目惯例 |

#### 新建文件

| 文件 | 作用 |
|---|---|
| `web/src/components/billing/InvitationDetailModal.jsx` | Modal 主体 + Tabs |
| `web/src/components/billing/InvitationSummaryTable.jsx` | 按人汇总表格 |
| `web/src/components/billing/InvitationLogsTable.jsx` | 流水表格 |
| `web/src/helpers/invitation.js` | API 调用封装 |

#### 状态与异常

- 加载中：Semi `Spin`
- 空状态（无邀请）：`暂无邀请记录` 占位 + "复制邀请链接"快捷按钮
- 错误：`Toast.error`
- 已注销 invitee：流水视图显示 `（已注销用户 #invitee_id）`

#### i18n

新增 key 采用项目"中文源串作为 key + 复合 key 加下划线"的避碰撞惯例：

| Key | 用途 |
|---|---|
| `查看邀请明细` | 按钮 |
| `邀请明细` | Modal 标题 |
| `按人汇总` / `返利流水` | Tab |
| `累计充值` / `累计返利` / `最后返利时间` | 列名 |
| `返利规则` | 列名 |
| `已注销用户` | 占位 |
| `邀请明细_流水说明` | banner 文本，详见下 |
| `邀请明细_空状态` | 空数据提示 |

banner 文本（中文源）：`功能上线前的返利数据按当前规则估算，部分手动加额/订阅续费的返利可能未收录`

需同步到 `web/src/i18n/locales/{zh,en,fr,ja,ru,vi}.json`。

## 错误处理与边界

| 场景 | 处理 |
|---|---|
| INSERT log 失败 | 整事务回滚（CAS、aff_quota 都不动），返利完全没发；caller 已有 SysLog warning 路径 |
| 事务后 RecordLog 失败 | 与现状一致，不影响返利已发的事实 |
| 补录单条失败 | 不中断整批，累计 `Failed` |
| 补录整批结束 | 写 SysLog 报告；只在 `Failed=0` 时置 `InviteRewardLogBackfilled=true` |
| 自邀请脏数据 | 补录跳过 `InviterId == invitee.Id`；运行时由注册逻辑防御 |
| 规则关闭 | 运行时早退；补录跳过整批 + 不置标记 |
| 手动加额（topUpId=0） | log 表 TopUpId=0 允许；前端 topup_id 显示为 "-" |
| 已注销 invitee | summary 自动隐藏（GORM 软删除）；流水显示"已注销用户 #ID" |
| 邀请关系运行时变更 | 历史 log 不动（快照），未来按新关系发放 |
| 鉴权 | inviter_id 强制取自 JWT，用户不能查别人 |
| 大量被邀请人 | 分页 page_size ≤ 50 |

## 测试策略

### 后端单元测试

参考 `model/payment_method_guard_test.go` 的 SQLite in-memory 模式：

| 测试 | 覆盖 |
|---|---|
| `TestProcessInviterReward_FirstTime` | CAS 成功 → quota 加 → log 写 1 条 |
| `TestProcessInviterReward_Duplicate` | CAS 失败 → return nil → log 不增长 |
| `TestProcessInviterReward_InsertLogFail_Rollback` | 注入 INSERT 失败 → quota 不加、`inviter_reward_sent` 不变 |
| `TestProcessInviterReward_ManualGrant` | topUpId=0 → log 写入，TopUpId=0 |
| `TestProcessInviterReward_RuleDisabled` | type="" → 不进事务、不写 log |
| `TestBackfill_Basic` | N 个历史 TopUp → log 表 N 条 |
| `TestBackfill_Idempotent` | 重复跑不重复增长 |
| `TestBackfill_SkipSelfInvite` | InviterId==UserId 跳过 |
| `TestBackfill_RuleDisabled` | 全跳过 + 不置 InviteRewardLogBackfilled |

### API integration 测试

- summary 排序：返利倒序，未充值垫底
- summary stats: active_invitees = DISTINCT(invitee_id)
- logs 时间倒序 + 软删除 invitee 显示 username=NULL
- 鉴权：A 用户 endpoint 不能看到 B 的明细

### 前端手测

`bun run dev` 后：
- Golden path：InvitationCard → 查看明细 → Modal → Tab 切换 → 数据正确
- 边界：空状态、未充值的人、已注销的人、分页、紧凑/自适应切换
- i18n：切语言不崩，文案完整

## 性能

- 索引 `(inviter_id, created_at)` 覆盖两个主查询，单用户 5 万条 log 也是毫秒级
- 补录估计：1 万条 < 30 秒；如过慢可优化为 `CreateInBatches(batch=200)`
- 不上 Redis 缓存（YAGNI）
- summary 视图 GROUP BY 在 invite_count > 1000 时可能略慢，但用户级别基本不会触及（KOL 也几百级）

## 文件落位与改动量

### 新建文件（零冲突风险，§7.1 隔离）

| 文件 | 性质 |
|---|---|
| `model/invite_reward_log_custom.go` | 表 + DAO + 补录函数 |
| `controller/invite_reward_custom.go` | 两个 handler |
| `web/src/components/billing/InvitationDetailModal.jsx` | Modal |
| `web/src/components/billing/InvitationSummaryTable.jsx` | 表格 |
| `web/src/components/billing/InvitationLogsTable.jsx` | 表格 |
| `web/src/helpers/invitation.js` | API helper |

### 上游文件改动（§7.2 标记，全部 `// custom: invite reward log`）

| 文件 | 改动 | 风险 |
|---|---|---|
| `model/main.go` | +2 行（AutoMigrate + 调补录） | Low |
| `model/user.go::ProcessInviterReward` | -10/+15 行（事务化 + 插 log） | **Medium** |
| `model/option.go` | +1 行（注册 `InviteRewardLogBackfilled`） | Low |
| `router/api-router.go` | +2 行（注册两个 endpoint） | Low |
| `web/src/components/topup/InvitationCard.jsx` | +1 按钮 + state ≈ 10 行 | Low |
| `web/src/i18n/locales/{zh,en,fr,ja,ru,vi}.json` | 各 +8 个 key | Low |

**上游文件总改动 ~30 行**，符合 §7.2 "最小化 upstream diff" 规则。

## 上线步骤

1. `go test ./model/... ./controller/...` 全绿
2. `go build ./...` clean
3. `cd web && bun run build` clean
4. `cd web && bun run dev` 手测前端全流程
5. `VERSION` 更新到 `v0.13.2-ruoli-0.14`
6. `CLAUDE.local.md` 补一节 `Invite Reward Log (邀请返利明细)` 说明
7. `CLAUDE.md §7.2` 上游修改表格补 5 行
8. Docker build/push（按 CLAUDE.local.md 流程）
9. 服务器侧：拉镜像 + 重启 → 启动日志确认看到 `邀请返利明细补录完成: 处理 X 条, 跳过 Y 条, 失败 0 条`
10. 网页验证：用有邀请历史的账号打开 InvitationCard → 查看明细 → 数据完整

## 上游同步建议

如果上游未来：
- 给 `ProcessInviterReward` 做事务化 → 我们的事务化 diff 会冲突，保留上游版本，再 cherry-pick INSERT log 那 5 行
- 加自己的返利明细系统 → 评估字段差异，必要时迁移数据后退我们的表

`invite_reward_logs` 表本身、`controller/invite_reward_custom.go`、前端 `components/billing/Invitation*` 都是 `_custom` 文件 / 新文件，零冲突。
