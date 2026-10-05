# Frontend Color Tokens

Applied by arc:code when it sets up or changes a frontend theme; checked by arc:audit. Stack rules:
[frontend.md](frontend.md).

## 1. Rules

1. Use exactly one palette per product; the five presets are mutually exclusive.
2. Selection: admin/dashboard → ① or ③; content platform → ②; health or productivity tool → ④; design or
   emotional product → ⑤.
3. ③ is the default `.dark` variant and pairs with any light palette (①②④⑤). IF the product is dark-first
   THEN put ③ in `:root` instead. Never combine two light palettes (e.g. ② + ⑤): the hue drifts.
4. `--radius`: `0.5rem` for admin consoles, `0.75rem` for content products (§4 lists each preset's default).
5. Components use token classes only (`bg-primary`, `text-primary-foreground`, `border-border`). Changing the
   palette changes only the variable block; components stay untouched.
6. Light + dark: light palette in `:root`, ③ in `.dark`; on first load set `<html class="dark">` from
   `prefers-color-scheme` or the user's saved preference.
7. When adjusting a preset, keep primary saturation within 20–40%.

## 2. Wiring

Values in §4 are space-separated HSL without `hsl()`.

**Tailwind CSS v4** (new projects; what `shadcn init` generates):

1. Write the variables in the CSS file named by `components.json` → `tailwind.css`, at top level (not inside
   `@layer base`), each color wrapped in `hsl()`.
2. Keep the generated `@import` lines, `@custom-variant dark`, `@theme inline` mapping and `chart-*` /
   `sidebar-*` values; replace only the variables in §4.
3. Add `--color-destructive-foreground: var(--destructive-foreground);` to `@theme inline` (the v4 default
   mapping omits it).

```css
:root {
  --background: hsl(220 27% 97.8%);
  --foreground: hsl(215 33% 17.5%);
  --primary: hsl(216 38% 46.9%);
  /* …every remaining §4 row for the chosen palette… */
  --radius: 0.5rem;
}
.dark {
  --background: hsl(220 15% 12%);
  /* …every remaining ③ row… */
}
```

**Tailwind CSS v3** (existing v3 projects): bare values inside `@layer base`, mapped by `tailwind.config.js`.

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root { --background: 220 27% 97.8%; /* …§4 rows… */ --radius: 0.5rem; }
  .dark { --background: 220 15% 12%; /* …③ rows… */ }
}
```

```js
const animate = require("tailwindcss-animate");

