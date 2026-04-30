# Console Frontend Modernization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 Vite + React + TypeScript + Tailwind 重建管理后台，并接入现有 Go 管理接口、账号详情与 Codex OAuth 导入体验。

**Architecture:** 新增独立 `web/` 前端工程作为控制台源码来源，构建产物输出到 Go 的静态资源目录，由现有 embed 机制提供上线静态页。前端以单页 dashboard 为主，通过模块化组件拆分主表、详情抽屉、设置弹窗与 OAuth 导入弹窗，并用轻量 API client 封装 `/stats`、`/admin/accounts/ingest` 等接口。

**Tech Stack:** Vite, React, TypeScript, Tailwind CSS, Vitest, Testing Library, Go embed static assets

---

## File Structure

### New files/directories

- Create: `E:/work/codex-proxy/web/package.json`
- Create: `E:/work/codex-proxy/web/tsconfig.json`
- Create: `E:/work/codex-proxy/web/tsconfig.node.json`
- Create: `E:/work/codex-proxy/web/vite.config.ts`
- Create: `E:/work/codex-proxy/web/postcss.config.js`
- Create: `E:/work/codex-proxy/web/tailwind.config.ts`
- Create: `E:/work/codex-proxy/web/index.html`
- Create: `E:/work/codex-proxy/web/src/main.tsx`
- Create: `E:/work/codex-proxy/web/src/app/App.tsx`
- Create: `E:/work/codex-proxy/web/src/styles/index.css`
- Create: `E:/work/codex-proxy/web/src/lib/api.ts`
- Create: `E:/work/codex-proxy/web/src/lib/storage.ts`
- Create: `E:/work/codex-proxy/web/src/lib/format.ts`
- Create: `E:/work/codex-proxy/web/src/lib/oauth.ts`
- Create: `E:/work/codex-proxy/web/src/lib/types.ts`
- Create: `E:/work/codex-proxy/web/src/lib/stats.ts`
- Create: `E:/work/codex-proxy/web/src/components/ui/*.tsx`
- Create: `E:/work/codex-proxy/web/src/features/dashboard/*.tsx`
- Create: `E:/work/codex-proxy/web/src/features/account-detail/*.tsx`
- Create: `E:/work/codex-proxy/web/src/features/settings/*.tsx`
- Create: `E:/work/codex-proxy/web/src/features/oauth-import/*.tsx`
- Create: `E:/work/codex-proxy/web/src/test/*.test.ts(x)`

### Modified files

- Modify: `E:/work/codex-proxy/internal/static/embed.go`
- Modify: `E:/work/codex-proxy/internal/handler/index.go` (only if Vite asset routing requires fallback refinement)
- Modify: `E:/work/codex-proxy/.gitignore`
- Modify: `E:/work/codex-proxy/README.md` or `E:/work/codex-proxy/docs/DEPLOY.md` with frontend build instructions if needed

---

### Task 1: 搭建前端工程骨架与构建链路

**Files:**
- Create: `E:/work/codex-proxy/web/package.json`
- Create: `E:/work/codex-proxy/web/tsconfig.json`
- Create: `E:/work/codex-proxy/web/tsconfig.node.json`
- Create: `E:/work/codex-proxy/web/vite.config.ts`
- Create: `E:/work/codex-proxy/web/postcss.config.js`
- Create: `E:/work/codex-proxy/web/tailwind.config.ts`
- Create: `E:/work/codex-proxy/web/index.html`
- Create: `E:/work/codex-proxy/web/src/main.tsx`
- Create: `E:/work/codex-proxy/web/src/app/App.tsx`
- Create: `E:/work/codex-proxy/web/src/styles/index.css`
- Modify: `E:/work/codex-proxy/internal/static/embed.go`

- [ ] **Step 1: 写出前端工程依赖与脚本定义**

```json
{
  "name": "codex-proxy-console",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest"
  },
  "dependencies": {
    "react": "^19.1.0",
    "react-dom": "^19.1.0"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "^6.6.3",
    "@testing-library/react": "^16.2.0",
    "@testing-library/user-event": "^14.6.1",
    "@types/react": "^19.1.2",
    "@types/react-dom": "^19.1.2",
    "@vitejs/plugin-react": "^4.4.1",
    "autoprefixer": "^10.4.20",
    "postcss": "^8.5.3",
    "tailwindcss": "^3.4.17",
    "typescript": "^5.8.3",
    "vite": "^6.3.2",
    "vitest": "^3.1.2",
    "jsdom": "^26.0.0"
  }
}
```

