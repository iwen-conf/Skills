---
name: frontend-stack-baseline
description: 通用前端基线声明：用于收敛 React + Vite 前端项目的核心技术栈、默认配色 token 与架构策略，避免各领域 skill 重复定义。
---

# Frontend Stack Baseline

## Overview

`frontend-stack-baseline` 是 generic/fusion skill，只负责一件事：**当任务要求写 Web 前端时，把技术栈、目录习惯和配色 token 收敛到一个共享源**。业务 skill 只需声明“按 frontend-stack-baseline 的默认栈交付”。

## Quick Contract

- **Trigger**: 新建 React/Vite 前端、脚手架、UI 重构、配色方案、shadcn/ui、Tailwind 主题、dashboard 主题、后台管理视觉系统；或任何领域 skill 调用时指定“按基线交付”。
- **Inputs**: 产品类型、品牌倾向（冷/暖/深/薄荷/玫瑰）、是否需要深色主题、是否需要管理台。
- **Outputs**: `package.json` 依赖列表、`tailwind.config.js` 色板配置、`src/styles/globals.css` 中的 shadcn CSS 变量 `:root { ... }` 段，以及状态管理 / 路由 / 表单 / 请求层的入口骨架。
- **Quality Gate**: 栈内依赖必须与本基线一致；不接受脱离 shadcn CSS 变量的手写配色；不接受未声明默认配色方案就开工。
- **Decision Tree**: 先判断是否仍在本栈；不在本栈则拒绝，或明确标注例外原因。再按产品类型从五套配色中选一套；管理台优先 ①冷灰蓝 / ③深青灰，内容平台优先 ②暖米白 / ④薄荷绿。

## Context Search

- 在既有前端仓库中落地或改造时，先用目标仓库的 `.ai-code-index/search.sh` 定位现有入口、路由、主题、组件库和状态管理方式。
- 搜索 JSX/TSX 调用形态或重复模式时，优先用 `.ai-code-index/struct-search.sh typescript`。
- 已知文件名、依赖名或 token 的精确匹配，可用 `rg` 小范围补充；索引缺失时先考虑 `.ai-code-index/reindex.sh`。

## The Iron Law

1. **技术栈固定**（不允许无理由替换）：
   - 构建：`Vite`（默认）
   - 框架：`React 18+`
   - 样式：`Tailwind CSS 3+`
   - 组件库：`shadcn/ui`（CLI 按需添加组件，不整包安装）
   - 状态：`Zustand`（轻客户端状态）
   - 数据层：`TanStack Query v5`（服务端状态、缓存、失效、乐观更新）
   - 路由：`React Router v6+`
   - 表单：`React Hook Form` + `Zod`（`@hookform/resolvers/zod`）
   - 动画：`Framer Motion`（复杂交互）+ Tailwind transitions（简单）
2. **配色固定**：五套低饱和度方案任选其一，通过 shadcn/ui CSS 变量落地；主色饱和度 20–40 区间，避免原生饱和 Primary。
3. **主色由 CSS 变量驱动**，不在组件里写死 hex；换主题 = 换 `:root` 变量，组件零改动。
4. **栈封闭**：默认不引入本基线以外的 UI 组件库、状态库、表单库、请求库；若历史仓库必须沿用其他选型，需显式标注例外原因与边界。
5. **文件结构默认**：`src/{components,ui,hooks,stores,routes,lib,api,schemas,styles}`；`ui/` 固定放 shadcn 生成的原子组件，业务组件在 `components/`。

## Announce

Begin by stating clearly:
"I am using `frontend-stack-baseline` to apply the default React + Vite + Tailwind + shadcn/ui stack and a low-saturation palette."

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `product_type` | string | yes | 产品类型，例如后台管理、Dashboard、Landing、内容平台或工具型应用 |
| `brand_tone` | string | no | 品牌倾向，例如冷、暖、深色、薄荷、玫瑰 |
| `dark_mode` | boolean | no | 是否需要深色主题 |
| `admin_console` | boolean | no | 是否是管理台、控制台或 B 端工作台 |
| `existing_project` | string | no | 既有前端项目路径，用于按现状边界落地 |

## Workflow