/** @type {import("tailwindcss").Config} */
module.exports = {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{js,jsx,ts,tsx}"],
  theme: {
    container: { center: true, padding: "1rem", screens: { "2xl": "1400px" } },
    extend: {
      colors: {
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        card: { DEFAULT: "hsl(var(--card))", foreground: "hsl(var(--card-foreground))" },
        popover: { DEFAULT: "hsl(var(--popover))", foreground: "hsl(var(--popover-foreground))" },
        primary: { DEFAULT: "hsl(var(--primary))", foreground: "hsl(var(--primary-foreground))" },
        secondary: { DEFAULT: "hsl(var(--secondary))", foreground: "hsl(var(--secondary-foreground))" },
        muted: { DEFAULT: "hsl(var(--muted))", foreground: "hsl(var(--muted-foreground))" },
        accent: { DEFAULT: "hsl(var(--accent))", foreground: "hsl(var(--accent-foreground))" },
        destructive: { DEFAULT: "hsl(var(--destructive))", foreground: "hsl(var(--destructive-foreground))" },
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

## 3. Palette summary (hex)

| Palette | bg | card | border | primary | accent | fg | muted-fg |
|---|---|---|---|---|---|---|---|
| ① Slate 冷灰蓝（后台管理 / SaaS） | #F8F9FB | #FFFFFF | #E2E6ED | #4A6FA5 | #6B8FC2 | #1E2A3B | #6B7A90 |
| ② Warm Sand 暖米白（内容平台 / 博客） | #FAFAF8 | #FFFFFF | #E8E6E0 | #8B7355 | #C4A882 | #2C2416 | #7A6E60 |
| ③ Dark Slate 深青灰（开发工具 / 深色系） | #1A1D23 | #22262F | #2E3340 | #5B8DB8 | #7FB3D3 | #D8DDE6 | #8892A0 |
| ④ Sage 薄荷绿（健康 / 效率） | #F5F7F5 | #FFFFFF | #DDE4DC | #5A7D6A | #7BA08C | #1E2B22 | #6B7D71 |
| ⑤ Rose Mist 玫瑰灰（设计工具 / 女性向） | #FAF8F8 | #FFFFFF | #EDE6E6 | #9B6B6B | #C49090 | #2B1E1E | #8A7070 |

## 4. Variables

| Variable | ① Slate | ② Warm Sand | ③ Dark Slate (`.dark`) | ④ Sage | ⑤ Rose Mist |
|---|---|---|---|---|---|
| `--background` | 220 27% 97.8% | 60 17% 97.6% | 220 15% 12% | 120 11% 96.5% | 0 17% 97.6% |
| `--foreground` | 215 33% 17.5% | 38 33% 12.9% | 219 22% 87.5% | 138 18% 14.3% | 0 18% 14.3% |
| `--card` | 0 0% 100% | 0 0% 100% | 222 16% 15.9% | 0 0% 100% | 0 0% 100% |
| `--card-foreground` | 215 33% 17.5% | 38 33% 12.9% | 219 22% 87.5% | 138 18% 14.3% | 0 18% 14.3% |
| `--popover` | 0 0% 100% | 0 0% 100% | 222 16% 15.9% | 0 0% 100% | 0 0% 100% |
| `--popover-foreground` | 215 33% 17.5% | 38 33% 12.9% | 219 22% 87.5% | 138 18% 14.3% | 0 18% 14.3% |
| `--primary` | 216 38% 46.9% | 33 24% 43.9% | 208 40% 53.9% | 147 16% 42.2% | 0 19% 51.4% |
| `--primary-foreground` | 0 0% 100% | 60 17% 97.6% | 220 15% 12% | 120 11% 96.5% | 0 17% 97.6% |
| `--secondary` | 218 23% 94% | 45 15% 93% | 223 16% 19% | 113 13% 92% | 0 16% 94% |
| `--secondary-foreground` | 215 33% 17.5% | 38 33% 12.9% | 219 22% 87.5% | 138 18% 14.3% | 0 18% 14.3% |
| `--muted` | 218 23% 94% | 45 15% 93% | 223 16% 19% | 113 13% 92% | 0 16% 94% |
| `--muted-foreground` | 216 15% 49.2% | 32 12% 42.7% | 215 11% 58% | 140 8% 45.5% | 0 10% 49% |
| `--accent` | 215 42% 59% | 35 36% 63.9% | 203 49% 66.3% | 148 16% 55.5% | 0 31% 66.7% |
| `--accent-foreground` | 0 0% 100% | 38 33% 12.9% | 220 15% 12% | 138 18% 14.3% | 0 18% 14.3% |
| `--destructive` | 0 72% 51% | 0 65% 48% | 0 62% 45% | 0 65% 48% | 0 72% 46% |
| `--destructive-foreground` | 0 0% 100% | 0 0% 100% | 0 0% 100% | 0 0% 100% | 0 0% 100% |
| `--border` | 218 23% 90.8% | 45 15% 89.4% | 223 16% 21.6% | 113 13% 87.8% | 0 16% 91.6% |
| `--input` | 218 23% 90.8% | 45 15% 89.4% | 223 16% 21.6% | 113 13% 87.8% | 0 16% 91.6% |
| `--ring` | 216 38% 46.9% | 33 24% 43.9% | 208 40% 53.9% | 147 16% 42.2% | 0 19% 51.4% |
| `--radius` | 0.5rem | 0.75rem | — (inherits `:root`) | 0.5rem | 0.75rem |