- [ ] **Step 2: 配置 Vite 输出到 Go 静态目录并代理本地 API**

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';

export default defineConfig({
  plugins: [react()],
  root: path.resolve(__dirname),
  build: {
    outDir: path.resolve(__dirname, '../internal/static/assets'),
    emptyOutDir: true,
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/stats': 'http://127.0.0.1:8080',
      '/refresh': 'http://127.0.0.1:8080',
      '/check-quota': 'http://127.0.0.1:8080',
      '/recover-auth': 'http://127.0.0.1:8080',
      '/admin': 'http://127.0.0.1:8080'
    }
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts'
  }
});
```

- [ ] **Step 3: 初始化 React 入口与 Tailwind 全局样式骨架**

```tsx
import React from 'react';
import ReactDOM from 'react-dom/client';
import { App } from './app/App';
import './styles/index.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  color-scheme: light;
  --bg-base: #f7efe3;
  --bg-surface: rgba(255, 251, 245, 0.78);
  --bg-surface-strong: rgba(255, 250, 245, 0.94);
  --border-soft: rgba(122, 91, 62, 0.14);
  --text-primary: #1f1a17;
  --text-secondary: #6d5a4b;
  --accent-amber: #ff9a3d;
  --accent-cyan: #3bb8c5;
}

