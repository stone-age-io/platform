# CLAUDE.md - Stone Age IoT Platform

## Project Overview

Stone Age IoT Platform is a single-binary IoT and Event-Driven management platform combining:
- Go backend extending PocketBase
- Embedded Vue 3 frontend
- SQLite database
- NATS messaging infrastructure integration
- Nebula overlay network management
- Edge nodes: a NATS leaf node + local KV mirror of an org's config, kept in sync by the `leaf-sync` agent

## Tech Stack

### Backend
- **Go 1.25.0** with PocketBase 0.35.0
- **Cobra** for CLI commands (bootstrap, NATS management)
- **Viper** for configuration management
- **Key libraries**: pb-audit, pb-tenancy, pb-nats, pb-nebula, nats-io/jwt, slackhq/nebula

### Frontend
- **Vue 3** (Composition API + `<script setup>`)
- **Vite 6.0** build tool
- **Pinia 2.1** state management
- **Vue Router 4.2** routing
- **Tailwind CSS 3.4 + DaisyUI 4.4** styling (light/dark themes)
- **TypeScript 5.3**
- **NATS WebSocket** (@nats-io/nats-core, jetstream, kv)
- **Leaflet 1.9** for maps
- **ECharts 5.5 + vue-echarts** for charts
- **grid-layout-plus** for dashboard grid layout
- **@vueuse/core** for reactive utilities
- **date-fns** for date formatting
- **jsonpath-plus** for JSON path extraction in widget data
- **marked** for Markdown rendering
- **PocketBase JS SDK** for API client

### Database
- SQLite (managed by PocketBase, stored in `pb_data/`)

## Directory Structure

```
platform/
├── main.go                 # Backend entry point
├── go.mod, go.sum          # Go dependencies
├── config.yaml             # Application configuration
├── schema.json             # PocketBase collection schema
├── hooks/                  # Server-side hooks: org + leaf-node provisioning, operator-jwt route
├── migrations/             # Schema migrations (re-import schema.json)
├── cmd/leaf-sync/          # Edge agent binary (separate from the server; PocketBase → local NATS KV)
├── internal/leafsync/      # leaf-sync engine (pbclient, sync, kv, bootstrap)
├── ui/                     # Frontend Vue 3 application
│   ├── public/             # PWA assets (manifest, service worker, icons)
│   ├── src/
│   │   ├── main.ts         # Vue app initialization
│   │   ├── App.vue         # Root component
│   │   ├── components/     # UI components
│   │   │   ├── common/     # Reusable UI (ConfirmDialog, ErrorBoundary, etc.)
│   │   │   ├── layout/     # AppHeader, AppSidebar, MainLayout
│   │   │   ├── ui/         # Base UI primitives (BaseCard, ResponsiveList)
│   │   │   ├── dashboard/  # Dashboard grid, widget containers, variables
│   │   │   │   └── config/ # 18 widget configuration form components
│   │   │   ├── widgets/    # 14 widget type components
│   │   │   │   └── map/    # Map marker sub-components (detail, kv, publish, switch, text)
│   │   │   ├── map/        # FloorPlanMap component
│   │   │   ├── nats/       # NATS-specific components (KvDashboard, LiveMessageStream)
│   │   │   └── locations/  # Location map visualization
│   │   ├── composables/    # Vue composables (useNatsKv, useLeafletMap, etc.)
│   │   ├── stores/         # Pinia stores (auth, dashboard, nats, ui, widgetData)
│   │   ├── router/         # Vue Router with auth guards
│   │   ├── types/          # TypeScript interfaces
│   │   ├── utils/          # Utility functions
│   │   └── views/          # Route page components
│   ├── vite.config.ts      # Vite configuration
│   ├── tailwind.config.js  # Tailwind configuration
│   └── package.json        # Frontend dependencies
├── pb_public/              # Compiled frontend (generated)
└── pb_data/                # Runtime database (generated)
```

## Build & Run Commands

### Development
```bash
# Terminal 1: Backend
go run main.go serve

# Terminal 2: Frontend with HMR
cd ui
npm install
npm run dev
```
- Backend: http://localhost:8090
- Frontend dev: http://localhost:5173 (proxies to backend)
- PocketBase Admin: http://localhost:8090/_/

### Bootstrap (Initial Setup)
```bash
# Interactive setup for first-time deployment
./stone-age bootstrap --email admin@example.com --org "System"
```

### Production Build
```bash
# Build frontend
cd ui && npm run build

# Build Go binary
go build -o stone-age main.go

# Build the edge agent (separate lean binary — runs on edge boxes, not the server)
go build -o leaf-sync ./cmd/leaf-sync

# Run
./stone-age serve
```

