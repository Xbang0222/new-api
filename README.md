<div align="center">

![new-api](https://raw.githubusercontent.com/QuantumNous/new-api/main/web/default/public/logo.png)

# New API · ruoli fork

二次开发版的 AI 网关 / 资产管理系统

[上游 QuantumNous/new-api](https://github.com/QuantumNous/new-api) · [License (Apache 2.0)](./LICENSE) · `v0.13.2-ruoli-0.14`

</div>

> [!IMPORTANT]
> Fork 自 [QuantumNous/new-api](https://github.com/QuantumNous/new-api) 的 `ruoli` 分支，**仅供个人学习使用**。
> 上游品牌、署名与许可证全部保留；完整功能、官方文档、社区支持请前往上游仓库。

## 定制特性

> 上游 new-api 已有的能力（多渠道分发、令牌、订阅、计费表达式、Webhook、邀请系统等）参阅 [上游 README](https://github.com/QuantumNous/new-api)。下表只列本 fork **额外** 增加的能力。

- **数据面板**：消耗排行 / 趋势对普通用户开放，其他用户名脱敏为 `a***n`，附自己的排名徽章
- **发票模块**：用户申请、抬头管理、后台审核，开票服务费率可配置
- **钱包支付订阅**：用 `User.Quota` 直接购买订阅，原子事务，免外部支付通道
- **订阅周期限购**：按订阅自身周期滚动计数，替代上游的"终身计数"
- **订阅扣费顺序**：拖拽排序，历史 / 濒尽订阅自动折叠，管理员 `sort_order` 仅影响展示
- **令牌多分组**：一个 token 挂多个分组（有序），按列表顺序熔断到第一个有可用渠道的分组
- **仅 Claude Code 客户端**：渠道级白名单 + 路径 + UA/Session/SystemPrompt 指纹双层校验，严格 403
- **亲和性缓存自动失效**：禁用渠道时自动清除并 fallback
- **分组时段定价**：每个分组可叠加时段表（夜间折扣 / 高峰溢价 / 周末特价），按请求开始时刻锁价
- **邀请充值返利**：来自上游未合并 PR #3495，附可配置的划转门槛防薅羊毛
- **顶栏商城外链**：与"文档"同位置，含 `javascript:` 等危险协议过滤
- **品牌定制层**：Logo / 字体 / favicon 由 `web/src/helpers/brand.js` 集中管理
- **统一双模式表格**：充值 / 账单 / 发票表格紧凑—自适应切换，列宽自适应

完整功能演变记录见 [`CLAUDE.local.md`](./CLAUDE.local.md)。

## 部署

```bash
docker run -d --name new-api --restart always \
  -p 3000:3000 -e TZ=Asia/Shanghai \
  -v /opt/new-api/data:/data \
  xbang0222/new-api:latest
```

镜像：`xbang0222/new-api:latest` 或具体版本号（见 [`VERSION`](./VERSION)）。
MySQL + Redis 生产部署、构建推送、回滚详见 [`CLAUDE.local.md`](./CLAUDE.local.md)。

## 开发

| 文档 | 内容 |
|------|------|
| [`CLAUDE.md`](./CLAUDE.md) | 项目规约（JSON 封装、跨数据库、i18n、Fork 维护） |
| [`CLAUDE.local.md`](./CLAUDE.local.md) | 定制功能详单与本地运维（不入 git） |
| [`pkg/billingexpr/expr.md`](./pkg/billingexpr/expr.md) | 计费表达式系统 |

上游同步：

```bash
git fetch upstream
git merge upstream/main --no-edit
go build ./... && cd web && bun run build
```

## License

继承上游 [Apache License 2.0](./LICENSE)。感谢上游 [QuantumNous/new-api](https://github.com/QuantumNous/new-api) 作者与贡献者。
