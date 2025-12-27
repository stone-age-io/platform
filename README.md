# Stone Age IoT Platform

A comprehensive, single-binary IoT and Event-Driven platform built on [PocketBase](https://pocketbase.io/). It integrates multi-tenancy, NATS messaging, and Nebula overlay networks into a unified management console.

## 🏗 Architecture

The platform is designed as a **Single Deployment Unit**:
*   **Backend**: Go (1.23+) extending PocketBase.
*   **Frontend**: Vue 3 + TypeScript (Embedded in the binary).
*   **Database**: SQLite (managed by PocketBase).

### Core Components

1.  **Multi-Tenancy**: Built-in organization switching. Data isolation is enforced via PocketBase API Rules.
2.  **Infrastructure Provisioning**:
    *   **NATS**: Automatically provisions Accounts, Users, and Roles when Organizations are created.
    *   **Nebula**: Automatically creates Certificate Authorities (CAs) and manages Host certificates/keys.
3.  **Audit Logging**: comprehensive tracking of all create/update/delete/auth events.
4.  **Embedded UI**: The frontend is compiled and embedded directly into the Go binary using `embed.FS`, served via a custom SPA fallback handler.

---

## 🚀 Getting Started

### Prerequisites
*   Go 1.23+
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
```

You can also use Environment Variables with the prefix `STONE_AGE_`:
*   `STONE_AGE_NATS_SERVER_URL="nats://10.0.0.1:4222"`
*   `STONE_AGE_TENANCY_LOG_TO_CONSOLE=true`

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

## 🧠 Hooks & Glue Logic

The `main.go` file contains critical business logic hooks:

*   **`OnBootstrap`**: Injects the `organization` relation field into NATS and Nebula collections if missing.
*   **`OnRecordAfterCreateSuccess` (Organizations)**:
    1.  Creates a **NATS Account** specifically for that organization.
    2.  Creates a **Nebula CA** specifically for that organization.
*   **`OnServe`**: Registers the custom router handler that serves the embedded `pb_public` filesystem, handling SPA history mode (redirecting unknown routes to `index.html`).