See [cmd/leaf-sync/README.md](cmd/leaf-sync/README.md) for the edge deployment flow.

### NPM Scripts
- `npm run dev` - Vite dev server with HMR
- `npm run build` - TypeScript check + production build to `../pb_public`
- `npm run preview` - Preview production build

## Configuration

### config.yaml
Located at project root. Key sections:
- `tenancy` - Multi-tenant settings (collections, invite expiry)
- `nats` - NATS server URL, operator name, default limits
- `nebula` - Nebula CA/network/host settings
- `audit` - Audit logging configuration

### Environment Variables
Prefix: `STONE_AGE_`
```bash
STONE_AGE_NATS_SERVER_URL="nats://localhost:4422"
STONE_AGE_TENANCY_LOG_TO_CONSOLE=true
```

### Config Priority (highest to lowest)
1. Environment variables (`STONE_AGE_*`) - override everything
2. CLI flag: `--config /path/to/config.yaml`
3. Current directory: `./config.yaml`
4. System: `/etc/stone-age/config.yaml`
5. Hardcoded defaults

## Code Conventions

### Frontend (Vue/TypeScript)

**Components**: Use Composition API with `<script setup lang="ts">`
```vue
<script setup lang="ts">
import { ref, computed } from 'vue'
const items = ref([])
</script>
```

**Pinia Stores**: Composition style with `defineStore`
```typescript
export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const isAuthenticated = computed(() => !!user.value)
  return { user, isAuthenticated }
})
```

**Composables**: `use` prefix, handle lifecycle cleanup
```typescript
export function useNatsKv() {
  onMounted(() => { /* setup */ })
  onUnmounted(() => { /* cleanup */ })
}
```

**Naming**:
- Components: PascalCase (`ConfirmDialog.vue`)
- Composables: camelCase with `use` prefix (`useNatsKv.ts`)
- Stores: camelCase (`auth.ts`)
- Views: End with `View` (`LoginView.vue`)

**Type Safety**: All types in `src/types/`, strict TypeScript enabled

### Backend (Go)

**Hooks Pattern**:
```go
app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error { /* init */ })
app.OnRecordAfterCreateSuccess("collection").BindFunc(func(e *core.RecordEvent) error { /* provision */ })
```

**Logging**: Structured with emoji prefixes

## Key Features

1. **Authentication & Multi-Tenancy** - PocketBase auth, OAuth2, organization switching, RBAC
2. **Dashboard/Visualizer** - Grid-based dashboards, 14 widget types, variable substitution
3. **NATS Integration** - Account/User/Role provisioning, real-time WebSocket connection
4. **Nebula Networks** - Certificate Authority, network, and host management
5. **Resource Inventory** - Things and Locations with type definitions and metadata
6. **Digital Twin** - Live state via NATS KV buckets, revision history
7. **Audit Logging** - Comprehensive audit trail with searchable viewer
8. **Maps** - Leaflet-based maps with floorplan overlays
9. **PWA** - Service worker, manifest, installable
10. **Keyboard Shortcuts** - Configurable keyboard shortcuts with modal reference
11. **Edge / Leaf Nodes** - `leaf_nodes` auth collection (a "special thing" with one nats_user, server-provisioned). The `leaf-sync` agent runs on the edge, authenticates as the leaf node, and mirrors its org's config collections into a NATS leaf node's local JetStream KV. Operator JWT served via the dedicated `GET /api/leaf/operator-jwt` route (nats_system_operator stays superuser-only).

## Testing

No automated test framework configured. Development relies on:
- HMR during `npm run dev`
- Browser DevTools
- PocketBase admin panel at `/_/`

## Important Files

- `main.go` - Backend entry, PocketBase setup, hooks, bootstrap command
- `hooks/leaf_node_provisioning.go` - Mints a leaf node's NATS user on create
- `hooks/leaf_node_routes.go` - `GET /api/leaf/operator-jwt` (leaf-node-authed)
- `cmd/leaf-sync/` + `internal/leafsync/` - Edge agent (config bootstrap + KV sync); see `cmd/leaf-sync/README.md`
- `ui/src/stores/auth.ts` - Authentication and organization context
- `ui/src/stores/nats.ts` - NATS WebSocket connection manager
- `ui/src/stores/dashboard.ts` - Dashboard state and persistence
- `ui/src/router/index.ts` - Route definitions with auth guards
- `ui/src/types/pocketbase.ts` - All PocketBase record type interfaces
- `ui/src/types/dashboard.ts` - Widget and dashboard type definitions
- `ui/src/utils/pb.ts` - PocketBase client singleton
- `config.yaml` - Application configuration
- `schema.json` - PocketBase collection definitions
