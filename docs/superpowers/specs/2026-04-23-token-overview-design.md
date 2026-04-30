# Token Overview And Usage Detail Design

**日期：** 2026-04-23  
**状态：** 已确认，待用户复核  
**关联页面：** 主控制台总览卡片、账号池表格、账号详情抽屉、新增 Token 概览抽屉

## 1. 目标

本次改造聚焦解决当前控制台中 token 用量“有数字但不够真实、不够有用”的问题，并补足以下体验：

1. 将 token 用量从“进程内临时累计”升级为“真实累计、可跨重启保留的逐日聚合统计”。
2. 顶部总览区把现有“令牌流量”升级为“Token 概览”，默认展示今日消耗。
3. 点击 `Token 概览` 卡片后，打开与账号详情抽屉同风格的细化面板，展示今日、近 7 日、近 30 日和累计统计。
4. 账号池表格中的 token 用量采用 `K / M` 紧凑单位，提升扫表效率。
5. 账号详情抽屉中展示完整数字与周期分布，让单账号分析更可信、更细。

## 2. 当前问题

### 2.1 数据口径问题

现有 token 统计来自运行时内存中的累计字段：

- 单账号 usage 由 `Account.RecordUsage()` 累加。
- `/stats` 中的 `summary.total_input_tokens` 与 `summary.total_output_tokens` 由当前进程中的账号快照求和得到。
- 这些统计不会持久化，服务重启后归零。
- 现有结构无法区分“今天”“近 7 天”“近 30 天”的消耗。

因此当前数字更准确的含义是：

> 当前服务进程在本次运行期间捕获到的 usage 累计值。

这个定义不适合继续作为运营面板中的核心指标。

### 2.2 展示问题

当前展示存在三个不足：

1. 顶部卡片只有输入 / 输出两行，没有总量、时间范围、平均请求消耗，也无法进入更细分的分析视图。
2. 账号池表格只显示一个 `usage.totalTokens`，无法快速判断账号近期活跃度，也不适合扫大数。
3. 账号详情抽屉只有累计输入 / 输出 / 总量，没有周期视角，无法回答“这个账号今天是不是异常活跃”“这 7 天是否稳定消耗”。

## 3. 设计原则

### 3.1 口径优先于包装

所有展示都必须基于真实累计的逐日聚合数据，而不是根据总量反推近 7 日和近 30 日。

### 3.2 主面板看今天，抽屉看全貌

- 主面板强调“现在该怎么判断号池状态”，因此默认展示今日 token 概览。
- 抽屉承载“细看趋势和分布”，负责展示周期对比与账号贡献。

### 3.3 与现有视觉语言一致

- 新增面板使用现有 [Drawer](/E:/work/codex-proxy/web/src/components/ui/Drawer.tsx) 容器。
- 内部卡片层级、半透明底色、圆角、弱高光、说明文案密度，与 [AccountDetailDrawer.tsx](/E:/work/codex-proxy/web/src/features/account-detail/AccountDetailDrawer.tsx) 保持统一。
- 不引入另一套图表风格；第一版以数字卡片、进度条、简洁列表为主。

## 4. 数据模型设计

### 4.1 新增逐日聚合表

新增一张 token usage 日聚合表，用于跨重启保留真实累计数据。

建议字段：

- `id`
- `account_key`
- `account_id`
- `email`
- `usage_date`
- `request_count`
- `input_tokens`
- `output_tokens`
- `total_tokens`
- `created_at`
- `updated_at`

约束与说明：

- `usage_date` 使用服务端本地日界限，按自然日聚合。
- `account_key` 作为稳定归属键，优先使用 `account_id`，缺失时退回 `email`。
- 建立 `(account_key, usage_date)` 唯一键，用于 upsert。
- `request_count` 记录当日成功记录到 usage 的请求次数。

### 4.2 写入策略

每次上游返回有效 usage 后：

1. 继续更新内存态 `Account.RecordUsage()`，保留热路径快速读取能力。
2. 同步或异步写入 usage 日聚合表，对当日记录执行 upsert：
   - `request_count += 1`
   - `input_tokens += input`
   - `output_tokens += output`
   - `total_tokens += total`

第一版要求：

- 写失败不影响主请求返回。
- 写失败应记录日志，便于后续排查。
- 若 `total_tokens` 缺失，则以 `input + output` 补齐，与当前行为保持一致。

## 5. 接口设计

### 5.1 `/stats` 扩展字段

保留现有 `/stats`，在 summary 和单账号 usage 上增加聚合统计字段。

#### summary 新增

