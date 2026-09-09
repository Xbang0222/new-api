# 订阅扣款顺序

钱包中的「订阅扣款顺序」用于安排当前用户持有的有效订阅。拖动手柄、使用方向键或上下按钮调整列表，点击「保存扣款顺序」后生效。已过期或已取消的订阅单独展示，不参与排序。

## 扣款规则

- 保存的顺序优先，列表上方的订阅先被检查。
- 当前订阅的剩余额度不足以覆盖本次预扣时，继续检查下一项；不会将一次预扣拆到多个订阅。
- 无限额度订阅仍按相同顺序参与选择。周期额度重置逻辑保持原有行为。
- 未设置顺序时，按到期时间升序选择；到期时间相同则按订阅 ID 升序。
- 保存后新购买、未列入自定义顺序的订阅放在已指定订阅之后，再按到期时间、ID 升序排列。
- 失效的订阅 ID 不影响剩余有效订阅。恢复「最早到期优先」后，后续新订阅也按到期时间自动排序。
- 排序只影响之后的新预扣。已经预扣的请求继续在原订阅上结算或退款。
- 「优先订阅 / 优先钱包 / 仅订阅 / 仅钱包」仍独立决定资金来源；本设置仅决定使用订阅时的内部顺序。

## 面板接口

`GET /api/subscription/self` 返回 `subscriptions`（按实际扣款顺序排列的有效订阅）、`all_subscriptions`（包含历史订阅）、`subscription_order`（保存顺序中仍有效的 ID，默认 `[]`）及 `billing_preference`。

`PUT /api/subscription/self/order` 使用面板用户鉴权，接收：

```json
{"subscription_ids": [123, 456]}
```

`subscription_ids` 必须存在且为整数数组；空数组恢复默认。请求体在 JSON 解析前限制为 32 KiB，最多接受 1000 项，拒绝重复、非正、无归属、过期、已取消或不存在的 ID。未列出的有效订阅自动按默认规则接在已指定项后。成功响应与 `GET /api/subscription/self` 相同。

顺序保存在现有 `users.setting` JSON 的 `subscription_order` 字段，无新增数据库表、列、索引或迁移。其他设置保存不会覆盖该字段。数据库写入成功后的缓存刷新失败会记录日志；页面读取和扣款选择直接读取数据库中的排序。

## 回归验证

- `go test ./model -run 'Test(SubscriptionDebitOrderDatabaseContract|SortSubscriptionsForDebit)' -count=1`：默认、自定义、恢复、归属校验、缓存失败、额度不足、无限额度、新订阅、失效、预扣幂等和退款事务回滚。
- `go test ./controller -run 'Test(GetSubscriptionSelf|UpdateSubscriptionOrder|UpdateUserNotificationSettingsPreservesSubscriptionOrder)' -count=1`：接口读回、非法请求和通知设置入口的兼容性。
- 前端 `subscription-order-editor.test.tsx`：真实 motion 指针拖动、键盘排序、保存成功/失败、恢复默认、保存时禁用及历史订阅展示。

数据库契约测试默认使用独立的 SQLite 文件。也可设置 `SUBSCRIPTION_TEST_MYSQL_DSN` 或 `SUBSCRIPTION_TEST_POSTGRES_DSN`，在真实数据库上执行相同测试。两种 DSN 必须指向专用的 `codex_subscription_order_*` 测试数据库；测试会重建其中的用户和订阅相关表，请勿使用业务数据库。
