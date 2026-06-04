# Color Token Presets

五套低饱和度配色方案。每套都以 shadcn/ui 的 HSL CSS 变量系统落地，主色饱和度控制在 20–40%，长时间使用不刺眼。

> HSL 值已从原始 hex 精确换算，空格分隔、不带 `hsl()`，直接粘到 shadcn 变量块即可。

---

## 通用约定

- **命名选择一套 palette 即可**。五套互斥，不要混用。
- 管理台 / Dashboard 首选 ① 冷灰蓝 或 ③ 深青灰；内容平台选 ② 暖米白；健康 / 效率工具选 ④ 薄荷绿；设计 / 情感向产品选 ⑤ 玫瑰灰。
- ③ 深青灰默认作为 `.dark` 变体，可与任意浅色 palette 组合为 light/dark 双主题；若产品天然深色，也可把 ③ 放进 `:root`。
- `--radius` 默认 `0.5rem`（管理台）或 `0.75rem`（内容向），按产品调性自选。

---

## ① Slate 冷灰蓝（后台管理 / SaaS）

```text
bg #F8F9FB  card #FFFFFF  border #E2E6ED
primary #4A6FA5  accent #6B8FC2
fg #1E2A3B  muted-fg #6B7A90
```

### `globals.css`

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    --background: 220 27% 97.8%;
    --foreground: 215 33% 17.5%;

    --card: 0 0% 100%;
    --card-foreground: 215 33% 17.5%;

    --popover: 0 0% 100%;
    --popover-foreground: 215 33% 17.5%;

    --primary: 216 38% 46.9%;
    --primary-foreground: 0 0% 100%;

    --secondary: 218 23% 94%;
    --secondary-foreground: 215 33% 17.5%;

    --muted: 218 23% 94%;
    --muted-foreground: 216 15% 49.2%;

    --accent: 215 42% 59%;
    --accent-foreground: 0 0% 100%;

    --destructive: 0 72% 51%;
    --destructive-foreground: 0 0% 100%;

    --border: 218 23% 90.8%;
    --input: 218 23% 90.8%;
    --ring: 216 38% 46.9%;

    --radius: 0.5rem;
  }
}
```

---

## ② Warm Sand 暖米白（内容平台 / 博客）

```text
bg #FAFAF8  card #FFFFFF  border #E8E6E0
primary #8B7355  accent #C4A882
fg #2C2416  muted-fg #7A6E60
```

### `globals.css`

```css
@layer base {
  :root {
    --background: 60 17% 97.6%;
    --foreground: 38 33% 12.9%;

    --card: 0 0% 100%;
    --card-foreground: 38 33% 12.9%;

    --popover: 0 0% 100%;
    --popover-foreground: 38 33% 12.9%;

    --primary: 33 24% 43.9%;
    --primary-foreground: 60 17% 97.6%;

    --secondary: 45 15% 93%;
    --secondary-foreground: 38 33% 12.9%;

    --muted: 45 15% 93%;
    --muted-foreground: 32 12% 42.7%;

    --accent: 35 36% 63.9%;
    --accent-foreground: 38 33% 12.9%;

    --destructive: 0 65% 48%;
    --destructive-foreground: 0 0% 100%;

    --border: 45 15% 89.4%;
    --input: 45 15% 89.4%;
    --ring: 33 24% 43.9%;

    --radius: 0.75rem;
  }
}
```

---

## ③ Dark Slate 深青灰（开发工具 / 深色系）

```text
bg #1A1D23  card #22262F  border #2E3340
primary #5B8DB8  accent #7FB3D3
fg #D8DDE6  muted-fg #8892A0
```

### `globals.css` — 作为 `.dark` 变体

```css
@layer base {
  .dark {
    --background: 220 15% 12%;
    --foreground: 219 22% 87.5%;

    --card: 222 16% 15.9%;
    --card-foreground: 219 22% 87.5%;

    --popover: 222 16% 15.9%;
    --popover-foreground: 219 22% 87.5%;

    --primary: 208 40% 53.9%;
    --primary-foreground: 220 15% 12%;

    --secondary: 223 16% 19%;
    --secondary-foreground: 219 22% 87.5%;

    --muted: 223 16% 19%;
    --muted-foreground: 215 11% 58%;

    --accent: 203 49% 66.3%;
    --accent-foreground: 220 15% 12%;

    --destructive: 0 62% 45%;
    --destructive-foreground: 0 0% 100%;

    --border: 223 16% 21.6%;
    --input: 223 16% 21.6%;
    --ring: 208 40% 53.9%;
  }
}
```

> 若产品以深色为主，可把同样的变量直接放进 `:root`，省掉 `.dark` 嵌套。

---

## ④ Sage 薄荷绿（健康 / 效率）

```text
bg #F5F7F5  card #FFFFFF  border #DDE4DC
primary #5A7D6A  accent #7BA08C
fg #1E2B22  muted-fg #6B7D71
```

### `globals.css`

```css
@layer base {
  :root {
    --background: 120 11% 96.5%;
    --foreground: 138 18% 14.3%;

    --card: 0 0% 100%;
    --card-foreground: 138 18% 14.3%;

    --popover: 0 0% 100%;
    --popover-foreground: 138 18% 14.3%;

    --primary: 147 16% 42.2%;
    --primary-foreground: 120 11% 96.5%;

    --secondary: 113 13% 92%;
    --secondary-foreground: 138 18% 14.3%;

    --muted: 113 13% 92%;
    --muted-foreground: 140 8% 45.5%;

    --accent: 148 16% 55.5%;
    --accent-foreground: 138 18% 14.3%;

    --destructive: 0 65% 48%;
    --destructive-foreground: 0 0% 100%;

    --border: 113 13% 87.8%;
    --input: 113 13% 87.8%;
    --ring: 147 16% 42.2%;

    --radius: 0.5rem;
  }
}
```

---

## ⑤ Rose Mist 玫瑰灰（设计工具 / 女性向）

```text
bg #FAF8F8  card #FFFFFF  border #EDE6E6
primary #9B6B6B  accent #C49090
fg #2B1E1E  muted-fg #8A7070
```

### `globals.css`

```css
@layer base {
  :root {
    --background: 0 17% 97.6%;
    --foreground: 0 18% 14.3%;

    --card: 0 0% 100%;
    --card-foreground: 0 18% 14.3%;

    --popover: 0 0% 100%;
    --popover-foreground: 0 18% 14.3%;

    --primary: 0 19% 51.4%;
    --primary-foreground: 0 17% 97.6%;

    --secondary: 0 16% 94%;
    --secondary-foreground: 0 18% 14.3%;

    --muted: 0 16% 94%;
    --muted-foreground: 0 10% 49%;

    --accent: 0 31% 66.7%;
    --accent-foreground: 0 18% 14.3%;

    --destructive: 0 72% 46%;
    --destructive-foreground: 0 0% 100%;

    --border: 0 16% 91.6%;
    --input: 0 16% 91.6%;
    --ring: 0 19% 51.4%;

    --radius: 0.75rem;
  }
}
```

---

## `tailwind.config.js`（五套共用，只换 `:root` 变量即可）

```js
const animate = require("tailwindcss-animate");

/** @type {import("tailwindcss").Config} */
module.exports = {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{js,jsx}"],
  theme: {
    container: { center: true, padding: "1rem", screens: { "2xl": "1400px" } },
    extend: {
      colors: {
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
        popover: {
          DEFAULT: "hsl(var(--popover))",
          foreground: "hsl(var(--popover-foreground))",
        },
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
      },
    },
  },
  plugins: [animate],
};
```

组件里一律用 `bg-primary`、`text-primary-foreground`、`border-border` 等 token 类；换 palette 只改 `:root`，组件零改动。

---

## Light + Dark 组合范式

若需要同时支持浅 / 深色：

1. 在 `:root { ... }` 里放 ①②④⑤ 中的一套作为浅色。
2. 在 `.dark { ... }` 里放 ③ 深青灰变量。
3. 初次加载按 `prefers-color-scheme` 或用户偏好切换 `<html class="dark">`。

不推荐把两套浅色叠加（例如 ② + ⑤），会出现色相漂移。
