# Stone Age Console (UI)

The official web management console for the Stone Age Platform. Built with Vue 3, TypeScript, and Tailwind CSS.

## ⚡ Tech Stack

*   **Framework**: [Vue 3](https://vuejs.org/) (Composition API + `<script setup>`)
*   **Build Tool**: [Vite](https://vitejs.dev/)
*   **State Management**: [Pinia](https://pinia.vuejs.org/)
*   **Styling**: [Tailwind CSS](https://tailwindcss.com/) + [DaisyUI](https://daisyui.com/)
*   **Routing**: [Vue Router](https://router.vuejs.org/)
*   **Maps**: [Leaflet](https://leafletjs.com/)
*   **Backend SDK**: [PocketBase JS SDK](https://github.com/pocketbase/js-sdk)

---

## 📂 Project Structure

```text
src/
├── components/
│   ├── layout/       # App shell (Sidebar, Header)
│   └── ui/           # Generic UI (BaseCard, ResponsiveList)
├── composables/      # Shared logic (Pagination, Toast, Map)
├── stores/           # Global State (Auth, UI preferences)
├── types/            # TypeScript interfaces (Synced with PB Schema)
├── utils/            # Helpers (Formatters, PB Client)
└── views/            # Page definitions
    ├── auth/         # Login, Invite Acceptance
    ├── dashboard/    # Main overview
    ├── map/          # Global location map
    ├── nats/         # NATS infrastructure management
    ├── nebula/       # Nebula VPN management
    └── ...
```

---

## 🧩 Key Architectural Concepts

### 1. Backend-Enforced Security
The frontend is "dumb" regarding security. We do not filter sensitive data in JavaScript. We rely on PocketBase **API Rules** to ensure users only receive data for their active Organization.
*   *Developer Note:* Do not write `items.filter(i => i.org === currentOrg)`. Trust the API response.

### 2. Reactive Organization Context
The `auth` store manages the `currentOrgId`. When a user switches organizations in the header:
1.  The User record is updated in the backend (`current_organization`).
2.  A global `organization-changed` event is dispatched.
3.  Views listen to this event and reload data automatically (e.g., `usePagination` resets, Map re-fetches).

### 3. Responsive Lists
We use a custom `ResponsiveList.vue` component that renders a **Table** on desktop and **Card Grid** on mobile.
*   **Usage:** Define columns with `mobileLabel` to automatically handle responsive layouts without writing duplicate HTML.

### 4. Infrastructure "Provisioning"
Forms for **NATS Users** and **Nebula Hosts** are designed as "Provisioning" flows rather than simple CRUD.
*   We collect high-level intent (Hostname, Role, Network).
*   The Backend Hook handles the complex cryptography (Key generation, IP allocation).
*   The UI then presents the generated credentials/config for download.

---

## 🏃‍♂️ Development

### Setup
```bash
npm install
```

### Dev Server
```bash
npm run dev
```
This starts the Vite dev server at `http://localhost:5173`.
*   **Proxying**: Requests to `/api` and `/_` are automatically proxied to `http://127.0.0.1:8090` (configured in `vite.config.ts`). Ensure your Go backend is running!

### Type Generation
(Optional) If you change the PocketBase schema, update `src/types/pocketbase.ts` to maintain type safety.

### Building for Production
```bash
npm run build
```
This generates the `../pb_public` directory. The Go `main.go` file embeds this directory into the final binary.

---

## 🎨 Themeing

Themeing is handled by DaisyUI.
*   **Toggle**: Controlled via `useUIStore`.
*   **Persistence**: Saved to `localStorage`.
*   **Maps**: The `useMap` composable watches the theme store and dynamically swaps map tiles (Carto Dark vs. OSM Light) to match the UI.
