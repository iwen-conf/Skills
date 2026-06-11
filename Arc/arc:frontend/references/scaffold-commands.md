# Scaffold Commands

一键初始化一个符合 `arc:frontend` 基线的 React 19 + TypeScript 前端。

## 1. 新建项目

```bash
pnpm create vite@latest my-app -- --template react-ts
cd my-app
pnpm install
```

## 2. 装基线依赖

```bash
pnpm add react@^19 react-dom@^19 zustand \
  @tanstack/react-query @tanstack/react-router @tanstack/router-devtools \
  react-hook-form @hookform/resolvers zod \
  clsx tailwind-merge class-variance-authority lucide-react

pnpm add -D tailwindcss postcss autoprefixer tailwindcss-animate
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
├── components/ui/     # shadcn 原子组件（由 CLI 生成）
├── hooks/             # 自定义 hooks
├── stores/            # zustand store
├── routes/            # TanStack Router 路由
├── lib/               # utils（cn、fetcher 等）
├── api/               # TanStack Query 查询/变更
├── schemas/           # zod schema
└── styles/globals.css # shadcn CSS 变量 + Tailwind 指令
```

## 6. `src/main.tsx` 最小范式

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  Outlet,
  RouterProvider,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import App from "./App";
import "./styles/globals.css";

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, retry: 1 } },
});

const rootRoute = createRootRoute({
  component: () => <Outlet />,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: App,
});

const routeTree = rootRoute.addChildren([indexRoute]);
const router = createRouter({
  routeTree,
  context: { queryClient },
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
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

## 8. 移动端 iOS/Android 基线

```bash
pnpm create expo-app my-mobile
cd my-mobile
pnpm add nativewind zustand @tanstack/react-query expo-router
pnpm add -D tailwindcss
```

约束：
- 使用 React Native + Expo + TypeScript + NativeWind。
- 路由使用 Expo Router。
- 轻客户端状态使用 Zustand，服务端状态使用 TanStack Query。
- 不默认改用 Flutter、Capacitor、Ionic 或裸 React Native CLI。

## 9. 桌面端 Mac/Windows/Linux 基线

先按 Web 基线创建 React 19 + TypeScript + Vite 项目，再接入 Tauri 2：

```bash
pnpm add -D @tauri-apps/cli
pnpm tauri init
```

约束：
- 桌面壳使用 Tauri 2。
- UI、状态、查询、表单、schema 和 token 优先复用 Web 基线。
- 不默认改用 Electron，也不把桌面端 UI 重写成另一套。

## 10. 多厂家小程序基线

```bash
pnpm dlx @tarojs/cli init my-miniapp
cd my-miniapp
pnpm add zustand
```

`taro init` 交互中选择：
- Framework: `React`
- Language: `TypeScript`
- Version: `Taro 4`

约束：
- 多厂家小程序默认使用 Taro 4 + React + TypeScript + Zustand。
- 微信原生单端小程序仍走 `wxskills`，不要为了单端任务强行套 Taro。
