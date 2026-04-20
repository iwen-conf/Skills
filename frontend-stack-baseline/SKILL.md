---
name: frontend-stack-baseline
description: 通用前端基线声明：任何需要搭建 Web 前端（新项目、脚手架、后台管理、B 端工作台、Landing、Dashboard、管理台、控制台）时的默认技术栈、默认配色与默认 token。固定栈为 React + Vite + Tailwind CSS + shadcn/ui + Zustand + TanStack Query + React Router + React Hook Form + Zod + Framer Motion；默认配色从五套低饱和度方案中选一套，并按 shadcn/ui CSS 变量落地。与领域 skill（如 lazycat:create-app / lazycat:admin-ui）组合使用，由本 skill 专管“栈 + 配色”这一层，避免各 skill 内重复定义。
---

# Frontend Stack Baseline

`frontend-stack-baseline` 是 generic/fusion skill，只负责一件事：**当任务要求写 Web 前端时，把技术栈、目录习惯和配色 token 收敛到一个共享源**。业务 skill 只需声明“按 frontend-stack-baseline 的默认栈交付”。

## Quick Contract

- **Trigger**: 新建 React/Vite 前端、脚手架、UI 重构、配色方案、shadcn/ui、Tailwind 主题、dashboard 主题、后台管理视觉系统；或任何领域 skill 调用时指定“按基线交付”。
- **Inputs**: 产品类型、品牌倾向（冷/暖/深/薄荷/玫瑰）、是否需要深色主题、是否需要管理台。
- **Outputs**: `package.json` 依赖列表、`tailwind.config.ts` 色板配置、`app/globals.css` 中的 shadcn CSS 变量 `:root { ... }` 段，以及状态管理 / 路由 / 表单 / 请求层的入口骨架。
- **Quality Gate**: 栈内依赖必须与本基线一致；不接受脱离 shadcn CSS 变量的手写配色；不接受未声明默认配色方案就开工。
- **Decision Tree**: 先判断是否仍在本栈；不在本栈则拒绝，或明确标注例外原因。再按产品类型从五套配色中选一套；管理台优先 ①冷灰蓝 / ③深青灰，内容平台优先 ②暖米白 / ④薄荷绿。

## The Iron Law

1. **技术栈固定**（不允许无理由替换）：
   - 构建：`Vite`（默认）+ `TypeScript`
   - 框架：`React 18+`
   - 样式：`Tailwind CSS 3+`
   - 组件库：`shadcn/ui`（CLI 按需添加组件，不整包安装）
   - 状态：`Zustand`（轻客户端状态）
   - 数据层：`TanStack Query v5`（服务端状态、缓存、失效、乐观更新）
   - 路由：`React Router v6+`（纯 SPA）或 Next.js App Router（仅 SSR / SEO 场景）
   - 表单：`React Hook Form` + `Zod`（`@hookform/resolvers/zod`）
   - 动画：`Framer Motion`（复杂交互）+ Tailwind transitions（简单）
2. **配色固定**：五套低饱和度方案任选其一，通过 shadcn/ui CSS 变量落地；主色饱和度 20–40 区间，避免原生饱和 Primary。
3. **主色由 CSS 变量驱动**，不在组件里写死 hex；换主题 = 换 `:root` 变量，组件零改动。
4. **栈封闭**：默认不引入本基线以外的 UI 组件库、状态库、表单库、请求库；若历史仓库必须沿用其他选型，需显式标注例外原因与边界。
5. **文件结构默认**：`src/{components,ui,hooks,stores,routes,lib,api,schemas,styles}`；`ui/` 固定放 shadcn 生成的原子组件，业务组件在 `components/`。

## Announce

Begin by stating clearly:
"I am using `frontend-stack-baseline` to apply the default React + Vite + Tailwind + shadcn/ui stack and a low-saturation palette."

## Workflow

