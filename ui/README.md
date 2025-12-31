# Stone Age Console (UI)

The official web management console for the Stone Age Platform. A "Single Pane of Glass" for IoT asset management, Edge orchestration, and Real-time Digital Twins.

## ⚡ Tech Stack

*   **Framework**: [Vue 3](https://vuejs.org/) (Composition API + `<script setup>`)
*   **Build Tool**: [Vite](https://vitejs.dev/)
*   **State Management**: [Pinia](https://pinia.vuejs.org/)
*   **Styling**: [Tailwind CSS](https://tailwindcss.com/) + [DaisyUI](https://daisyui.com/)
*   **Routing**: [Vue Router](https://router.vuejs.org/)
*   **Maps**: [Leaflet](https://leafletjs.com/)
*   **Real-time / Messaging**: [NATS.js](https://github.com/nats-io/nats.js) (Modular: Core, KV, JetStream)
*   **Backend SDK**: [PocketBase JS SDK](https://github.com/pocketbase/js-sdk)

---

## 📂 Project Structure

```text
src/
├── components/
│   ├── layout/       # App shell (Sidebar, Header)
│   ├── map/          # Leaflet & Floorplan visualizers
│   ├── nats/         # NATS specific UI (KvDashboard)
│   └── ui/           # Generic UI (BaseCard, ResponsiveList)
├── composables/      # Shared logic
│   ├── useNatsKv.ts  # NATS Key-Value wrapper
│   ├── useMap.ts     # Leaflet wrapper
│   ├── usePagination.ts # PocketBase CRUD pagination
│   └── ...
├── stores/           # Global State
│   ├── auth.ts       # PocketBase Session & Org Context
│   ├── nats.ts       # NATS WebSocket Connection Manager
│   └── ui.ts         # Theme & Sidebar state
├── types/            # TypeScript interfaces (Synced with PB Schema)
└── views/            # Page definitions
    ├── dashboard/    # Analytics & Overview
    ├── locations/    # Hierarchy, Maps, & Digital Twins
    ├── things/       # Device Inventory & Connectivity
    ├── nats/         # Infrastructure (Accounts/Users/Roles)
    ├── nebula/       # VPN Infrastructure (CAs/Hosts)
    └── settings/     # User Profile & Connection Config
```

---

## 🧩 Key Features & Architecture

### 1. Multi-Tenancy & Context
The application is fully multi-tenant.
*   **Organization Switching:** Changing the organization in the sidebar updates the user's context in the backend.
*   **Reactive Reloads:** A global event triggers all active views to reload data relevant to the selected organization.
*   **Permissions:** The UI relies on backend API rules. We do not filter data for security in JS; we display what the API returns.

### 2. Infrastructure as Data
We manage the connectivity layer as first-class entities:
*   **NATS:** Manage Accounts, Users, and Roles.
*   **Nebula:** Manage Certificate Authorities, Networks, and Hosts.
*   **Provisioning:** The UI facilitates the creation of cryptographic identities (NKeys, Certificates) via backend hooks and allows downloading `.creds` and `config.yaml` files directly.

### 3. Digital Twin (Real-Time State)
We bridge the gap between **SQL Metadata** (PocketBase) and **Live State** (NATS JetStream).
*   **Direct Connectivity:** The browser connects directly to the NATS cluster via WebSocket (`wss://`).
*   **Hybrid Auth:** Connection URLs are stored in local storage; Credentials (`.creds`) are fetched securely from the PocketBase user record.
*   **KV Store:**
    *   **Locations:** Each location code (e.g., `LOC_01`) maps to a NATS KV Bucket.
    *   **Things:** Things store their state in the bucket of their parent Location.
    *   **Visualization:** The `KvDashboard` component provides real-time CRUD, Watch, and Revision History for these keys.

### 4. Responsive Design 
We prioritize simplicity and maintainability:
*   **ResponsiveList:** A core component that renders as a high-density Table on desktop and a Card Grid on mobile.
*   **Themeing:** Dark/Light mode support (including map tiles).
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
*   **Backend Proxy:** API requests (`/api`, `/_`) are proxied to `http://127.0.0.1:8090`. Ensure the Go backend is running.

### Building for Production
```bash
npm run build
```
This compiles the application into `../pb_public`. The Go binary embeds this directory, allowing the entire platform to be deployed as a single executable.

---

## 🔌 NATS Connection Setup

To enable the Digital Twin features in development or production:

1.  **NATS Server:** Ensure your NATS server has WebSockets enabled in its config (`websocket { port: 9222 }`).
2.  **User Settings:**
    *   Go to **Settings** in the UI.
    *   Add your NATS WebSocket URL (e.g., `ws://localhost:9222`).
    *   Select your **Operational Identity** (links your web user to a NATS User/Creds file).
    *   Enable **Auto-connect**.
3.  **Digital Twin:** Navigate to a Location or Thing with a valid `Code` to see the live KV dashboard.
