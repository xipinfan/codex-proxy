# Console Frontend Modernization Design

**日期：** 2026-04-21  
**状态：** 已确认，可进入实现  
**视觉稿基线：** `zTCIL`（主面板）、`yYT3I`（账号详情抽屉）、`veVlb`（设置弹窗）、`SQXKL`（hover）、`zmRd9`（empty）、`S6NZp`（error）、`EPvAS`（Codex OAuth 导入）

## 1. 目标

将当前基于单文件 HTML 的管理后台升级为现代化前端控制台，覆盖以下目标：

1. 提供可上线的数据统计与账号展示后台。
2. 与现有 Go 服务直接集成，不改变后端对外接口协议。
3. 保留当前服务的静态资源嵌入模式，但将前端研发迁移到 `Vite + React + TypeScript + Tailwind CSS`。
4. 将已在 pencli 中确认的视觉稿落为真实可交互页面，重点加强账号额度、状态反馈、空态/异常态与导入体验。
5. 新增“Codex（OpenAI OAuth）导入”能力，采用“打开授权页 -> 浏览器跳回 localhost -> 粘贴完整 callback URL -> 前端解析并提交导入”的交互。

## 2. 当前项目现状

### 2.1 服务端现状

- 后端为 Go 服务，首页静态文件由 [embed.go](/E:/work/codex-proxy/internal/static/embed.go) 内嵌并由 [index.go](/E:/work/codex-proxy/internal/handler/index.go) 返回。
- 现有管理接口已经可支撑第一版控制台：
  - `GET /stats`：获取账号汇总与账号列表。
  - `POST /refresh`：SSE 刷新全部账号。
  - `POST /check-quota`：SSE 查询额度。
  - `POST /recover-auth`：触发 401 恢复。
  - `POST /admin/accounts/ingest`：导入账号。
  - `GET/POST /admin/accounts/ingest` + WebSocket：分批导入账号。
- OAuth 刷新逻辑已存在于 [refresh.go](/E:/work/codex-proxy/internal/auth/refresh.go)，说明服务端对 `refresh_token` 账户模型是成熟的。

### 2.2 前端现状

- 当前后台页面仅为单一 HTML 文件 [index.html](/E:/work/codex-proxy/internal/static/assets/index.html)，无工程化构建、无组件边界、无类型约束。
- 页面仅做基础统计展示，无法承载复杂状态、设置管理、导入流程和现代交互细节。

## 3. 用户体验定位

新后台定位为“Codex 账号池运营控制台”，强调三件事：

1. **可观测性**：一眼看清账号池健康度、总量、请求量、token 使用量、异常分布。
2. **可操作性**：从主表即可完成筛选、刷新、额度校验、查看详情、导入账号、调整连接设置。
3. **可恢复性**：对于空数据、接口失败、导入失败、OAuth 回调粘贴错误等情况给出清晰反馈与明确下一步。

## 4. 信息架构

### 4.1 页面层级

后台先维持单页面控制台，但内部拆分为五个交互层：

1. **顶部导航区**
   - 品牌标题、副标题、环境状态。
   - 主操作：刷新数据、检查额度、导入账号、打开设置。

2. **总览指标区**
   - 总账号数。
   - Active / Cooldown / Disabled 分布。
   - RPM。
   - Total Input Tokens / Output Tokens。
   - 可扩展卡片：异常占比、最近同步时间。

3. **账号表格区**
   - 搜索、筛选、分页、刷新状态。
   - 行 hover / selected 态。
   - 每行支持查看详情。

4. **账号详情抽屉**
   - 账号基础信息。
   - 配额与使用量细节。
   - 刷新时间、过期时间、禁用原因。
   - 预留更多面板位：授权信息、额度窗口、失败记录、下一步建议。

5. **模态层**
   - 设置弹窗。
   - Codex OAuth 导入弹窗。
   - 可能的二次确认 / 错误提示。

### 4.2 状态页要求

主页面必须内建四类状态：

1. **Hover / Selected**：突出当前行与其联动详情。
2. **Empty**：零账号时展示引导文案与导入入口。
3. **Error**：接口失败时展示错误摘要、重试动作、诊断建议。
4. **Loading**：指标骨架屏、表格骨架、按钮 loading。

## 5. 视觉系统

### 5.1 风格方向

采用已确认的 warm-tech 方向：

- 明亮暖色背景，但避免重复且廉价的渐变噪声。
- 用大面积柔和色块、光晕和低对比层次制造空间感。
- 卡片强调玻璃感/半透明叠层，而不是厚重拟物。
- 关键色保持橙金 + 湖蓝的双强调体系，用于 CTA、状态和图标高光。

### 5.2 视觉约束

- 背景必须是大尺度、非重复感的渐变与光斑组合，不能出现明显 tiled 纹理感。
- 数据卡、表格、抽屉、弹窗四类容器必须保持同一圆角、描边和阴影语言。
- 抽屉与主表视觉上要属于同一产品层级，不能像另一套产品。
- 额度面板要更“运营看板化”，通过刻度、标签、剩余额度、周期重置提示体现专业性。

## 6. 功能设计

### 6.1 数据统计与列表展示

**数据来源：** `GET /stats`

**前端行为：**

- 默认使用分页模式请求：`/stats?page=1&page_size=20&include_quota=true`。
- 搜索时透传 `q`。
- 在总览卡片中展示 `summary`。
- 在表格中展示 `accounts`。
- 若后端未返回 quota，则前端以安全方式降级展示 “No quota data”。

**表格字段建议：**

- 邮箱
- 状态
- 套餐类型
- 请求数
- 错误数
- Token 用量
- 最近使用时间
- 最近刷新时间
- 额度状态 / reset 时间