body {
  min-height: 100vh;
  margin: 0;
  background:
    radial-gradient(circle at top left, rgba(255, 194, 121, 0.38), transparent 34%),
    radial-gradient(circle at top right, rgba(110, 217, 229, 0.3), transparent 28%),
    linear-gradient(180deg, #fcf6ed 0%, #f2e8da 100%);
  color: var(--text-primary);
}
```

- [ ] **Step 4: 调整 Go embed 以兼容 Vite 产物**

```go
//go:embed assets assets/*
var Assets embed.FS
```

- [ ] **Step 5: 安装依赖并验证前端可以构建**

Run: `corepack pnpm --dir web install`
Expected: install 成功，无 lockfile 冲突致命错误

Run: `corepack pnpm --dir web build`
Expected: 生成 `E:/work/codex-proxy/internal/static/assets/index.html` 与 `assets/*` 资源文件

- [ ] **Step 6: Commit**

```bash
git add web internal/static/embed.go .gitignore
git commit -m "feat: scaffold console frontend"
```

### Task 2: 建立数据模型、设置存储与 API Client

**Files:**
- Create: `E:/work/codex-proxy/web/src/lib/types.ts`
- Create: `E:/work/codex-proxy/web/src/lib/api.ts`
- Create: `E:/work/codex-proxy/web/src/lib/storage.ts`
- Create: `E:/work/codex-proxy/web/src/lib/format.ts`
- Create: `E:/work/codex-proxy/web/src/lib/stats.ts`
- Create: `E:/work/codex-proxy/web/src/test/storage.test.ts`
- Create: `E:/work/codex-proxy/web/src/test/stats.test.ts`

- [ ] **Step 1: 先写设置存储与 stats 适配器测试**

```ts
import { describe, expect, it } from 'vitest';
import { loadConsoleSettings, saveConsoleSettings } from '../lib/storage';
import { adaptStatsResponse } from '../lib/stats';

describe('console settings storage', () => {
  it('loads defaults when localStorage is empty', () => {
    expect(loadConsoleSettings()).toMatchObject({
      baseUrl: '',
      apiKey: '',
      pageSize: 20,
      autoRefreshSeconds: 0,
      includeQuota: true,
    });
  });

  it('round-trips persisted settings', () => {
    saveConsoleSettings({
      baseUrl: 'http://127.0.0.1:8080',
      apiKey: 'sk-test',
      pageSize: 50,
      autoRefreshSeconds: 30,
      includeQuota: false,
    });

    expect(loadConsoleSettings()).toMatchObject({
      baseUrl: 'http://127.0.0.1:8080',
      apiKey: 'sk-test',
      pageSize: 50,
      autoRefreshSeconds: 30,
      includeQuota: false,
    });
  });
});

describe('adaptStatsResponse', () => {
  it('maps summary and accounts safely', () => {
    const adapted = adaptStatsResponse({
      summary: { total: 3, active: 2, cooldown: 1, disabled: 0, rpm: 5 },
      accounts: [{ email: 'a@example.com', status: 'active', usage: { total_tokens: 12 } }],
      pagination: { page: 1, page_size: 20, filtered_total: 1, total_pages: 1 },
    });

    expect(adapted.summary.total).toBe(3);
    expect(adapted.accounts[0].email).toBe('a@example.com');
    expect(adapted.accounts[0].usage.totalTokens).toBe(12);
  });
});
```

- [ ] **Step 2: 定义前端领域类型与默认设置**

```ts
export type AccountStatus = 'active' | 'cooldown' | 'disabled';

export interface ConsoleSettings {
  baseUrl: string;
  apiKey: string;
  pageSize: number;
  autoRefreshSeconds: number;
  includeQuota: boolean;
}

export const defaultSettings: ConsoleSettings = {
  baseUrl: '',
  apiKey: '',
  pageSize: 20,
  autoRefreshSeconds: 0,
  includeQuota: true,
};
```

- [ ] **Step 3: 实现 localStorage 读写与 stats response 适配器**

```ts
const STORAGE_KEY = 'codex-proxy-console-settings';

export function loadConsoleSettings(): ConsoleSettings {
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) return defaultSettings;
  try {
    return { ...defaultSettings, ...JSON.parse(raw) };
  } catch {
    return defaultSettings;
  }
}

export function saveConsoleSettings(value: ConsoleSettings): void {
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(value));
}
```

- [ ] **Step 4: 实现 API client，统一 Header、错误解析与 URL 拼接**

```ts
export async function fetchStats(settings: ConsoleSettings, params: StatsQuery): Promise<StatsResponse> {
  const url = new URL('/stats', resolveBaseUrl(settings.baseUrl));
  url.searchParams.set('page', String(params.page));
  url.searchParams.set('page_size', String(params.pageSize));
  url.searchParams.set('include_quota', String(params.includeQuota));
  if (params.query) url.searchParams.set('q', params.query);

  const response = await fetch(url, {
    headers: buildHeaders(settings),
  });

  if (!response.ok) {
    throw await toApiError(response);
  }

  return response.json() as Promise<StatsResponse>;
}
```

- [ ] **Step 5: 运行单元测试验证存储和适配器**

Run: `corepack pnpm --dir web test -- src/test/storage.test.ts src/test/stats.test.ts`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/lib web/src/test
git commit -m "feat: add console client foundations"
```

### Task 3: 实现主 Dashboard、空态/错误态/选择态

**Files:**
- Create: `E:/work/codex-proxy/web/src/components/ui/Button.tsx`
- Create: `E:/work/codex-proxy/web/src/components/ui/Card.tsx`
- Create: `E:/work/codex-proxy/web/src/components/ui/Badge.tsx`
- Create: `E:/work/codex-proxy/web/src/components/ui/EmptyState.tsx`
- Create: `E:/work/codex-proxy/web/src/components/ui/ErrorState.tsx`
- Create: `E:/work/codex-proxy/web/src/features/dashboard/DashboardPage.tsx`
- Create: `E:/work/codex-proxy/web/src/features/dashboard/StatsOverview.tsx`
- Create: `E:/work/codex-proxy/web/src/features/dashboard/AccountsTable.tsx`
- Create: `E:/work/codex-proxy/web/src/test/dashboard.test.tsx`
- Modify: `E:/work/codex-proxy/web/src/app/App.tsx`

- [ ] **Step 1: 先写 dashboard 状态渲染测试**

```tsx
it('renders empty state when there are no accounts', () => {
  render(<DashboardPage state="empty" />);
  expect(screen.getByText(/导入你的第一个 Codex 账号/i)).toBeInTheDocument();
});

it('renders error state when stats request fails', () => {
  render(<DashboardPage state="error" errorMessage="401 Unauthorized" />);
  expect(screen.getByText(/401 Unauthorized/i)).toBeInTheDocument();
});

it('highlights selected row', () => {
  render(<AccountsTable accounts={[sampleAccount]} selectedAccountId="a@example.com" onSelect={() => {}} />);
  expect(screen.getByRole('row', { name: /a@example.com/i })).toHaveAttribute('data-selected', 'true');
});
```

- [ ] **Step 2: 实现应用壳和 warm-tech 主页面布局**

```tsx
export function App() {
  return <DashboardPage />;
}
```

```tsx
export function DashboardPage() {
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-[1480px] flex-col gap-6 px-6 py-8 lg:px-10">
      <header className="flex flex-col gap-4 rounded-[32px] border border-white/50 bg-[var(--bg-surface)] p-6 shadow-[0_24px_80px_rgba(99,66,31,0.12)] backdrop-blur-xl lg:flex-row lg:items-center lg:justify-between">
        {/* title + actions */}
      </header>
      {/* overview + table + empty/error */}
    </main>
  );
}
```

- [ ] **Step 3: 实现总览卡片、账号表格与 hover/selected 样式**

```tsx
<tr
  data-selected={isSelected ? 'true' : 'false'}
  className={cn(
    'group cursor-pointer border-b border-[rgba(122,91,62,0.08)] transition',
    isSelected ? 'bg-[rgba(255,154,61,0.12)] shadow-[inset_0_0_0_1px_rgba(255,154,61,0.24)]' : 'hover:bg-white/70',
  )}