1. **确认栈适用性**：新项目直接按本基线；遗留项目若无法全量迁移，允许“新模块按基线、旧模块保持”，但必须标注边界。
2. **选配色**：按产品类型选一套；管理台 ①或③；内容 ②；效率/健康 ④；设计/情感向 ⑤。
3. **落 `tailwind.config.ts`**：把选中的 palette 映射到 Tailwind color 令牌（见 [references/color-tokens.md](./references/color-tokens.md)）。
4. **落 `globals.css`**：把选中 palette 对应的 shadcn CSS 变量贴进 `:root`；若需深色主题，补 `.dark { ... }`。
5. **初始化 shadcn/ui**：`pnpm dlx shadcn@latest init`，按 palette 回答基色；再用 `shadcn@latest add button card input form dialog ...` 按需补组件。
6. **装齐依赖**：见 [references/scaffold-commands.md](./references/scaffold-commands.md) 的一键脚本。
7. **绑定领域 skill**：如 `lazycat:create-app` 仅声明“按 frontend-stack-baseline 交付 React 前端”，不得重复定义栈；`lazycat:admin-ui` 引用本 skill 的配色 token 而非另立色板。

## Color Palette Presets

| 编号 | 名称 | 适用场景 | 主色 | 背景 | 备注 |
|---|---|---|---|---|---|
| ① | Slate 冷灰蓝 | 后台管理 / SaaS | `#4A6FA5` | `#F8F9FB` | 管理台首选 |
| ② | Warm Sand 暖米白 | 内容平台 / 博客 | `#8B7355` | `#FAFAF8` | 长阅读不疲劳 |
| ③ | Dark Slate 深青灰 | 开发工具 / 深色 | `#5B8DB8` | `#1A1D23` | 唯一深色预设 |
| ④ | Sage 薄荷绿 | 健康 / 效率 | `#5A7D6A` | `#F5F7F5` | 冷静克制 |
| ⑤ | Rose Mist 玫瑰灰 | 设计工具 / 女性向 | `#9B6B6B` | `#FAF8F8` | 情感、柔和 |

完整的 `:root` CSS 变量、`tailwind.config.ts` 色板映射、深色变体和使用示例见 [references/color-tokens.md](./references/color-tokens.md)。

## Quality Gates

- `package.json` 里同时出现本基线的 10 项依赖，没有同职责的第三方库重复引入。
- `tailwind.config.ts` 的 `theme.extend.colors` 引用了 shadcn 变量（如 `hsl(var(--primary))`），没有写死 hex。
- `globals.css` 存在 `:root { --background: ...; --primary: ...; }` 完整段。
- 如果是管理台，至少存在工作台、列表、详情、表单、设置、登录六类页面，配色保持一致。
- 切换 palette 只需要改 `:root`，组件层不需要改。

## Red Flags

- 组件里直接写 `bg-[#4A6FA5]`，没有通过 token。
- 五套配色自己魔改，饱和度 > 50，导致长时间使用刺眼。
- 领域 skill 里重复声明另一套默认栈或自有色板。

## Bundled References

- 完整配色与 Tailwind 片段：[references/color-tokens.md](./references/color-tokens.md)
- 脚手架安装命令：[references/scaffold-commands.md](./references/scaffold-commands.md)

## Outputs

```text
Phase: Frontend Baseline / Theming

Stack
- Vite + React + TypeScript
- Tailwind CSS + shadcn/ui
- Zustand / TanStack Query
- React Router (or Next.js)
- React Hook Form + Zod
- Framer Motion

Palette
- Chosen: <① Slate / ② Warm Sand / ③ Dark Slate / ④ Sage / ⑤ Rose Mist>
- Dark mode: <needed / skipped>

Artifacts
- package.json dependency diff
- tailwind.config.ts (colors extended)
- src/styles/globals.css (:root and .dark blocks)
- src/components/ui (shadcn components added)

Gaps / Risks
- ...

Next Steps
1. ...
```
