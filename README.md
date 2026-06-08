# Stone Age IoT Platform

A comprehensive, single-binary IoT and Event-Driven platform built on [PocketBase](https://pocketbase.io/). It integrates multi-tenancy, NATS messaging, and Nebula overlay networks into a unified management console.

## 🏗 Architecture

The platform is designed as a **Single Deployment Unit**:
*   **Backend**: Go (1.25+) extending PocketBase.
*   **Frontend**: Vue 3 + TypeScript (Embedded in the binary).
*   **Database**: SQLite (managed by PocketBase).

### Core Components

1.  **Multi-Tenancy**: Built-in organization switching. Data isolation is enforced via PocketBase API Rules.
2.  **Infrastructure Provisioning**:
    *   **NATS**: Automatically provisions Accounts, Users, and Roles when Organizations are created.
    *   **Nebula**: Automatically creates Certificate Authorities (CAs) and manages Host certificates/keys.
3.  **Thing Modeling**: Declarative device contracts composed of three collections:
    *   **Thing Types** (`thing_types`) define a subject prefix and a set of Operations.
    *   **Operations** (`thing_type_operations`) declare a capability (`publish` / `subscribe` / `request` / `reply`), a subject suffix, and an optional Message Schema.
    *   **Message Schemas** (`message_schemas`) are versioned JSON Schema documents (namespace / name / semver) that describe operation payloads. The console includes a visual schema builder and an "infer from sample" tool.
4.  **Edge / Leaf Nodes**: Each edge site is a `leaf_nodes` record ("a special thing" with one server-provisioned NATS user). The separate **`leaf-sync`** agent runs on the edge, authenticates as the leaf node, and mirrors its organization's config collections into a local NATS leaf node's JetStream KV. See [`cmd/leaf-sync/README.md`](./cmd/leaf-sync/README.md).
5.  **Audit Logging**: comprehensive tracking of all create/update/delete/auth events.
6.  **Embedded UI**: The frontend is compiled and embedded directly into the Go binary using `embed.FS`, served via a custom SPA fallback handler.

---

## 🚀 Getting Started

### Prerequisites
*   Go 1.25+
*   Node.js 20+ (for building the UI)

### 1. Build the Frontend
Before building the Go binary, you must generate the frontend assets.

```bash
cd ui
npm install
npm run build
```
*This compiles the Vue application into the `pb_public/` directory at the project root.*

### 2. Build the Backend
```bash
# From the root directory
go build -o stone-age main.go
```

### 3. Run the Platform
```bash
./stone-age serve
```
Access the dashboard at: `http://localhost:8090`

### Edge Agent (Optional)
The edge agent is a separate, lean binary built from the same repo — it runs on edge boxes, not the central server:
```bash
go build -o leaf-sync ./cmd/leaf-sync
```
See [`cmd/leaf-sync/README.md`](./cmd/leaf-sync/README.md) for the full edge deployment flow.

---

## ⚙️ Configuration

The application looks for a `config.yaml` in the current directory or `/etc/stone-age/`.

**Default Configuration (`config.yaml`):**
```yaml
tenancy:
  organizations_collection: "organizations"
  memberships_collection: "memberships"
  invites_collection: "invites"
  invite_expiry_days: 7

nats:
  server_url: "nats://localhost:4222"
  operator_name: "stone-age.io"
  default_limits:
    max_connections: 10
    max_subscriptions: 50
    max_payload: 1048576 # 1MB

nebula:
  default_ca_validity_years: 10

audit:
  log_console: false

branding:
  dir: ""   # set to a host directory to enable the operator branding overlay
```

You can also use Environment Variables with the prefix `STONE_AGE_`:
*   `STONE_AGE_NATS_SERVER_URL="nats://10.0.0.1:4222"`
*   `STONE_AGE_TENANCY_LOG_TO_CONSOLE=true`

### Branding Overlay (Optional)

Operators can re-skin the console without rebuilding the binary. Point `branding.dir` at a directory on the host containing any of:

| File              | Purpose                                                                 |
| ----------------- | ----------------------------------------------------------------------- |
| `branding.json`   | `{ "appName": "...", "logo": "logo.svg" }` — overrides shown in the UI |
| `logo.svg`        | Brand mark used on login, sidebar, and mobile header                    |
| `theme.css`       | Override DaisyUI v4 CSS custom properties for `[data-theme=light/dark]` |

The platform serves the directory at `/branding/*` and the frontend picks up overrides on boot. Missing files fall back to embedded defaults.

A starting template lives at [`branding.example/`](./branding.example/) — copy it somewhere on the host, edit, and point `branding.dir` at the result:

```bash
cp -r branding.example /etc/stone-age/branding
# edit /etc/stone-age/branding/{branding.json,theme.css,logo.svg}
# then in config.yaml:
#   branding:
#     dir: /etc/stone-age/branding
```

For per-theme logo art (different SVG for light vs. dark), the example `theme.css` documents a CSS-only swap pattern using the `.brand-logo-img` class hook.

---

## 🛠 Development Workflow

For active development, run the backend and frontend separately to enable Hot Module Replacement (HMR).

**Terminal 1 (Backend):**
```bash
go run main.go serve
```

**Terminal 2 (Frontend):**
```bash
cd ui
npm run dev
```
*   Frontend: `http://localhost:5173` (Proxies API requests to backend)
*   Backend Admin: `http://localhost:8090/_/`

---

## 📦 Schema Management

The platform uses a simple, declarative approach to schema management:

*   **`schema.json`**: The single source of truth for all PocketBase collections. This file is embedded in the binary and imported on every startup.
*   **Extend Mode**: The import uses `deleteMissing=false`, meaning it adds/updates collections from the schema but preserves any collections created by packages that aren't in the file.

### Updating the Schema

1.  Make changes in the PocketBase Admin UI (`http://localhost:8090/_/`)
2.  Export collections from **Settings → Export collections**
3.  Replace `schema.json` with the exported file
4.  Commit the updated `schema.json`
5.  Rebuild the binary - the new schema is embedded automatically

---

## 🧠 Hooks & Glue Logic

The `main.go` file contains critical business logic hooks:

*   **`OnBootstrap`**: Imports `schema.json` to ensure all collections and fields are up to date.
*   **`OnRecordAfterCreateSuccess` (Organizations)**:
    1.  Creates a **NATS Account** specifically for that organization.
    2.  Creates a **Nebula CA** specifically for that organization.
*   **`OnRecordAfterCreateSuccess` (Leaf Nodes)**: Mints the edge node's **NATS user** so the `leaf-sync` agent can authenticate as the leaf node (`hooks/leaf_node_provisioning.go`).
*   **Leaf operator-JWT route**: `GET /api/leaf/operator-jwt` serves the operator JWT to leaf-node-authenticated callers, so the `nats_system_operator` collection can stay superuser-only (`hooks/leaf_node_routes.go`).
*   **`OnServe`**: Registers the custom router handler that serves the embedded `pb_public` filesystem, handling SPA history mode (redirecting unknown routes to `index.html`).