>
```

- [ ] **Step 4: 接入真实 `fetchStats` 请求与刷新、搜索、分页状态**

```tsx
useEffect(() => {
  void loadStats({ page, pageSize: settings.pageSize, query, includeQuota: settings.includeQuota });
}, [page, query, settings.pageSize, settings.includeQuota]);
```

- [ ] **Step 5: 运行 dashboard 组件测试并人工截图核对布局**

Run: `corepack pnpm --dir web test -- src/test/dashboard.test.tsx`
Expected: PASS

Run: `corepack pnpm --dir web build`
Expected: PASS，可在本地打开新页面进行视觉检查

- [ ] **Step 6: Commit**

```bash
git add web/src/app web/src/components web/src/features/dashboard
git commit -m "feat: build console dashboard shell"
```

### Task 4: 实现账号详情抽屉与额度面板

**Files:**
- Create: `E:/work/codex-proxy/web/src/components/ui/Drawer.tsx`
- Create: `E:/work/codex-proxy/web/src/features/account-detail/AccountDetailDrawer.tsx`
- Create: `E:/work/codex-proxy/web/src/features/account-detail/QuotaPanel.tsx`
- Create: `E:/work/codex-proxy/web/src/test/account-detail.test.tsx`
- Modify: `E:/work/codex-proxy/web/src/features/dashboard/DashboardPage.tsx`

- [ ] **Step 1: 先写详情抽屉和 quota 面板测试**

```tsx
it('renders quota reset and usage values', () => {
  render(<AccountDetailDrawer account={sampleAccount} open onClose={() => {}} />);
  expect(screen.getByText(/Quota window/i)).toBeInTheDocument();
  expect(screen.getByText(/Reset/i)).toBeInTheDocument();
});

it('renders fallback when quota data is missing', () => {
  render(<AccountDetailDrawer account={{ ...sampleAccount, quota: null }} open onClose={() => {}} />);
  expect(screen.getByText(/No quota data/i)).toBeInTheDocument();
});
```

- [ ] **Step 2: 实现与视觉稿一致的右侧抽屉框架**

```tsx
<aside className={cn(
  'fixed inset-y-4 right-4 z-40 w-[min(460px,calc(100vw-24px))] rounded-[30px] border border-white/55 bg-[rgba(255,251,245,0.92)] p-5 shadow-[0_28px_80px_rgba(74,51,29,0.22)] backdrop-blur-2xl transition duration-300',
  open ? 'translate-x-0 opacity-100' : 'translate-x-8 opacity-0 pointer-events-none',
)}>
```

- [ ] **Step 3: 将账号基础信息、usage、quota、健康提示拆成子卡片**

```tsx
<section className="rounded-[24px] border border-[var(--border-soft)] bg-white/70 p-4">
  <p className="text-xs uppercase tracking-[0.24em] text-[var(--text-secondary)]">Quota window</p>
  <div className="mt-3 h-3 overflow-hidden rounded-full bg-[rgba(59,184,197,0.12)]">
    <div className="h-full rounded-full bg-gradient-to-r from-[#3bb8c5] to-[#ff9a3d]" style={{ width: `${usedPercent}%` }} />
  </div>
