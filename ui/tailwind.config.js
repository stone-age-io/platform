/** @type {import('tailwindcss').Config} */

// TAILWIND IS PINNED TO 3.x AND DAISYUI TO 4.x, DELIBERATELY.
//
// Upgrading is a design-system migration, not a version bump, and nothing in CI
// renders the UI: `vue-tsc && vite build` stays green while it renders wrong.
// The measured cost:
//
//   - 376 references in 27 files use the v4 form `oklch(var(--b1))`. daisyUI 5
//     renames those vars (--b1 -> --color-base-100) and makes them complete
//     colors, so each becomes an invalid `oklch(oklch(...))` that silently falls
//     back. Every site has to change.
//   - The alpha form `oklch(var(--bc) / 0.7)` would need color-mix() or relative
//     color syntax, both already rejected in assets/dashboard-compat.css for
//     failing silently on older engines.
//   - The theme block below is copied verbatim into the access-control console
//     (tailwind ^3.4 / daisyui ^4.4). daisyUI 5 moves themes into CSS `@plugin`
//     syntax and renames the radius vars (--rounded-box -> --radius-box,
//     --rounded-btn -> --radius-field, --rounded-badge -> --radius-selector,
//     --tab-radius dropped), so migrating one repo alone breaks that contract.
//   - 133 distinct daisyUI utility-class variants, plus Tailwind 4's renames.
//
// So upgrade only as scheduled work across both repos, with a human clicking
// through dashboards, widgets and both themes. Never in a routine "bump
// everything" pass. Verified on Vite 8 at 3.4.19 / 4.12.24.
//
// Custom `light` / `dark` themes override the stock pair, so the ui store's
// data-theme toggle keeps working. base-200 is the page ground (assets/main.css
// body), base-100 the card surface, base-300 the border. Shared with the
// access-control console: keep in step, copied verbatim, not adapted.
//
// Widgets read this palette via assets/dashboard-compat.css and
// composables/useDesignTokens.ts. `secondary` and `neutral` are desaturated,
// which is why getChartColor() keeps its own categorical ramp.
const shared = {
  "--rounded-box": "0.75rem",
  "--rounded-btn": "0.5rem",
  "--rounded-badge": "0.375rem",
  "--tab-radius": "0.5rem",
  "--border-btn": "1px",
  "--animation-btn": "0.2s",
}

export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {},
  },
  plugins: [require("daisyui")],
  daisyui: {
    logs: false,
    darkTheme: "dark",
    themes: [
      {
        light: {
          "color-scheme": "light",
          primary: "#4F46E5",
          "primary-content": "#FFFFFF",
          secondary: "#64748B",
          "secondary-content": "#FFFFFF",
          accent: "#0D9488",
          "accent-content": "#FFFFFF",
          neutral: "#1F2733",
          "neutral-content": "#F4F6FA",
          "base-100": "#FFFFFF",
          "base-200": "#F4F6FA",
          "base-300": "#E4E8EF",
          "base-content": "#1B2330",
          info: "#2563EB",
          "info-content": "#FFFFFF",
          success: "#16A34A",
          "success-content": "#FFFFFF",
          warning: "#D97706",
          "warning-content": "#FFFFFF",
          error: "#DC2626",
          "error-content": "#FFFFFF",
          ...shared,
        },
      },
      {
        dark: {
          "color-scheme": "dark",
          primary: "#6366F1",
          "primary-content": "#FFFFFF",
          secondary: "#94A3B8",
          "secondary-content": "#0D1017",
          accent: "#2DD4BF",
          "accent-content": "#0D1017",
          neutral: "#232A35",
          "neutral-content": "#E7EBF2",
          "base-100": "#12161D",
          "base-200": "#0D1017",
          "base-300": "#232A35",
          "base-content": "#E7EBF2",
          info: "#3B82F6",
          "info-content": "#FFFFFF",
          success: "#22C55E",
          "success-content": "#052E16",
          warning: "#F59E0B",
          "warning-content": "#1C1300",
          error: "#EF4444",
          "error-content": "#FFFFFF",
          ...shared,
        },
      },
    ],
  },
}
