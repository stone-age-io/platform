export interface BaseRecord {
  id: string
  created: string
  updated: string
  collectionId?: string
  collectionName?: string
  expand?: Record<string, any>
}

export interface AuthRecord extends BaseRecord {
  email: string
  emailVisibility: boolean
  verified: boolean
}

export interface SuperUser extends AuthRecord {
  current_organization?: string 
  // Optional fields to satisfy UI components shared with User
  name?: string
  avatar?: string
}

export interface Organization extends BaseRecord {
  name: string
  // Namespace root (ADR 0002): optional, immutable, globally unique; `name` is not.
  code?: string
  description?: string
  active: boolean
  owner: string // User ID
  logo?: string
  is_system_org?: boolean // adopts the $SYS NATS records (cluster ops only)
  is_operator_org?: boolean // the platform operator's org; its NATS account is the services hub
  managed?: boolean // exports its service events into the operator hub account
}

// org_stats view: counts per organization, operator-read. Not a BaseRecord (a
// view row has no created/updated). migrations/schema_update_org_stats.go.
export interface OrgStats {
  id: string // the organization's id
  things: number
  things_active: number
  locations: number
  members: number
}

export interface Membership extends BaseRecord {
  user: string // User ID
  organization: string // Organization ID
  role: 'owner' | 'admin' | 'member' | 'viewer' | 'dashboard'
  invited_by?: string
  nats_user?: string
}

export interface Invitation extends BaseRecord {
  email: string
  token?: string
  expires_at?: string
  resend_invite?: boolean
  organization: string
  role: 'admin' | 'member' | 'viewer' | 'dashboard'
  invited_by?: string
}

export interface User extends AuthRecord {
  name?: string
  avatar?: string
  current_organization?: string // Relation to Organization - this sets the org context
  is_operator?: boolean // Platform operator flag - can manage all organizations
}

// A capability belongs to an operation, not a type: it tells a request from a
// reply on the same subject suffix.
export type ThingTypeCapability = 'publish' | 'subscribe' | 'request' | 'reply'

export interface ThingType extends BaseRecord {
  organization?: string
  name?: string
  description?: string
  code?: string
  // 1-4 capitals starting every generated Thing code of this type (CA-9KD-4PX).
  // Separate from Location Type prefixes. ADR 0003.
  prefix?: string
  subject_prefix?: string
  operations?: string[] // Thing Type Operation IDs
  // Optional JSON Schema for the Thing form's metadata editor; not validated on write.
  metadata_schema?: Record<string, any> | null
}

// Shareable across Thing Types.
export interface ThingTypeOperation extends BaseRecord {
  organization?: string
  name: string
  capability: ThingTypeCapability
  subject_suffix: string
  description?: string
}

export interface Thing extends AuthRecord {
  organization?: string
  name?: string
  description?: string
  // Edit-only: POST /api/org/things is a JSON route and cannot carry an upload.
  photo?: string
  type?: string // Thing Type ID
  code?: string
  location?: string // Location ID
  metadata?: Record<string, any>
  floorplan_position?: { x: number; y: number } // Pixel coords on the location's floorplan image
  nats_user?: string
  nebula_host?: string
  // Deactivating blocks logins, invalidates tokens and revokes the linked NATS
  // identity. Owner/admin only. hooks/active_flag.go.
  active?: boolean
}

export interface LocationType extends BaseRecord {
  organization?: string
  name?: string
  description?: string
  code?: string
  // 1-4 capitals starting every generated Location code of this type.
  // Separate from Thing Type prefixes. ADR 0003.
  prefix?: string
  // Optional JSON Schema for the Location form's metadata editor; not validated on write.
  metadata_schema?: Record<string, any> | null
}

export interface Location extends BaseRecord {
  organization?: string
  name?: string
  description?: string
  type?: string // Location Type ID
  code?: string
  floorplan?: string
  photo?: string
  parent?: string // Parent Location ID
  // Codes from the root down, `/KC/BD-3/RM-204/`. Written only by the server
  // (hooks/location_path.go, ADR 0004); a value sent in a save is overwritten.
  path?: string
  coordinates?: {
    lat: number
    lon: number  // NOTE: PocketBase uses 'lon' not 'lng'
  }
  metadata?: Record<string, any>
}

export interface NatsAccount extends BaseRecord {
  name: string
  description?: string
  public_key?: string
  private_key?: string
  seed?: string
  signing_keys?: any[]
  add_signing_key?: boolean
  remove_signing_key?: string
  jwt?: string
  active?: boolean
  rotate_keys?: boolean
  max_connections?: number
  max_subscriptions?: number
  max_data?: number
  max_payload?: number
  max_jetstream_disk_storage?: number
  max_jetstream_memory_storage?: number
  organization?: string
}

export interface NatsUser extends AuthRecord {
  nats_username: string
  description?: string
  public_key?: string
  private_key?: string
  seed?: string
  account_id: string
  role_id: string
  jwt?: string
  creds_file?: string
  bearer_token?: boolean
  jwt_expires_at?: string
  regenerate?: boolean
  revoke?: boolean
  active?: boolean
  publish_permissions?: string
  subscribe_permissions?: string
  publish_deny_permissions?: string
  subscribe_deny_permissions?: string
  organization?: string
}