</section>
```

- [ ] **Step 4: 在 dashboard 中连接选中账号与抽屉开关**

```tsx
const [selectedAccount, setSelectedAccount] = useState<AccountView | null>(null);
```

- [ ] **Step 5: 运行详情测试并手动核对主表与抽屉适配性**

Run: `corepack pnpm --dir web test -- src/test/account-detail.test.tsx`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/features/account-detail web/src/components/ui/Drawer.tsx web/src/features/dashboard/DashboardPage.tsx
git commit -m "feat: add account detail drawer"
```

### Task 5: 实现设置弹窗与页面配置持久化

**Files:**
- Create: `E:/work/codex-proxy/web/src/components/ui/Modal.tsx`
- Create: `E:/work/codex-proxy/web/src/features/settings/SettingsDialog.tsx`
- Create: `E:/work/codex-proxy/web/src/test/settings-dialog.test.tsx`
- Modify: `E:/work/codex-proxy/web/src/features/dashboard/DashboardPage.tsx`

- [ ] **Step 1: 先写设置弹窗测试**

```tsx
it('saves settings and closes dialog', async () => {
  render(<SettingsDialog open initialValue={defaultSettings} onSave={onSave} onClose={onClose} />);
  await user.type(screen.getByLabelText(/API Base URL/i), 'http://127.0.0.1:8080');
  await user.click(screen.getByRole('button', { name: /保存设置/i }));
  expect(onSave).toHaveBeenCalledWith(expect.objectContaining({ baseUrl: 'http://127.0.0.1:8080' }));
});
```

- [ ] **Step 2: 实现与 `veVlb` 视觉稿匹配的设置弹窗布局**

```tsx
<form className="grid gap-4">
  <label className="grid gap-2 text-sm font-medium text-[var(--text-secondary)]">
    <span>API Base URL</span>
    <input className="h-12 rounded-2xl border border-[var(--border-soft)] bg-white/80 px-4" />
  </label>
</form>
```

- [ ] **Step 3: 连接 localStorage 设置、重新拉取 stats 与默认分页逻辑**

```tsx
const handleSaveSettings = (next: ConsoleSettings) => {
  saveConsoleSettings(next);
  setSettings(next);
  setPage(1);
  void loadStats({ page: 1, pageSize: next.pageSize, query, includeQuota: next.includeQuota });
};
```

- [ ] **Step 4: 运行设置测试**

Run: `corepack pnpm --dir web test -- src/test/settings-dialog.test.tsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/features/settings web/src/components/ui/Modal.tsx web/src/features/dashboard/DashboardPage.tsx
git commit -m "feat: add console settings dialog"
```

### Task 6: 实现 Codex OAuth 导入弹窗与回调解析

**Files:**
- Create: `E:/work/codex-proxy/web/src/lib/oauth.ts`
- Create: `E:/work/codex-proxy/web/src/features/oauth-import/OAuthImportDialog.tsx`
- Create: `E:/work/codex-proxy/web/src/test/oauth.test.ts`
- Create: `E:/work/codex-proxy/web/src/test/oauth-import-dialog.test.tsx`
- Modify: `E:/work/codex-proxy/web/src/lib/api.ts`
- Modify: `E:/work/codex-proxy/web/src/features/dashboard/DashboardPage.tsx`

- [ ] **Step 1: 先写 callback URL 解析与导入测试**

```ts
it('extracts tokens from callback url hash', () => {
  const parsed = parseOAuthCallbackUrl('http://127.0.0.1:1455/callback#access_token=at&id_token=it&refresh_token=rt');
  expect(parsed.refreshToken).toBe('rt');
  expect(parsed.idToken).toBe('it');
});

it('throws when refresh token is missing', () => {
  expect(() => parseOAuthCallbackUrl('http://127.0.0.1:1455/callback#access_token=at')).toThrow(/refresh_token/i);
});
```

- [ ] **Step 2: 实现最小可用 OAuth callback 解析器**

