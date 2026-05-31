<div align="center">

![New API](https://raw.githubusercontent.com/QuantumNous/new-api/main/web/default/public/logo.png)

# New API · ruoli fork

基于 [QuantumNous/new-api](https://github.com/QuantumNous/new-api) 的二次开发(fork)版 AI 网关 —— 在上游之上增加了一批面向**运营、计费与渠道可靠性**的定制能力。

[上游 QuantumNous/new-api](https://github.com/QuantumNous/new-api) · [Apache 2.0](./LICENSE) · `v0.13.2-ruoli-0.19`

</div>

> [!IMPORTANT]
> 本仓库是 [QuantumNous/new-api](https://github.com/QuantumNous/new-api) `ruoli` 分支的**二次开发(fork)**,仅供个人学习。上游品牌、署名与许可证全部保留。
> 上游本身已有的能力(40+ 渠道聚合分发、令牌、订阅、计费表达式、Webhook、邀请系统、管理后台等)请以上游仓库为准 —— **本 README 只介绍本 fork 二开新增 / 改造的功能。**

## 二开定制特性

以下为相对上游**额外**增加或改造的能力,按领域分组。

### 渠道与路由

- **渠道失败重试故障转移** —— 失败渠道排除后,在同一优先级内换没试过的渠道 → 同档耗尽才降级 → 全部用尽才报**真实上游错误**,绝不重试已失败渠道;多 key 渠道按 key 数轮换,`RetryTimes` 作全局总预算。
- **令牌多分组** —— 一个 token 可挂多个有序分组,按顺序熔断到第一个有可用渠道的分组,并按命中分组计费。
- **仅 Claude Code 客户端** —— 渠道级白名单:路径 + UA / Session / SystemPrompt 指纹双层校验,非 Claude Code 请求严格 403、不 fallback、不重试。
- **亲和性缓存自动失效** —— 禁用渠道时自动清除渠道亲和性缓存并 fallback。

### 订阅 · 钱包 · 发票

- **钱包支付订阅** —— 用账户余额(`User.Quota`)单事务原子购买订阅,免外部支付通道。
- **订阅周期限购** —— 按订阅自身周期滚动计数限购,替代上游的"终身计数"。
- **订阅扣费顺序** —— 多个生效订阅的扣费顺序可拖拽排序,历史 / 濒尽订阅自动折叠收纳。
- **发票模块** —— 用户申请、抬头管理、后台审核的完整开票流程,开票服务费率可配置。

### 邀请返利

- **邀请充值返利** —— 被邀请人充值时给邀请人返利(源自上游未合并 PR #3495),附可配置的划转门槛防薅羊毛。
- **邀请返利明细** —— 每次返利发放持久化(含规则快照),用户可按人 / 按流水两视图查看,支持历史补录。

### 界面 · 运营

- **Token 消耗排行榜** —— 消耗排行 / 趋势对所有登录用户开放,他人用户名脱敏为 `a***n`,附本人排名徽章。
- **顶栏商城外链** —— 顶栏可配"商城"外链(与"文档"同位置),含 `javascript:` 等危险协议过滤。
- **顶栏服务状态外链** —— 顶栏可配"服务状态"外链(Uptime Kuma / status.io 等),复用同一危险协议过滤。
- **模型广场卡片字体 / 价格** —— 卡片视图模型名 / 价格改用自带西文字体,价格标签精简(输入 / 输出 / 缓存,后缀 `/M`)。
- **充值 / 账单双模式表格** —— 充值、账单、发票等表格支持紧凑 / 自适应双模式切换,列宽自适应、时间不换行。
- **品牌定制层** —— Logo / 字体 / favicon 经 `web/src/helpers/brand.js` 集中管理。

> [!NOTE]
> 完整功能演变与设计取舍记录见 [`CLAUDE.local.md`](./CLAUDE.local.md)。

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.22+ · Gin · GORM v2 |
| 前端 | React 18 · Vite · Semi Design(Bun 包管理) |
| 数据库 | SQLite / MySQL ≥ 5.7.8 / PostgreSQL ≥ 9.6(三库同时兼容) |
| 缓存 | Redis + 内存缓存 |
| 认证 | JWT · WebAuthn / Passkey · OAuth(GitHub / Discord / OIDC) |

## 部署

```bash
docker run -d --name new-api --restart always \
  -p 3000:3000 -e TZ=Asia/Shanghai \
  -v /opt/new-api/data:/data \
  xbang0222/new-api:latest
```

镜像:`xbang0222/new-api:latest` 或具体版本号(见 [`VERSION`](./VERSION))。MySQL + Redis 生产部署、构建推送与回滚流程见 [`CLAUDE.local.md`](./CLAUDE.local.md)。

## 开发

```bash
# 后端(默认连项目根 one-api.db)
go build ./... && TZ=Asia/Shanghai go run main.go
# 前端
cd web && bun install && bun run dev
```

| 文档 | 内容 |
|------|------|
| [`CLAUDE.md`](./CLAUDE.md) | 项目规约(JSON 封装、跨数据库、i18n、Fork 维护) |
| [`CLAUDE.local.md`](./CLAUDE.local.md) | 定制功能详单与本地运维(不入 git) |
| [`pkg/billingexpr/expr.md`](./pkg/billingexpr/expr.md) | 计费表达式系统 |

上游同步:

```bash
git fetch upstream
git merge upstream/main --no-edit
go build ./... && cd web && bun run build
```

## 致谢

本项目二次开发自 [QuantumNous/new-api](https://github.com/QuantumNous/new-api),继承其 [Apache License 2.0](./LICENSE)。感谢上游作者与全体贡献者。
