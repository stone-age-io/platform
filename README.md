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
go build -o stone-age .
```

### 3. Initial Setup (first deployment only)

Run these three in order — **the order matters**:

```bash
./stone-age superuser upsert admin@example.com 'your-password'
```
```bash
./stone-age migrate up
```
```bash
./stone-age bootstrap --email admin@example.com --org "System" --operator-org "your-company"
```

1.  **`superuser upsert`** creates the PocketBase superuser that owns the admin
    panel at `/_/`. This is also what seeds the NATS operator and `$SYS` records.
2.  **`migrate up`** imports `schema.json`, creating the platform's own
    collections and fields. (Starting the server once with `serve` applies them
    too, but the explicit command is better in a deploy script.)
3.  **`bootstrap`** provisions the System organization, the platform operator
    user, and the operator organization, and links the `$SYS` NATS records to the
    System org. It prompts for anything not passed as a flag; prefer the prompt
    over `--password`, which is visible in shell history and the process list
    (`STONE_AGE_BOOTSTRAP_PASSWORD` also works for automation).

**Why the order matters:** `bootstrap` writes platform flags — `is_operator`,
`is_system_org`, `is_operator_org` — that exist only after `schema.json` has been
imported. PocketBase silently discards a write to a field that does not exist, so
running `bootstrap` before `migrate up` would print "Bootstrap complete!" while
leaving you with **no platform operator at all**. `bootstrap` now refuses to run
in that state and tells you what to do.

Granting operator status is deliberately not possible through the API — only this
command or the admin panel can do it.

### 4. Run the Platform
```bash
./stone-age serve
```
Access the console at `http://localhost:8090`, and the PocketBase admin panel at
`http://localhost:8090/_/`.

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

## 🔐 Authorization Tests

Tenant isolation and privilege boundaries are enforced **entirely** by the
PocketBase API rules in `schema.json` — the NATS and Nebula packages contain no
tenancy logic. Those rules are plain strings with no compiler and no type
checker behind them, so there is one script that exercises the ones that matter
against a real server:

```bash
./scripts/test-authz.sh
```

It builds the binary, stands up a throwaway database, runs 84 checks, and tears
everything down; `pb_data/` is never touched. Run it after any change to a
`listRule` / `viewRule` / `createRule` / `updateRule` / `deleteRule`, and add a
check whenever you add a rule (bump `EXPECTED_CHECKS`, which catches a suite that
exits early). Requires `go`, `curl`, and `node`.

Two things to know when reading a failure: PocketBase answers **404, not 403**,
when an update rule rejects a request (deliberately — it avoids confirming the
record exists), and every "cannot do X" check is paired with a "can still do Y"
check on the same record, so a rule that simply denies everything fails the
suite rather than passing it.

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

*   **`schema.json`**: The single source of truth for all PocketBase collections, embedded in the binary.
*   **Applied by migrations — not on every startup.** Each file in `migrations/` calls `ImportCollectionsByMarshaledJSON(SchemaJSON, false)`, and PocketBase runs each migration exactly once per database. On a fresh database the first migration imports everything; on an existing database, only *new* migration files run.
*   **Extend Mode**: The import uses `deleteMissing=false`, meaning it adds/updates collections from the schema but preserves any collections created by packages that aren't in the file.

### Updating the Schema

1.  Make changes in the PocketBase Admin UI (`http://localhost:8090/_/`)
2.  Export collections from **Settings → Export collections**
3.  Replace `schema.json` with the exported file
4.  **Add a new file to `migrations/`** that re-imports the schema — copy an existing `schema_update_*.go`, rename it, and describe the change in its doc comment
5.  Commit both the updated `schema.json` and the new migration
6.  Rebuild the binary - the new schema is embedded automatically

> ⚠️ Step 4 is not optional. Every existing deployment has already run the
> existing migrations, so editing `schema.json` alone changes *fresh* databases
> only and is a silent no-op everywhere else. Skipping it is the easiest way to
> ship a schema change that works on your laptop and does nothing in production.

---

## 🧠 Hooks & Glue Logic

The `main.go` file contains critical business logic hooks:

*   **Migrations** (`migrations/`): Import `schema.json` to bring collections and fields up to date. Each runs once per database — see [Schema Management](#-schema-management).
*   **`OnRecordAfterCreateSuccess` (Organizations)**:
    1.  Creates a **NATS Account** specifically for that organization.
    2.  Creates a **Nebula CA** specifically for that organization.
*   **`OnRecordAfterCreateSuccess` (Leaf Nodes)**: Mints the edge node's **NATS user** so the `leaf-sync` agent can authenticate as the leaf node (`hooks/leaf_node_provisioning.go`).
*   **Leaf operator-JWT route**: `GET /api/leaf/operator-jwt` serves the operator JWT to leaf-node-authenticated callers, so the `nats_system_operator` collection can stay superuser-only (`hooks/leaf_node_routes.go`).
*   **`OnServe`**: Registers the custom router handler that serves the embedded `pb_public` filesystem, handling SPA history mode (redirecting unknown routes to `index.html`).