### 6.2 账号详情抽屉

抽屉从选中行打开，展示：

- 账号邮箱、套餐、状态、文件标识。
- 输入 / 输出 / 总 token。
- 总补全次数。
- token 过期时间。
- last refreshed / last used。
- disable reason / consecutive failures。
- quota summary，包括：
  - used percent
  - exhausted 状态
  - resets at
  - 原始 quota 缺失时的空态文案

详情区允许后续继续扩展，但第一版先做好信息组织和视觉层次。

### 6.3 设置弹窗

设置项先做前端侧连接配置，不先改动 Go 配置文件：

- API Base URL
- Admin API Key
- 默认页大小
- 自动刷新间隔（先支持前端轮询）
- 是否默认请求 `include_quota`

设置存储在 `localStorage`，用于浏览器端会话持久化。

### 6.4 Codex OAuth 导入

**体验流程：**

1. 用户点击“导入 Codex 账号”。
2. 弹窗展示三步说明：
   - 打开授权链接。
   - 完成浏览器登录，等待跳回 localhost。
   - 复制完整 callback URL 粘贴回来。
3. 前端提供 callback URL 文本域。
4. 用户点击“解析并导入”。
5. 前端从 URL 中提取授权信息，生成符合导入接口的 payload。
6. 提交到 `POST /admin/accounts/ingest`。
7. 成功后刷新列表并展示成功提示；失败时在弹窗内给出错误信息。

**设计边界：**

- 第一版优先支持“用户粘贴完整回调 URL”的手工流程。
- 不在这一版里直接监听本地端口，也不在前端直接完成完整 OAuth code exchange。
- 如果 callback URL 解析不出可用凭据，则明确提示用户回调内容无效。
- 当前仓库没有 `newapi` 代码可参考，因此实现以现有仓库可确认的导入协议为准，保持扩展位，后续可替换为更接近目标站点的解析逻辑。

### 6.5 运维动作反馈

以下动作需要统一 action feedback 机制：

- 刷新全部账号
- 检查额度
- 导入账号
- 保存设置
- 重试 stats 请求

反馈形式：

- 页内 toast / action banner。
- 按钮 loading。
- 必要时在详情抽屉或弹窗里显示更细的错误说明。

## 7. 技术方案

### 7.1 前端栈

采用：

- `Vite`
- `React`
- `TypeScript`
- `Tailwind CSS`

原因：

- 快速搭建与热更新效率高。
- 组件拆分、状态组织、类型安全都比纯 HTML 更适合这个后台长期迭代。
- Tailwind 适合将 pencli 的视觉语言沉淀为 token + utility 的组合，而不是继续维护巨大的内联样式块。

### 7.2 目录结构

建议新增 `web/` 目录作为独立前端工程：

- `web/src/app`：应用壳与路由入口。
- `web/src/features/dashboard`：总览和表格。
- `web/src/features/account-detail`：详情抽屉。
- `web/src/features/settings`：设置弹窗。
- `web/src/features/oauth-import`：Codex OAuth 导入弹窗与解析逻辑。
- `web/src/components/ui`：Button / Card / Badge / Modal / Drawer / EmptyState / ErrorState 等基础组件。
- `web/src/lib`：API client、格式化函数、storage、schema parser。
- `web/src/styles`：Tailwind 主题层与全局样式。

### 7.3 与 Go 集成方式

- 前端在 `web/` 内开发与构建。
- 生产构建输出到 `internal/static/assets/`。
- Go 端保持 embed 静态资源服务方式不变，只调整嵌入内容以兼容 Vite 构建后的资源结构。
- 本地开发可允许 Vite 独立 dev server，请求代理到 Go 服务。

## 8. 数据与状态管理

第一版使用 React 原生能力与轻量封装，不额外引入重型状态库。

- 页面级状态：筛选、分页、选中账号、弹窗开关。
- 远程数据：`stats` 请求与动作请求状态。
- 本地持久化：设置项存于 `localStorage`。
- 动作事件流：对 SSE 先做基础支持入口，第一版至少保证普通 POST 动作和 ingest 流程完整可用。

## 9. 错误处理

必须覆盖以下错误：

1. API Base URL 缺失或格式错误。
2. API Key 缺失导致 401。
3. `/stats` 请求失败。
4. 导入接口返回解析失败或字段校验失败。
5. callback URL 粘贴内容非法。
6. quota 缺失或部分字段不完整。

每类错误都要映射为用户能理解的文案，而不是直接暴露原始异常对象。

## 10. 测试策略

实现阶段采用分层验证：

1. **单元测试**
   - callback URL 解析。
   - `/stats` 数据适配。
   - localStorage 设置读写。
   - 时间与数值格式化。

2. **组件测试**
   - empty / error / selected / modal 状态。
   - 账号详情面板渲染。

3. **集成验证**
   - 前端构建产物被 Go 正确嵌入。
   - 本地访问 `/` 能看到新后台。
   - 使用 mock 或真实本地服务完成一次 ingest 请求。

## 11. 非目标

本轮不做：

- 多页面路由化后台。
- 完整 OAuth 自动回调监听服务。
- 复杂 RBAC 权限系统。
- 服务端新增大量后台接口。
- 大屏图表化监控系统。

## 12. 交付标准

上线前应满足：

1. 新首页完全替换旧静态 HTML 风格。
2. 视觉效果与 pencli 主稿基本一致，并包含 hover/empty/error/OAuth 导入状态稿。
3. 可配置 API 地址与密钥并拉取数据。
4. 可查看账号详情抽屉。
5. 可通过导入弹窗完成至少一种有效导入流程。
6. Go 构建成功，静态资源嵌入成功。
7. 基础测试通过。
