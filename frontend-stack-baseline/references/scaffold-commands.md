# Scaffold Commands

一键初始化一个符合 `frontend-stack-baseline` 的 React 前端。

## 1. 新建项目

```bash
pnpm create vite@latest my-app -- --template react-ts
cd my-app
pnpm install
```

## 2. 装基线依赖

```bash
pnpm add react-router-dom zustand @tanstack/react-query \
  react-hook-form @hookform/resolvers zod framer-motion \
  clsx tailwind-merge class-variance-authority lucide-react

pnpm add -D tailwindcss postcss autoprefixer tailwindcss-animate \
  @types/node
```

## 3. Tailwind + shadcn 初始化

```bash
pnpm dlx tailwindcss init -p
pnpm dlx shadcn@latest init
```

`shadcn init` 交互中：
- Style: `Default` 或 `New York`（管理台推荐 `New York`）
- Base color: 选 `Slate` / `Stone` / `Zinc` / `Neutral` 中与当前 palette 最接近的一项占位，稍后用 [color-tokens.md](./color-tokens.md) 的变量整体覆盖 `globals.css`。
- CSS variables: **yes**（强制，便于后续换 palette）

## 4. 按需补 shadcn 组件

```bash
pnpm dlx shadcn@latest add button card input label form dialog \
  dropdown-menu select table tabs toast tooltip sheet avatar badge
```

管理台额外补：

```bash
pnpm dlx shadcn@latest add data-table pagination breadcrumb \
  command popover calendar date-picker
```

## 5. 目录骨架

```text
src/
├── components/        # 业务组件
├── ui/                # shadcn 原子组件（由 CLI 生成，默认就在 src/components/ui）
├── hooks/             # 自定义 hooks
├── stores/            # zustand store
├── routes/            # React Router 路由
├── lib/               # utils（cn、fetcher 等）
├── api/               # TanStack Query 查询/变更
├── schemas/           # zod schema
└── styles/globals.css # shadcn CSS 变量 + Tailwind 指令
```

## 6. `src/main.tsx` 最小范式

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import App from "./App";
import "./styles/globals.css";

const qc = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, retry: 1 } },
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>
);
```

## 7. 验证

```bash
pnpm dev
```

打开浏览器，确认：
- 背景、卡片、主按钮颜色与所选 palette 一致；
- 切换 `<html class="dark">` 能进入 `.dark` 变体（若配置了双主题）；
- `pnpm build` 无错。