- `token_overview.today.input_tokens`
- `token_overview.today.output_tokens`
- `token_overview.today.total_tokens`
- `token_overview.today.request_count`
- `token_overview.seven_days.input_tokens`
- `token_overview.seven_days.output_tokens`
- `token_overview.seven_days.total_tokens`
- `token_overview.seven_days.request_count`
- `token_overview.thirty_days.input_tokens`
- `token_overview.thirty_days.output_tokens`
- `token_overview.thirty_days.total_tokens`
- `token_overview.thirty_days.request_count`
- `token_overview.lifetime.input_tokens`
- `token_overview.lifetime.output_tokens`
- `token_overview.lifetime.total_tokens`
- `token_overview.lifetime.request_count`
- `token_overview.updated_at`

兼容策略：

- 保留当前 `total_input_tokens` / `total_output_tokens` 字段，但前端新版本不再依赖它们作为主展示字段。
- 第一版可将 `lifetime` 作为新主口径，旧字段视为兼容输出。

#### account usage 新增

每个账号的 `usage` 增加：

- `today_input_tokens`
- `today_output_tokens`
- `today_total_tokens`
- `today_request_count`
- `seven_day_input_tokens`
- `seven_day_output_tokens`
- `seven_day_total_tokens`
- `seven_day_request_count`
- `thirty_day_input_tokens`
- `thirty_day_output_tokens`
- `thirty_day_total_tokens`
- `thirty_day_request_count`
- `lifetime_input_tokens`
- `lifetime_output_tokens`
- `lifetime_total_tokens`
- `lifetime_request_count`

兼容策略：

- 现有 `input_tokens / output_tokens / total_tokens / total_completions` 继续保留。
- 前端新展示优先使用 `lifetime_*`；若聚合字段为空，再退回旧字段。

### 5.2 是否新增独立接口

第一版不强制新增独立 token 接口，优先在 `/stats` 中返回所需聚合数据。

原因：

- 当前 dashboard 已经以 `/stats` 为核心载体。
- 顶部概览与账号表格本来就依赖同一请求。
- 抽屉细化面板第一版不需要图表级大 payload，扩展 `/stats` 即可满足。

若实现时发现 `/stats` 响应过重，可在后续拆出 `/stats/token-overview`，但不作为本轮前置条件。

## 6. 前端信息架构

### 6.1 顶部卡片区

现有“令牌流量”卡片改为“Token 概览”。

卡片展示内容：

- 标题：`Token 概览`
- 主值：`今日总量`
- 次级信息：
  - `输入`
  - `输出`
  - `请求数`
- 辅助说明：`今日统计`

交互：

- 整张卡片可点击。
- hover 时视觉反馈与表格行、现有卡片 hover 一致。
- 点击后打开 `TokenOverviewDrawer`。

### 6.2 账号池表格

表格中原 `令牌用量` 列改为 `今日 / 累计` 双层展示：

- 第一行：今日 total tokens，使用紧凑数字格式。
- 第二行：累计 total tokens，使用更轻的次级文字和紧凑格式。

紧凑格式要求：

- `950` 保持原样。
- `12.4K`
- `1.2M`
- `3.4B`

若今日无消耗：

- 第一行显示 `0`
- 第二行仍显示累计

不在表格中平铺输入 / 输出，避免列宽过重。

### 6.3 账号详情抽屉

账号详情中的“用量概览”升级为两层结构。

第一层：周期总览

- 今日总量
- 近 7 日总量
- 近 30 日总量
- 累计总量

第二层：累计拆分

- 完成次数
- 累计输入
- 累计输出
- 累计请求数

展示要求：

- 周期总览使用完整数字格式。
- 累计总量仍作为最强调的大数字区域。
- 补一行说明：`统计基于系统记录到的真实 usage 聚合，不等同于 OpenAI 官方账单。`

### 6.4 Token 概览抽屉

新增 `TokenOverviewDrawer`，视觉和账号详情抽屉保持同级。

抽屉分为四块：

#### A. 今日概览

- 今日总 tokens
- 今日输入
- 今日输出
- 今日请求数

#### B. 周期对比

展示三张小卡：

- 近 7 日
- 近 30 日
- 累计

每张卡展示：

- 总 tokens
- 输入 / 输出
- 请求数

#### C. 账号贡献 Top

展示最近 30 日 token 消耗最高的账号列表，默认 5 条：

- 邮箱
- 近 30 日 total tokens
- 今日 total tokens

采用和账号表格一致的轻列表风格，不另做复杂图表。

#### D. 统计说明

说明以下内容：

- 统计口径为系统记录到的成功 usage 聚合。
- 今日 / 7 日 / 30 日基于自然日聚合。
- 与官方账单、上游控制台可能存在差异。

## 7. 组件设计

### 7.1 新增组件

- `web/src/features/dashboard/TokenOverviewCard.tsx`
- `web/src/features/token-overview/TokenOverviewDrawer.tsx`
- `web/src/features/token-overview/TokenSummarySection.tsx`
- `web/src/features/token-overview/TopAccountsByTokenList.tsx`

