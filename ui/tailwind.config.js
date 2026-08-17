/** @type {import('tailwindcss').Config} */

// TAILWIND IS PINNED TO 3.x AND DAISYUI TO 4.x, DELIBERATELY.
//
// Tailwind 4 and daisyUI 5 are both out, and everything else in package.json
// tracks latest. These two do not, because upgrading them is a design-system
// migration rather than a version bump, and nothing in this repo would catch a
// mistake -- there is no frontend test runner, so `vue-tsc && vite build` stays
// green while the UI renders wrong.
//
// What the migration actually costs, measured rather than estimated:
//
//   - 376 references across 27 files (23 .vue, 3 .css, 1 .ts) are the v4 form
//     `oklch(var(--b1))`. daisyUI 5 renames those vars (--b1 -> --color-base-100)
//     AND changes their contents from bare OKLCH components to complete color
//     values, so every one of the 376 becomes `oklch(oklch(...))`, which is
//     invalid and silently falls back. Every site has to change, not just be
//     renamed.
//   - The alpha form `oklch(var(--bc) / 0.7)` has no direct v5 equivalent; it
//     needs color-mix() or relative color syntax. See the note in
//     assets/dashboard-compat.css: both were rejected once already because they
//     fail on older engines in the same silent way that bit us with --pf.
//   - The theme block below is copied verbatim into the access-control console
//     (see the next paragraph). daisyUI 5 moves theme definitions out of JS into
//     CSS `@plugin` syntax and renames the radius vars (--rounded-box ->
//     --radius-box, --rounded-btn -> --radius-field, --rounded-badge ->
//     --radius-selector, --tab-radius dropped). Migrating platform alone breaks
//     the "copied verbatim" contract; access-control is on tailwind ^3.4 /
//     daisyui ^4.4.
//   - 133 distinct daisyUI utility-class variants are used in templates, on top
//     of Tailwind 4's own utility renames.
//
// So: bump these two only as a deliberate, scheduled piece of work covering both
// repos, with a human clicking through the dashboards, widgets and both themes
// afterwards. Do not let a routine `npm update` or a "bump everything" pass drag
// them along. Verified working on Vite 8 at 3.4.19 / 4.12.24.
//
// Two custom daisyUI themes named `light` / `dark` (overriding the stock pair of
// the same name, so the data-theme toggle in the ui store keeps working). Cool
// neutrals with a faint indigo bias, a single indigo primary, and semantic colors
// reserved for state. Radii are tuned down from daisyUI's defaults — notably
// --rounded-badge, which squares the status pills into chips. base-200 is the page
// ground (see assets/main.css body), base-100 the card surface, base-300 the border.
//
// Shared with the access-control console so the two apps read as one product.
// Keep them in step: this block is copied verbatim, not adapted.
//
// Note for dashboards: the widget layer reads this palette indirectly, via the
// DaisyUI vars bridged in assets/dashboard-compat.css and resolved to RGB in
// composables/useDesignTokens.ts. `secondary` and `neutral` are deliberately
// desaturated here, which makes them poor chart series colors — that is why
// getChartColor() keeps its own categorical ramp instead of reusing them.
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
