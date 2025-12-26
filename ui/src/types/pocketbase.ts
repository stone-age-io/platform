// Base record interface that all PocketBase records extend
export interface BaseRecord {
  id: string
  created: string
  updated: string
  collectionId?: string
  collectionName?: string
  expand?: Record<string, any>
}

// Auth collections add these fields on top of BaseRecord
export interface AuthRecord extends BaseRecord {
  email: string
  emailVisibility: boolean
  verified: boolean
}

// Organization
export interface Organization extends BaseRecord {
  name: string
  description?: string
  active: boolean
  owner: string // User ID
}

// Membership
export interface Membership extends BaseRecord {
  user: string // User ID
  organization: string // Organization ID
  role: 'owner' | 'admin' | 'member'
  invited_by?: string
}

// Invitation
export interface Invitation extends BaseRecord {
  email: string
  token?: string
  expires_at?: string
  resend_invite?: boolean
  organization: string
  role: 'admin' | 'member'
  invited_by?: string
}

// User
export interface User extends AuthRecord {
  name?: string
  avatar?: string
  current_organization?: string // Relation to Organization - this sets the org context
  nats_user?: string
  nebula_host?: string
}

// Thing Type
export interface ThingType extends BaseRecord {
  organization?: string
  name?: string
  description?: string
  code?: string
  capabilities?: string[]
}

// Thing
export interface Thing extends AuthRecord {
  organization?: string
  name?: string
  description?: string
  type?: string // Thing Type ID
  code?: string
  location?: string // Location ID
  metadata?: Record<string, any>
  nats_user?: string
  nebula_host?: string
}

// Edge Type
export interface EdgeType extends BaseRecord {
  organization?: string
  name?: string
  description?: string
  code?: string
  features?: Record<string, any>
}

// Edge
export interface Edge extends AuthRecord {
  organization?: string
  name?: string
  description?: string
  type?: string // Edge Type ID
  code?: string
  metadata?: Record<string, any>
  nats_user?: string
  nebula_host?: string
}

// Location Type
export interface LocationType extends BaseRecord {
  organization?: string
  name?: string
  description?: string
  code?: string
}

// Location
export interface Location extends BaseRecord {
  organization?: string
  name?: string
  description?: string
  type?: string // Location Type ID
  code?: string
  floorplan?: string
  parent?: string // Parent Location ID
  coordinates?: {
    lat: number
    lng: number
  }
  metadata?: Record<string, any>
}

// NATS Account
export interface NatsAccount extends BaseRecord {
  name: string
  description?: string
  public_key?: string
  private_key?: string
  seed?: string
  signing_public_key?: string
  signing_private_key?: string
  signing_seed?: string
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

// NATS User
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
  active?: boolean
  organization?: string
}

// NATS Role
export interface NatsRole extends BaseRecord {
  name: string
  description?: string
  is_default?: boolean
  publish_permissions?: string
  subscribe_permissions?: string
  max_subscriptions?: number
  max_data?: number
  max_payload?: number
  organization?: string
}

// Nebula CA
export interface NebulaCA extends BaseRecord {
  name: string
  certificate?: string
  private_key?: string
  validity_years?: number
  expires_at?: string
  curve?: string
  organization?: string
}

// Nebula Network
export interface NebulaNetwork extends BaseRecord {
  name: string
  description?: string
  cidr_range: string
  active?: boolean
  ca_id: string
  organization?: string
}

// Nebula Host
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
}

// Audit Log
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