### 7.2 现有组件改造

- [StatsOverview.tsx](/E:/work/codex-proxy/web/src/features/dashboard/StatsOverview.tsx)
  - 将最后一张卡从静态展示改为可点击的 token 概览入口。
- [AccountsTable.tsx](/E:/work/codex-proxy/web/src/features/dashboard/AccountsTable.tsx)
  - 调整 token 列为今日 / 累计双层紧凑展示。
- [AccountDetailDrawer.tsx](/E:/work/codex-proxy/web/src/features/account-detail/AccountDetailDrawer.tsx)
  - 引入周期 token 总览与完整数字展示。
- [QuotaPanel.tsx](/E:/work/codex-proxy/web/src/features/account-detail/QuotaPanel.tsx)
  - 将 `历史令牌消耗` 文案替换为更准确的 `累计 Token`，避免和新周期统计混淆。

## 8. 类型与格式化设计

### 8.1 类型

扩展 [types.ts](/E:/work/codex-proxy/web/src/lib/types.ts)：

- `TokenBucketView`
- `TokenOverviewView`
- `AccountUsageWindowView`

建议结构：

- `TokenBucketView`
  - `inputTokens`
  - `outputTokens`
  - `totalTokens`
  - `requestCount`

- `TokenOverviewView`
  - `today`
  - `sevenDays`
  - `thirtyDays`
  - `lifetime`
  - `updatedAt`

- `UsageView`
  - 保留旧字段
  - 新增 `today`、`sevenDays`、`thirtyDays`、`lifetime`

### 8.2 格式化

扩展 [format.ts](/E:/work/codex-proxy/web/src/lib/format.ts)：

- 新增 `formatTokenCompact`
- 新增 `formatTokenFull`

规则：

- 表格、顶部卡片、副文案使用 `formatTokenCompact`
- 详情抽屉与说明区使用 `formatTokenFull`

紧凑单位使用英文后缀：

- `K`
- `M`
- `B`

原因是运营视图中对 token 数量的扫读效率更高，且与技术语境兼容。

## 9. 错误与降级策略

### 9.1 聚合数据缺失

若后端聚合字段暂时为空：

- 顶部 `Token 概览` 卡片显示 `暂无统计`
- 账号详情抽屉中的周期卡片显示 `暂无`
- 账号池表格回退到旧的 `usage.totalTokens`

### 9.2 历史数据空白

新表上线初期，历史 7 日 / 30 日数据可能不足。

前端行为：

- 不伪造历史周期。
- 实际有多少展示多少。
- 文案保持中性，不显示告警。

### 9.3 统计写入失败

服务端写 usage 聚合失败时：

- 主请求不报错。
- 记录 warn/error 日志。
- 前端继续展示已存在的最近成功聚合结果。

## 10. 测试策略

### 10.1 后端

- usage 聚合表 schema 创建测试。
- 单次 usage 写入会正确 upsert。
- 同账号同日期多次写入会正确累加。
- 跨日期聚合能正确分桶。
- `/stats` 返回 summary 与 per-account 周期统计正确。

### 10.2 前端

- `StatsOverview` 能展示今日 token，并可点击打开抽屉。
- `AccountsTable` token 列能展示紧凑单位。
- `AccountDetailDrawer` 能展示今日 / 7 日 / 30 日 / 累计完整数字。
- 聚合字段缺失时会正确回退。

### 10.3 视觉与交互验收

- Token 概览抽屉与账号详情抽屉风格一致。
- 主卡 hover、click、drawer open/close 动效自然。
- 表格在桌面宽度下不因新增 token 信息导致布局崩坏。

## 11. 非目标

本轮不做：

- 小时级 usage 曲线图
- 可筛选日期范围
- 官方账单对账
- 单请求 usage 明细列表
- 导出 token 统计报表

## 12. 实施顺序

1. 新增 usage 日聚合表与 upsert 能力。
2. 扩展 `/stats` 返回 summary 与 per-account 周期统计。
3. 扩展前端类型、适配器与格式化函数。
4. 改造顶部 `Token 概览` 卡片。
5. 改造账号池 token 列。
6. 改造账号详情抽屉。
7. 新增 `TokenOverviewDrawer`。
8. 补测试与回归验证。

## 13. 交付标准

完成后应满足：

1. 今日 / 近 7 日 / 近 30 日 / 累计 token 数据可跨重启保留。
2. 主面板默认展示今日 token 概览。
3. 点击 `Token 概览` 可打开细化抽屉。
4. 账号池表格使用 `K / M` 紧凑单位展示 token 用量。
5. 账号详情抽屉展示完整数字和周期统计。
6. 全部新增展示符合当前控制台视觉规范。