1. **确认栈适用性**：新项目直接按本基线；遗留项目若无法全量迁移，允许“新模块按基线、旧模块保持”，但必须标注边界。
2. **选配色**：按产品类型选一套；管理台 ①或③；内容 ②；效率/健康 ④；设计/情感向 ⑤。
3. **落 `tailwind.config.js`**：把选中的 palette 映射到 Tailwind color 令牌（见 [references/color-tokens.md](./references/color-tokens.md)）。
4. **落 `globals.css`**：把选中 palette 对应的 shadcn CSS 变量贴进 `:root`；若需深色主题，补 `.dark { ... }`。
5. **初始化 shadcn/ui**：`pnpm dlx shadcn@latest init`，按 palette 回答基色；再用 `shadcn@latest add button card input form dialog ...` 按需补组件。
6. **装齐依赖**：见 [references/scaffold-commands.md](./references/scaffold-commands.md) 的一键脚本。
7. **绑定领域 skill**：如 `lazycat:create-app` 仅声明“按 frontend-stack-baseline 交付 React 前端”，不得重复定义栈；`lazycat:admin-ui` 引用本 skill 的配色 token 而非另立色板。

## Pagination Strategy

根据业务场景选择合适的分页模式，并在实现时贯彻：
- **传统分页（Offset Pagination）**：当业务为后台管理系统、订单列表、用户列表时，用户确实需要精确跳页、总数感知强烈，使用传统分页更自然。UI 应配合使用 pagination 组件。
- **游标分页（Cursor Pagination / 瀑布流 / 无限滚动）**：当业务为评论、动态流、聊天记录、消息通知、日志流时，数据频繁追加且不需要跳页，使用 cursor 分页更合适。UI 应配合无限滚动（Infinite Scroll）或“加载更多”按钮。

## State Management Strategy

前端状态应根据生命周期和共享范围进行严格分层，切忌“把所有东西都塞进全局 Store”：
- **URL State（路由状态）**：筛选条件、搜索关键字、当前页码、激活的 Tab。优先存入 URL Query Params，保证页面刷新不丢，且可被作为链接分享。
- **Server State（服务端状态）**：业务列表数据、详情数据。全权交由 `TanStack Query` 管理（查询、缓存、失效、乐观更新），严禁使用 `useEffect` 手动 fetch 并存入 Zustand。
- **Client Global State（客户端全局状态）**：主题模式（Dark Mode）、侧边栏折叠状态、跨页面的表单向导数据。使用 `Zustand` 管理。
- **Local State（组件局部状态）**：弹窗的开/关、输入框的实时受控值、下拉菜单的状态。直接使用 `useState`。

## Real-time & Polling Strategy

根据业务对数据时效性的要求选择同步机制：
- **长链接（WebSocket / SSE）**：当业务为 IM 聊天室、协同文档编辑、终端日志流、强实时交易盘口时。适合需要服务端高频主动推送的场景，需做好重连与心跳机制。
- **短轮询（Polling）**：当业务为大盘数据仪表盘（Dashboard）、耗时任务的状态跟进、订单状态查询时。利用 `TanStack Query` 的 `refetchInterval`（例如 5~15秒），实现成本低且能满足大部分 B 端监控需求。

## Rendering Architecture Strategy

根据业务对 SEO 和首屏时间（FCP）的诉求选择渲染模式：
- **纯客户端渲染（CSR）**：本基线默认方案（Vite + React）。当业务为后台管理系统、SaaS 工作台、内部工具、复杂交互仪表盘时。无需 SEO，首屏短暂白屏或骨架屏可接受，重在后续交互的流畅度与极低的静态托管部署成本。
- **服务端渲染 / 静态生成（SSR / SSG）**：当业务为 C 端门户、电商商品页、官方文档、博客时。对 SEO 有强诉求，对首屏加载速度要求极高，需打破本基线限制，引入 Next.js 或 Remix 框架。

## Auth & RBAC Strategy

根据安全与用户体验要求，在不同粒度实施权限控制：
- **路由级拦截（Route Guard）**：业务页面整体受控时。未登录或无权限直接重定向到 `/login` 或 `/403`，防止加载未经授权的页面级代码。
- **组件级细粒度控制（Component-level RBAC）**：页面可访问，但仅部分操作受限时。通过封装特定的包装组件，将无权限的按钮置为 `disabled`（并辅以 Tooltip 解释）或直接隐藏。

## Error Handling & Feedback Strategy

