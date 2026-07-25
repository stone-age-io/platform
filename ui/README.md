# Stone Age Console (UI)

The official web management console for the Stone Age Platform. A "Single Pane of Glass" for IoT asset management, Edge orchestration, Real-time Digital Twins, and Operator Badging.

## ⚡ Tech Stack

*   **Framework**: [Vue 3](https://vuejs.org/) (Composition API + `<script setup>`)
*   **Build Tool**: [Vite](https://vitejs.dev/)
*   **State Management**: [Pinia](https://pinia.vuejs.org/)
*   **Styling**: [Tailwind CSS](https://tailwindcss.com/) + [DaisyUI](https://daisyui.com/)
*   **Routing**: [Vue Router](https://router.vuejs.org/)
*   **Maps**: [Leaflet](https://leafletjs.com/) (geospatial + floor-plan overlays)
*   **Charts**: [ECharts](https://echarts.apache.org/) via `vue-echarts` (Visualizer chart/gauge widgets)
*   **QR / Scanning**: `qrcode` for badge rendering, camera-based scanner widget
*   **Real-time / Messaging**: [NATS.js](https://github.com/nats-io/nats.js) (Modular: Core, KV, JetStream)
*   **Backend SDK**: [PocketBase JS SDK](https://github.com/pocketbase/js-sdk)

---

## 📂 Project Structure

```text
src/
├── components/
│   ├── common/        # ConfirmDialog, DebugPanel, ErrorBoundary, JsonViewer, LoadingState, ResponseModal, KeyboardShortcutsModal
│   ├── dashboard/     # Visualizer shell (Grid, Sidebar, Tree, WidgetContainer, Add/Configure modals, VariableBar, GaugeZone/Threshold editors)
│   │   └── config/    # Per-widget config panels (Button, Chart, Console, Gauge, Kv, KvTable, Map, Markdown, PocketBase, Publisher, Scanner, Slider, Stat, Status, Switch, Text)
│   ├── layout/        # App shell (MainLayout, AppHeader, AppSidebar)
│   ├── locations/     # LocationMapViz, LocationMapDrawer
│   ├── map/           # FloorPlanMap (image-overlay indoor positioning)
│   ├── nats/          # KvDashboard, LiveMessageStream
│   ├── things/        # SchemaBuilder (visual JSON Schema editor used by MessageSchemaFormView)
│   ├── ui/            # Generic UI (BaseCard, ResponsiveList)
│   └── widgets/       # Visualizer widget implementations
│       └── map/       # Marker editors & detail panels for the MapWidget
├── composables/       # Shared logic
│   ├── useNatsKv.ts / useNatsKvWatcher.ts  # NATS KV read + reactive watch
│   ├── useSubscriptionManager.ts           # Core NATS subscription lifecycles
│   ├── useJetStreamManager.ts              # Stream / consumer ops
│   ├── useLeafletMap.ts / useMap.ts / useFloorPlan.ts
│   ├── useWidgetOperations.ts / useWidgetForm.ts / useThresholds.ts / useSwitchState.ts
│   ├── useKeyboardShortcuts.ts / useDesignTokens.ts
│   ├── usePagination.ts / useValidation.ts / useConfirm.ts / useToast.ts
├── stores/            # Pinia state
│   ├── auth.ts        # PocketBase session, memberships, current org, NATS identity
│   ├── nats.ts        # NATS WebSocket connection + status
│   ├── dashboard.ts   # Dashboard layout, widgets, variables, kiosk mode
│   ├── widgetData.ts  # Live KV values shared across widgets
│   └── ui.ts          # Theme, sidebar, global UI
├── types/             # TypeScript interfaces (synced with PB schema)
└── views/             # Page definitions
    ├── admin/         # Operator-only organization management
    ├── audit/         # Audit log viewer (pb-audit)
    ├── auth/          # Login, invitation acceptance
    ├── badge/         # Badge identity card (QR + NKey + live status)
    ├── dashboard/     # Visualizer (also reused as the Badge dashboard)
    ├── leaf_nodes/    # Edge nodes (leaf_nodes records) + NATS identity / Nebula linking
    ├── locations/     # Locations + LocationTypes
    ├── nats/          # Account, Users, Roles, Imports, Exports, Streams, KV Buckets
    ├── nebula/        # CA, Networks, Hosts
    ├── organization/  # Members, Invitations, Member detail
    ├── settings/      # User profile & NATS connection config
    └── things/        # Things, Thing Types, Operations, Message Schemas
```

---

## 🧩 Key Features & Architecture

### 1. Multi-Tenancy & Context
The application is fully multi-tenant.
*   **Organization Switching:** Changing the organization in the sidebar updates the user's context in the backend.
*   **Reactive Reloads:** A global event triggers all active views to reload data relevant to the selected organization.
*   **Role Gates:** Route meta (`requiresAuth`, `requiresOperator`, `requiresRole`) drives navigation; super-users and operators get cross-org admin screens, owners/admins see org-scoped config, members see the Visualizer and their own Things/Locations, and badge users are boxed into the badge experience.
*   **Permissions:** The UI relies on backend API rules. We do not filter data for security in JS; we display what the API returns.

### 2. Infrastructure as Data
We manage the connectivity layer as first-class entities:
*   **NATS Accounts & Users:** Manage per-org accounts, scoped users, and reusable roles.
*   **NATS Imports / Exports:** Publish subjects/streams out of an account as exports, and subscribe to exports from peer accounts as imports — enabling cross-tenant service exchange and shared data planes. Each import/export has its own list/detail/form flow under `views/nats/`.
*   **JetStream:** Browse and manage Streams and KV Buckets (create, inspect, edit) directly from the console, backed by `useJetStreamManager`.
*   **Nebula:** Manage Certificate Authorities, Networks, and Hosts.
*   **Provisioning:** The UI facilitates the creation of cryptographic identities (NKeys, Certificates) via backend hooks and allows downloading `.creds` and `config.yaml` files directly.
*   **Edge / Leaf Nodes:** Provision and manage `leaf_nodes` (edge sites) under `views/leaf_nodes/`. Each gets a server-minted NATS user and an optional Nebula host link; the detail view mirrors the Thing layout — identity, a Connectivity card (NATS role with reassignment, permission overrides, `.creds` download, plus Nebula hostname/IP/config), and synced-collection selection. The off-box [`leaf-sync`](../cmd/leaf-sync/README.md) agent mirrors the org's config into the edge's local NATS KV.

### 3. Thing Modeling
Things aren't just inventory records — they declare a messaging contract via three related collections managed under `views/things/`:

*   **Thing Types** (`ThingTypeFormView` / `ThingTypeListView`) own a subject prefix template (supporting `{org}`, `{location}`, `{thing}`, `{thing_type_code}` tokens) and a list of Operations.
*   **Operations** (`ThingTypeOperationFormView` / `ThingTypeOperationListView`) declare a capability (`publish` / `subscribe` / `request` / `reply`), a subject suffix appended to the Thing Type's prefix, and an optional Message Schema describing the payload.
*   **Message Schemas** (`MessageSchemaFormView` / `MessageSchemaListView`) are versioned JSON Schema documents identified by `namespace`, `name`, and semver `version` — each version is a separate record. The form offers a **SchemaBuilder** visual editor, a raw JSON view, and an **Infer from sample** action that generates a starting schema from a pasted example message.
*   **Quick-add flow:** Related-record selects (Operations on a Thing Type; Message Schema on an Operation) expose a `+` button that opens the target form in an embedded modal, so you can author the full graph without leaving the page.

### 4. Live State / Digital Twin
We bridge the gap between **SQL Metadata** (PocketBase) and **Live State** (NATS JetStream).
*   **Direct Connectivity:** The browser connects directly to the NATS cluster via WebSocket (`wss://`).
*   **Hybrid Auth:** Connection URLs are stored in local storage; Credentials (`.creds`) are fetched securely from the PocketBase user record.
*   **Two buckets per org**, split by who owns the data — defined once in
    [`utils/twin.ts`](src/utils/twin.ts):

    | Bucket | Written by | Flows |
    |---|---|---|
    | `twin` | the device | edge → hub |
    | `twin_desired` | operators | hub → edge |

    One writer per bucket, one direction per bucket. A single bucket written from
    both ends does not merely pick a loser on a conflict — it *oscillates*, the two
    values swapping across the link indefinitely. Encoding the owner in the key
    instead (`thing.S01.state.temp`) buys the same safety but taxes every key in
    the system and leaves a mistyped segment silently unsynced.
*   **Keys** are `<kind>.<code>.<prop>` by convention (`thing.S01.temp`), and the
    two buckets pair on the same key. Direction lives in the bucket, so keys carry
    no sync bookkeeping. Values may be primitives or whole JSON objects.
*   **The twin view is `KvDashboard` with `desiredBucket` set** — the same browser
    used for every other bucket, not a second one. It gains a Reported/Desired
    toggle in the detail pane, per-row twin markers, and a read-only reported side
    (the edge overwrites it, so an edit button would be a lie). Tree/flat,
    filtering, revision history and the responsive detail drawer all come from the
    browser itself. Omit the prop — as *NATS → KV Buckets* does — and it behaves
    exactly as it always has.
*   **The marker shows the difference, not a word for it.** A row whose desired
    value is a primitive renders it inline — `TRUE → false` — so the answer is on
    the row. Object-valued rows fall back to a `differs` badge (the value will not
    fit) and the detail pane lists each differing path as a reported/desired pair.
    Agreement is a quiet `✓`. "Differs on: mode" was the same width and sent the
    reader off to look both values up. Markers are right-anchored in both views,
    so they line up in a column instead of landing wherever the value ends.
    All / Desired / Differs filters sit with the tree controls, and the `N differs`
    badge in the card header is itself the Differs toggle.
*   **History follows the toggle.** Revisions come from whichever bucket the
    Reported/Desired switch points at — reading `twin` while the pane displayed a
    desired value produced a list that silently described the other side. The
    pane also stamps each side with when it was last written (`Set 2 minutes ago
    · rev 386`); NATS carries `created` on every entry and the revision number
    alone answers "how many times", never "how long ago".
*   **A desired value is a partial assertion, not a replacement.** `twinDrift()`
    checks only the keys present in the desired value, so `{arm: "armed"}` against
    a twelve-field object asserts one field and stays quiet about the rest. Full
    equality would flip every old assertion to "differs" the day a device reports
    one new field. Subset semantics apply to objects only — arrays and scalars
    compare exactly.
*   **Pair desired with an echo, never with a measurement.** Desired belongs on a
    property the device echoes back to acknowledge an instruction (`setpoint`,
    `mode`) — those converge exactly. Desired `temp = 20` against reported
    `temp = 20.3` compares an instruction to a continuous reading and differs
    forever. "What if desired is a range?" is a threshold over *reported* state:
    a rule-router rule, not a desired value. Do not add operators to `twinDrift()`.
*   **`differs`, never `pending`.** Nothing in the platform applies desired values
    to devices; there is no control loop. The UI reports that two values disagree
    and does not predict what happens next. If rule-router ever applies desired
    state, the stronger word becomes earned.
*   **Edge sync:** where a site runs a NATS leaf node, [`leaf-sync`](../cmd/leaf-sync/README.md)
    gives it a server-maintained JetStream **mirror** of `twin_desired` and relays
    its local `twin` up to the hub — so the edge keeps writing reported state, and
    reading the last-known desired state, straight through a WAN outage.
*   **Live Message Stream:** The `LiveMessageStream` component tails core NATS subjects for debugging wire traffic.

### 5. Visualizer (Dashboard)
The Visualizer is the home view and is also embedded as the Badge dashboard. It is a per-user, per-org widget canvas backed by `stores/dashboard.ts`.

*   **Grid Layout:** Draggable/resizable grid with a sidebar tree of saved dashboards (`DashboardSidebar`, `DashboardTree`, `DashboardGrid`).
*   **Widgets:** `button`, `switch`, `slider`, `publisher`, `scanner` (camera / QR), `kv`, `kvtable`, `chart`, `gauge`, `stat`, `status`, `text`, `markdown`, `console`, `map` (geospatial + floorplan with dynamic marker panels), and `pocketbase` (live PB collection views).
*   **Configuration:** Each widget type has a dedicated config panel under `components/dashboard/config/` plus shared editors for gauge zones, thresholds, and map markers.
*   **Variables:** Dashboards expose per-scope variables via `VariableBar` / `VariableEditorModal`, letting one layout parametrize subjects, topics, and bucket keys.
*   **Kiosk & Shortcuts:** Full-screen kiosk mode, keyboard shortcuts (`useKeyboardShortcuts`), and a debug panel for inspecting NATS traffic and widget state.
*   **Badge Mode:** When rendered at `/badge/dashboard`, the Visualizer strips chrome (no kiosk/debug/grid selector) and restricts the widget palette to a badge-safe set (`button`, `switch`, `slider`, `publisher`, `kv`, `kvtable`, `text`, `status`, `stat`, `scanner`).

### 6. Operator Badge
The `/badge` experience turns a browser into a wearable-style operator identity card.

*   **Identity Card (`BadgeView`):** Shows the user's name, email, organization, role, member-since date, NATS connection status, and a QR code encoding the user's NATS NKey public key for fast pairing/scanning.
*   **Badge Dashboard:** The Visualizer rendered in badge mode provides a touch-friendly control surface for floor operators.
*   **Access Model:** Users with the `badge` membership role are automatically routed into `/badge` and restricted to `/badge/*` and `/settings`. Non-badge users can still preview their own card at `/my-badge`.

### 7. Responsive Design
We prioritize simplicity and maintainability:
*   **ResponsiveList:** A core component that renders as a high-density Table on desktop and a Card Grid on mobile.
*   **Theming:** Dark/Light mode support (including map tiles and design tokens exposed via `useDesignTokens`).
*   **Map/Floorplans:** Geospatial visualization for global views, and image overlay mapping for indoor positioning.

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
*   **Frontend:** `http://localhost:5173`
*   **Backend Proxy:** API requests (`/api`, `/_`) are proxied to `http://127.0.0.1:8090`. Ensure the Go backend is running (`go run main.go serve` from the repo root).

### Type-check only
```bash
vue-tsc
```

### Building for Production
```bash
npm run build
```
This compiles the application into `../pb_public`. The Go binary embeds this directory, allowing the entire platform to be deployed as a single executable.

---

## 🔌 NATS Connection Setup

To enable Live State, Visualizer live widgets, and Badge live status:

1.  **NATS Server:** Ensure your NATS server has WebSockets enabled in its config (`websocket { port: 9222 }`).
2.  **User Settings:**
    *   Go to **Settings** in the UI.
    *   Add your NATS WebSocket URL (e.g., `ws://localhost:9222`).
    *   Select your **Operational Identity** (links your web user to a NATS User/Creds file).
    *   Enable **Auto-connect**.
3.  **Live State:** Navigate to a Location or Thing with a valid `Code` to see the live KV dashboard, or drop live widgets onto a Visualizer dashboard.