```ts
export function parseOAuthCallbackUrl(input: string): OAuthCallbackPayload {
  const url = new URL(input.trim());
  const hash = url.hash.startsWith('#') ? url.hash.slice(1) : url.hash;
  const params = new URLSearchParams(hash || url.search.slice(1));
  const refreshToken = params.get('refresh_token') ?? params.get('rk') ?? '';
  if (!refreshToken) {
    throw new Error('回调 URL 中缺少 refresh_token');
  }
  return {
    refreshToken,
    accessToken: params.get('access_token') ?? '',
    idToken: params.get('id_token') ?? '',
  };
}
```

- [ ] **Step 3: 将解析结果映射为 ingest payload 并调用 `/admin/accounts/ingest`**

```ts
export async function ingestAccountFromOAuth(settings: ConsoleSettings, callbackUrl: string) {
  const parsed = parseOAuthCallbackUrl(callbackUrl);
  return ingestAccounts(settings, [{
    refresh_token: parsed.refreshToken,
    access_token: parsed.accessToken,
    id_token: parsed.idToken,
    type: 'codex',
  }]);
}
```

- [ ] **Step 4: 实现 `EPvAS` 对应的三步导入弹窗、成功提示与失败文案**

```tsx
<ol className="grid gap-3 rounded-[24px] border border-[var(--border-soft)] bg-white/70 p-4">
  <li>1. 打开 OpenAI 授权页并完成登录。</li>
  <li>2. 浏览器跳回 localhost 后，复制完整回调 URL。</li>
  <li>3. 粘贴到下方，解析并导入到当前账号池。</li>
</ol>
```

- [ ] **Step 5: 运行 OAuth 测试与整体构建**

Run: `corepack pnpm --dir web test -- src/test/oauth.test.ts src/test/oauth-import-dialog.test.tsx`
Expected: PASS

Run: `corepack pnpm --dir web build`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/oauth.ts web/src/features/oauth-import web/src/lib/api.ts web/src/test
git commit -m "feat: add codex oauth import flow"
```

### Task 7: 接入 Go 静态服务、完善文档并做端到端验证

**Files:**
- Modify: `E:/work/codex-proxy/internal/static/embed.go`
- Modify: `E:/work/codex-proxy/internal/handler/index.go` (如需要)
- Modify: `E:/work/codex-proxy/docs/DEPLOY.md`
- Modify: `E:/work/codex-proxy/README.md` (如需要)

- [ ] **Step 1: 写出前端构建产物嵌入验证步骤**

```bash
corepack pnpm --dir web build
go test ./...
go build ./...
```

Expected: 前端构建通过，Go 测试通过，Go 可执行文件重新生成成功

- [ ] **Step 2: 如有必要，调整 index handler 的 SPA fallback**

```go
func (h *ProxyHandler) handleIndex(ctx *fasthttp.RequestCtx) {
    ctx.SetContentType("text/html; charset=utf-8")
    ctx.SetStatusCode(fasthttp.StatusOK)
    ctx.SetBody(h.indexHTML)
}
```

- [ ] **Step 3: 补充 README 或 DEPLOY 文档中的前端开发/构建说明**

```md
### Console Frontend Development

```bash
corepack pnpm --dir web install
corepack pnpm --dir web dev
```

### Build Embedded Assets

```bash
corepack pnpm --dir web build
go build ./...
```
```

- [ ] **Step 4: 手动验证最终页面**

Run: `corepack pnpm --dir web build`
Expected: PASS

Run: `go build ./...`
Expected: PASS

Run: `./codex-proxy.exe -c config.yaml`
Expected: 服务启动成功，可通过 `/` 访问新后台

- [ ] **Step 5: Commit**

```bash
git add internal/static/embed.go internal/handler/index.go docs/DEPLOY.md README.md internal/static/assets
git commit -m "feat: ship embedded console frontend"
```

## Self-Review

- Spec coverage:
  - 现代化前端栈：Task 1
  - 统计与展示：Task 2-3
  - 账号详情与额度面板：Task 4
  - 设置弹窗：Task 5
  - Codex OAuth 导入：Task 6
  - Go 集成与上线：Task 7
- Placeholder scan:
  - 已去除 TBD/TODO；所有任务都给出文件、命令和关键代码骨架。
- Type consistency:
  - 使用 `ConsoleSettings`、`StatsResponse`、`AccountView`、`parseOAuthCallbackUrl` 作为统一命名，后续实现需保持一致。