针对不同来源的错误提供分级反馈，避免粗暴打断用户心流：
- **全局故障边界（Global Error Boundary）**：捕获 React 渲染过程中的崩溃，展示友好的“页面崩溃”兜底 UI，并提供刷新按钮，避免整个应用直接白屏。
- **数据获取错误（Fetch Error）**：列表或详情页数据拉取失败时，应在**原数据渲染区域**展示局部错误状态（如“加载失败，点击重试”），严禁使用打断式的全局遮罩或全屏弹窗。
- **操作类错误（Mutation Error）**：表单提交或动作执行失败时，使用轻量级的 Toast / Snackbar 在角落提示具体原因，焦点必须保留在原表单，以便用户修改。

## Form & Validation Strategy

根据表单的复杂度决定工程化投入：
- **重型/复杂表单**：业务涉及多字段联动、动态增减表单项、分步表单、严格格式校验。强制使用 `React Hook Form` 配合 `Zod`，保证校验逻辑集中且类型安全，有效避免全局过度渲染。
- **超轻量输入**：单输入框（如简单的搜索、发送单条评论），直接使用 `useState` 作为受控组件即可，不要强行引入表单库增加不必要的复杂度。

## Color Palette Presets

| 编号 | 名称 | 适用场景 | 主色 | 背景 | 备注 |
|---|---|---|---|---|---|
| ① | Slate 冷灰蓝 | 后台管理 / SaaS | `#4A6FA5` | `#F8F9FB` | 管理台首选 |
| ② | Warm Sand 暖米白 | 内容平台 / 博客 | `#8B7355` | `#FAFAF8` | 长阅读不疲劳 |
| ③ | Dark Slate 深青灰 | 开发工具 / 深色 | `#5B8DB8` | `#1A1D23` | 唯一深色预设 |
| ④ | Sage 薄荷绿 | 健康 / 效率 | `#5A7D6A` | `#F5F7F5` | 冷静克制 |
| ⑤ | Rose Mist 玫瑰灰 | 设计工具 / 女性向 | `#9B6B6B` | `#FAF8F8` | 情感、柔和 |

完整的 `:root` CSS 变量、`tailwind.config.js` 色板映射、深色变体和使用示例见 [references/color-tokens.md](./references/color-tokens.md)。

## Quality Gates

- `package.json` 里同时出现本基线的 10 项依赖，没有同职责的第三方库重复引入。
- `tailwind.config.js` 的 `theme.extend.colors` 引用了 shadcn 变量（如 `hsl(var(--primary))`），没有写死 hex。
- `globals.css` 存在 `:root { --background: ...; --primary: ...; }` 完整段。
- 如果是管理台，至少存在工作台、列表、详情、表单、设置、登录六类页面，配色保持一致。
- 切换 palette 只需要改 `:root`，组件层不需要改。

## Red Flags

- 组件里直接写 `bg-[#4A6FA5]`，没有通过 token。
- 五套配色自己魔改，饱和度 > 50，导致长时间使用刺眼。
- 领域 skill 里重复声明另一套默认栈或自有色板。

## When to Use

- **首选触发**：用户要求搭建、改造或统一 Web 前端，且技术栈可落在 React + Vite + Tailwind CSS + shadcn/ui。
- **典型场景**：后台管理、Dashboard、控制台、Landing、内容平台、工具型应用、主题 token 统一、前端脚手架。
- **边界提示**：若任务是原生 iOS/Android、后端 API、纯设计稿或既有项目明确要求其他前端栈，不要强行套用本基线。

## Bundled References

- 完整配色与 Tailwind 片段：[references/color-tokens.md](./references/color-tokens.md)
- 脚手架安装命令：[references/scaffold-commands.md](./references/scaffold-commands.md)

## Outputs

```text
Phase: Frontend Baseline / Theming

Stack
- Vite + React
- Tailwind CSS + shadcn/ui
- Zustand / TanStack Query
- React Router
- React Hook Form + Zod
- Framer Motion

Palette
- Chosen: <① Slate / ② Warm Sand / ③ Dark Slate / ④ Sage / ⑤ Rose Mist>
- Dark mode: <needed / skipped>

Artifacts
- package.json dependency diff
- tailwind.config.js (colors extended)
- src/styles/globals.css (:root and .dark blocks)
- src/components/ui (shadcn components added)

Gaps / Risks
- ...

Next Steps
1. ...
```