export interface NatsRole extends BaseRecord {
  name: string
  description?: string
  is_default?: boolean
  publish_permissions?: string
  subscribe_permissions?: string
  publish_deny_permissions?: string
  subscribe_deny_permissions?: string
  allow_response?: boolean
  allow_response_max?: number
  allow_response_ttl?: number
  max_subscriptions?: number
  max_data?: number
  max_payload?: number
  organization?: string
}

export interface NatsAccountExport extends BaseRecord {
  account_id: string
  name: string
  subject: string
  type: 'stream' | 'service'
  token_req?: boolean
  response_type?: 'Singleton' | 'Stream' | 'Chunked'
  response_threshold?: number
  account_token_position?: number
  advertise?: boolean
  allow_trace?: boolean
  description?: string
  organization?: string
  /** Account public keys granted a token_req export; the platform signs their tokens. */
  allowed_importers?: string[] | null
  /** Importer key -> unix-second cutoff, written by pb-nats when a key is removed. Read-only. */
  revocations?: Record<string, number> | null
}

export interface NatsAccountImport extends BaseRecord {
  account_id: string
  name: string
  subject: string
  account: string
  token?: string
  local_subject?: string
  type: 'stream' | 'service'
  share?: boolean
  allow_trace?: boolean
  description?: string
  organization?: string
}

export interface NebulaCA extends BaseRecord {
  name: string
  certificate?: string
  private_key?: string
  validity_years?: number
  expires_at?: string
  curve?: string
  organization?: string

  // Rotation. `rotate` is a write-only action field the console never sets
  // (updateRule is operator-only; it uses POST /api/org/nebula-ca/rotate). The
  // rest are pb-nebula output, from which caRotationState() in utils/nebula.ts
  // derives the state: incoming CA = prepared, outgoing = committed.
  rotate?: string
  next_certificate?: string
  previous_certificate?: string
  rotated_at?: string
}

export interface NebulaNetwork extends BaseRecord {
  name: string
  description?: string
  cidr_range: string
  active?: boolean
  ca_id: string
  organization?: string
}

export interface NebulaHost extends AuthRecord {
  hostname: string
  overlay_ip: string
  groups?: string[]
  is_lighthouse?: boolean
  public_host_port?: string
  certificate?: string
  private_key?: string
  ca_certificate?: string
  config_yaml?: string
  firewall_outbound?: Record<string, any>
  firewall_inbound?: Record<string, any>
  validity_years?: number
  expires_at?: string
  active?: boolean
  network_id: string
  organization?: string

  // Config-only, but a relay still needs a public_host_port, or it listens on
  // an ephemeral port while peers are told it is reachable.
  is_relay?: boolean

  // Zero and '' mean "inherit the default".
  mtu?: number
  tun_device?: string

  // Gateway routing has two halves on different hosts. unsafe_networks is
  // signed into the gateway's certificate, so an edit is inert until a new
  // certificate is issued.
  unsafe_networks?: string[]
  // On each host that wants to reach those subnets; `via` is the gateway's
  // overlay IP. Plain config.
  unsafe_routes?: Array<{ route: string; via: string }>

  // Underlay prefixes to favour among a peer's addresses (typically its LAN).
  // The only place IPv6 is allowed: it constrains the underlay, not the overlay.
  preferred_ranges?: string[]

  // Further static_host_map entries beyond public_host_port (dual-stack,
  // multi-path, split-horizon). Only a lighthouse's list reaches peers.
  additional_host_ports?: string[]

  // What this host reports about itself, distinct from what peers are told;
  // behind 1:1 NAT it would otherwise report its private address. "ip:port",
  // where port 0 means the real listening port.
  advertise_addrs?: string[]

  // Which local interface addresses to advertise (Nebula default: all, so a
  // container host offers docker0 and every bridge). Nebula's structure: an
  // optional `interfaces` map of name-regex to bool, plus CIDR-to-bool entries.
  local_allow_list?: Record<string, any>

  // Action field: set true and pb-nebula re-issues the certificate immediately,
  // then resets it in the same save. Never read back as state.
  renew?: boolean
}

/**
 * One line in the tenant activity feed. Not AuditLog (the operator trail with
 * full snapshots): org-scoped, no record values, and only collections every
 * role can already read (hooks/activity.go).
 *
 * `actor` is a plain id, not a relation, so it survives user deletion; render
 * the write-time snapshots `actor_label` and `resource_label`, not a lookup.
 */
export interface Activity extends BaseRecord {
  organization: string
  actor?: string
  actor_type?: 'user' | 'superuser'
  actor_label?: string
  action: 'created' | 'updated' | 'deleted' | 'provisioned'
  resource: string
  resource_id?: string
  resource_label?: string
}

export interface AuditLog extends BaseRecord {
  event_type: 'create_request' | 'update_request' | 'delete_request' | 'create' | 'update' | 'delete' | 'auth'
  collection_name: string
  record_id?: string
  user?: string
  auth_method?: string
  request_method?: string
  request_ip?: string
  request_url?: string
  timestamp: string
  before_changes?: Record<string, any>
  after_changes?: Record<string, any>
}
