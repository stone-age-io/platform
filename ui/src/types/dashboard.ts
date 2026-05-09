// ui/src/types/dashboard.ts

/**
 * Widget Types
 */
export type WidgetType =
  | 'chart'
  | 'text'
  | 'button'
  | 'kv'
  | 'kvtable'
  | 'switch'
  | 'slider'
  | 'stat'
  | 'gauge'
  | 'map'
  | 'console'
  | 'publisher'
  | 'status'
  | 'markdown'
  | 'pocketbase'
  | 'scanner'

export type DataSourceType = 'subscription' | 'consumer' | 'kv'
export type ChartType = 'line' | 'bar' | 'pie' | 'gauge'

// --- Dashboard Variables ---
export type VariableType = 'text' | 'select'

export interface DashboardVariable {
  id: string
  name: string
  label: string
  type: VariableType
  options?: string[]
  defaultValue: string
}

// --- Threshold Types ---
export type ThresholdOperator = '>' | '>=' | '<' | '<=' | '==' | '!='

export interface ThresholdRule {
  id: string
  operator: ThresholdOperator
  value: string
  color: string
}

// --- Data Source Configuration ---
export type DeliverPolicy = 'all' | 'last' | 'new' | 'last_per_subject' | 'by_start_time'

export interface DataSourceConfig {
  type: DataSourceType
  subject?: string
  subjects?: string[]
  stream?: string
  consumer?: string
  kvBucket?: string
  kvKey?: string
  
  // JetStream Options
  useJetStream?: boolean
  deliverPolicy?: DeliverPolicy
  timeWindow?: string
}

export interface BufferConfig {
  maxCount: number
  maxAge?: number
}

// --- Widget-Specific Configurations ---
export interface ChartWidgetConfig {
  chartType: ChartType
  echartOptions?: any
}

export interface TextWidgetConfig {
  format?: string
  fontSize?: number
  color?: string
  thresholds?: ThresholdRule[]
}

export interface ButtonWidgetConfig {
  label: string
  publishSubject: string
  payload: string
  color?: string
  actionType?: 'publish' | 'request'
  timeout?: number
}

export interface KvWidgetConfig {
  displayFormat?: 'raw' | 'json'
  refreshInterval?: number
  thresholds?: ThresholdRule[]
}

export interface SwitchWidgetConfig {
  mode: 'kv' | 'core'
  kvBucket?: string
  kvKey?: string
  defaultState?: 'on' | 'off'
  stateSubject?: string
  publishSubject: string
  onPayload: any
  offPayload: any
  confirmOnChange?: boolean
  confirmMessage?: string
  labels?: {
    on?: string
    off?: string
  }
}

export interface SliderWidgetConfig {
  mode: 'core' | 'kv'
  publishSubject: string
  stateSubject?: string
  kvBucket?: string
  kvKey?: string
  min: number
  max: number
  step: number
  defaultValue: number
  unit?: string
  valueTemplate?: string
  jsonPath?: string
  confirmOnChange?: boolean
  confirmMessage?: string
}

export interface StatCardWidgetConfig {
  format?: string
  unit?: string
  showTrend?: boolean
  trendWindow?: number
  thresholds?: ThresholdRule[]
}

export interface GaugeWidgetConfig {
  min: number
  max: number
  unit?: string
  zones?: {
    min: number
    max: number
    color: string
  }[]
}

export interface ConsoleWidgetConfig {
  fontSize?: number
  showTimestamp?: boolean
}

export interface PublisherHistoryItem {
  timestamp: number
  subject: string
  payload: string
}

export interface PublisherWidgetConfig {
  defaultSubject?: string
  defaultPayload?: string
  history?: PublisherHistoryItem[]
  timeout?: number
  // Thing Type Spec binding — when both are set, the widget resolves the
  // subject from the Thing's context and renders a form driven by the
  // operation's linked message_schema instead of free-text JSON.
  thingId?: string
  thingTypeOperationId?: string
}

export interface StatusMapping {
  id: string
  value: string
  color: string
  label?: string
  blink?: boolean
}

export interface StatusWidgetConfig {
  mappings: StatusMapping[]
  defaultColor: string
  defaultLabel?: string
  showStale?: boolean
  stalenessThreshold?: number
  staleColor?: string
  staleLabel?: string
}

export interface MarkdownWidgetConfig {
  content: string
}

// NEW: PocketBase Widget Config
export interface PocketBaseWidgetConfig {
  collection: string
  filter?: string
  sort?: string
  limit?: number
  refreshInterval?: number // in seconds
  fields?: string // comma separated list
}

// --- Scanner Widget Config ---
export type ScanPurpose = 'muster' | 'verify' | 'other'

export interface ScannerWidgetConfig {
  // KV lookup — primary badge path (value of key is a badge record, see below)
  kvEnabled?: boolean
  kvBucket?: string          // e.g. "badges"
  kvKeyTemplate?: string     // e.g. "{value}" — {value} replaced with scanned content

  // Validation rules applied to the KV record to decide GO/NO-GO.
  // Empty/undefined → any found record is GO.
  rules?: ScannerRule[]

  // PocketBase lookup — optional, for non-badge (asset/thing) scan scenarios
  pbEnabled?: boolean
  pbCollection?: string
  pbFilter?: string          // e.g. 'public_key = "{value}"'
  pbFields?: string

  // Publish scan event. Subject is templatable; payload is a fixed JSON shape.
  publishEnabled?: boolean
  publishSubjectTemplate?: string   // tokens: {value} {scanner} {scanner_kind} {device_label} {purpose} {location} {passed} {reason} {ts}

  // Legacy (pre-template) — read as fallback if publishSubjectTemplate is absent
  publishSubject?: string

  // Scan context — populates tokens in subject/payload templates
  deviceLabel?: string
  scanPurpose?: ScanPurpose
  location?: string          // free-form id or label

  // Behavior
  dedupWindowMs?: number     // suppress repeat scans of same value within window
  lookupTimeoutMs?: number   // abort KV lookup after this
  allowManualEntry?: boolean // show a text input fallback
}

// --- Scanner validation rules ---
export type ScannerRuleOp =
  | 'truthy' | 'falsy'
  | 'equals' | 'not_equals'
  | 'in' | 'not_in'
  | 'future' | 'past'
  | 'exists' | 'missing'

export interface ScannerRule {
  field: string           // dot-path into the record, e.g. "revoked" or "metadata.level"
  op: ScannerRuleOp
  value?: any             // used by equals/not_equals/in/not_in
  reason?: string         // shown as NO-GO label; falls back to `${field} ${op}`
}

// --- Badge KV record (value stored in the `badges` bucket) ---
// Interpreted by the scanner widget for GO/NO-GO decisions.
export interface BadgeRecord {
  expires_at?: string | null   // ISO timestamp; null/absent = no expiry
  revoked?: boolean            // default false
  metadata?: Record<string, any>
}

// --- KV Table Widget Types ---
export type KvTableColumnFormat = 'text' | 'number' | 'relative-time' | 'datetime'

export type ConditionalRuleOp = 'eq' | 'gt' | 'lt' | 'gte' | 'lte' | 'contains'
export type ConditionalRuleStyle = 'success' | 'warning' | 'error' | 'info'

export interface ConditionalRule {
  op: ConditionalRuleOp
  value: string         // compared as number for gt/lt/gte/lte, string for eq/contains
  style: ConditionalRuleStyle
}

export interface KvTableColumn {
  id: string
  label: string
  path: string         // JSONPath (e.g., "$.temperature") or meta-path ("__key_suffix__")
  format: KvTableColumnFormat
  formatOptions?: string  // e.g., date-fns format string for 'datetime'
  rules?: ConditionalRule[]  // first-match-wins coloring on the cell
}

export interface KvTableWidgetConfig {
  kvBucket: string
  keyPattern: string    // e.g., "{{location_code}}.>"
  columns: KvTableColumn[]
  defaultSortColumn?: string   // column id
  defaultSortDirection?: 'asc' | 'desc'
  maxRows?: number             // hard cap on rendered rows (default 500)
}

// --- Map Widget Types ---
export const MAP_LIMITS = {
  MAX_MARKERS: 50,
  MAX_ITEMS_PER_MARKER: 10,
  MAX_DYNAMIC_MARKERS: 500,
} as const

export interface DynamicMarkerPopupField {
  label: string
  path: string              // JSONPath or meta-path (__key_suffix__, __revision__, __timestamp__)
  format?: KvTableColumnFormat  // reuse: text, number, relative-time, datetime
}

export interface DynamicMarkerSource {
  kvBucket: string          // supports {{variable}}
  keyPattern: string        // e.g. "vehicles.>" — supports {{variable}}
  latPath: string           // JSONPath e.g. "$.lat"
  lonPath: string           // JSONPath e.g. "$.lon"
  labelPath: string         // JSONPath or "__key_suffix__"
  popupFields?: DynamicMarkerPopupField[]
}

export type MapItemType = 'publish' | 'switch' | 'text' | 'kv'

export interface MapItemSwitchConfig {
  mode: 'kv' | 'core'
  kvBucket?: string
  kvKey?: string
  publishSubject?: string
  stateSubject?: string
  onPayload: any
  offPayload: any
  confirmOnChange?: boolean
  labels?: {
    on?: string
    off?: string
  }
}

export interface MapItemTextConfig {
  subject: string
  jsonPath?: string
  unit?: string
  useJetStream?: boolean
  deliverPolicy?: DeliverPolicy
  timeWindow?: string
}

export interface MapItemKvConfig {
  kvBucket: string
  kvKey: string
  jsonPath?: string
}

export interface MapMarkerItem {
  id: string
  type: MapItemType
  label: string
  subject?: string
  payload?: string
  actionType?: 'publish' | 'request'
  timeout?: number
  switchConfig?: MapItemSwitchConfig
  textConfig?: MapItemTextConfig
  kvConfig?: MapItemKvConfig
}

export interface MarkerPositionConfig {
  mode: 'static' | 'dynamic'
  
  // Dynamic settings
  subject?: string
  latJsonPath?: string // e.g. "$.lat" or "$[0]"
  lonJsonPath?: string // e.g. "$.lon" or "$[1]"
  
  // JetStream settings
  useJetStream?: boolean
  deliverPolicy?: DeliverPolicy
  timeWindow?: string
}

export interface MapMarker {
  id: string
  label: string
  lat: number
  lon: number
  
  // Position Config
  positionConfig?: MarkerPositionConfig
  
  color?: string
  items: MapMarkerItem[]
}

export interface MapWidgetConfig {
  center: {
    lat: number
    lon: number
  }
  zoom: number
  markers: MapMarker[]
  dynamicMarkers?: DynamicMarkerSource
  enableClustering?: boolean
  fitBoundsOnLoad?: boolean
}

// --- Main Widget Configuration ---
export interface WidgetConfig {
  id: string
  type: WidgetType
  title: string
  x: number
  y: number
  w: number
  h: number
  dataSource: DataSourceConfig
  jsonPath?: string
  buffer: BufferConfig
  
  chartConfig?: ChartWidgetConfig
  textConfig?: TextWidgetConfig
  buttonConfig?: ButtonWidgetConfig
  kvConfig?: KvWidgetConfig
  switchConfig?: SwitchWidgetConfig
  sliderConfig?: SliderWidgetConfig
  statConfig?: StatCardWidgetConfig
  gaugeConfig?: GaugeWidgetConfig
  mapConfig?: MapWidgetConfig
  consoleConfig?: ConsoleWidgetConfig
  publisherConfig?: PublisherWidgetConfig
  statusConfig?: StatusWidgetConfig
  markdownConfig?: MarkdownWidgetConfig
  pocketbaseConfig?: PocketBaseWidgetConfig
  kvtableConfig?: KvTableWidgetConfig
  scannerConfig?: ScannerWidgetConfig
}

export type StorageType = 'local' | 'kv'

export interface Dashboard {
  id: string
  name: string
  description?: string
  created: number
  modified: number
  widgets: WidgetConfig[]
  variables?: DashboardVariable[]
  isLocked?: boolean
  storage?: StorageType
  kvKey?: string
  kvRevision?: number
  columnCount?: number
}

export const DEFAULT_WIDGET_SIZES: Record<WidgetType, { w: number; h: number }> = {
  chart: { w: 6, h: 4 },
  text: { w: 2, h: 2 },
  button: { w: 2, h: 2 },
  kv: { w: 4, h: 3 },
  switch: { w: 2, h: 2 },
  slider: { w: 4, h: 2 },
  stat: { w: 3, h: 2 },
  gauge: { w: 3, h: 3 },
  map: { w: 6, h: 4 },
  console: { w: 6, h: 4 },
  publisher: { w: 4, h: 4 },
  status: { w: 2, h: 2 },
  markdown: { w: 4, h: 4 },
  pocketbase: { w: 6, h: 4 },
  kvtable: { w: 6, h: 4 },
  scanner: { w: 4, h: 4 },
}

export const DEFAULT_BUFFER_CONFIG: BufferConfig = {
  maxCount: 10,
}

export function createDefaultWidget(type: WidgetType, position: { x: number; y: number }): WidgetConfig {
  const size = DEFAULT_WIDGET_SIZES[type]
  const id = `widget_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`
  
  const base: WidgetConfig = {
    id,
    type,
    title: `New ${type.charAt(0).toUpperCase() + type.slice(1)} Widget`,
    x: position.x,
    y: position.y,
    w: size.w,
    h: size.h,
    dataSource: { type: 'subscription' },
    buffer: { ...DEFAULT_BUFFER_CONFIG },
  }
  
  switch (type) {
    case 'chart':
      base.chartConfig = { chartType: 'line' }
      break
    case 'text':
      base.textConfig = { fontSize: 24, thresholds: [] }
      break
    case 'button':
      base.buttonConfig = { 
        label: 'Send', 
        publishSubject: 'button.clicked', 
        payload: '{}',
        actionType: 'publish',
        timeout: 1000
      }
      break
    case 'kv':
      base.kvConfig = { displayFormat: 'json', thresholds: [] }
      break
    case 'switch':
      base.switchConfig = {
        mode: 'kv',
        kvBucket: 'device-states',
        kvKey: 'device.switch',
        publishSubject: 'device.control',
        onPayload: { state: 'on' },
        offPayload: { state: 'off' },
        labels: { on: 'ON', off: 'OFF' }
      }
      break
    case 'slider':
      base.sliderConfig = {
        mode: 'core',
        publishSubject: 'device.slider',
        min: 0,
        max: 100,
        step: 1,
        defaultValue: 50,
        valueTemplate: '{{value}}',
        unit: '%'
      }
      break
    case 'stat':
      base.dataSource = { type: 'subscription', subject: 'metrics.value' }
      base.statConfig = {
        unit: '',
        showTrend: true,
        trendWindow: 10,
        thresholds: []
      }
      break
    case 'gauge':
      base.dataSource = { type: 'subscription', subject: 'sensor.value' }
      base.gaugeConfig = {
        min: 0,
        max: 100,
        unit: '',
        zones: [
          { min: 0, max: 60, color: 'var(--color-success)' },
          { min: 60, max: 80, color: 'var(--color-warning)' },
          { min: 80, max: 100, color: 'var(--color-error)' }
        ]
      }
      break
    case 'map':
      base.mapConfig = {
        center: { lat: 39.8283, lon: -98.5795 },
        zoom: 4,
        markers: []
      }
      break
    case 'console':
      base.dataSource = { type: 'subscription', subject: '>', subjects: ['>'] }
      base.consoleConfig = { fontSize: 12, showTimestamp: true }
      base.buffer.maxCount = 100
      break
    case 'publisher':
      base.title = 'Publisher'
      base.publisherConfig = {
        defaultSubject: 'test.subject',
        defaultPayload: '{\n  "action": "test"\n}',
        history: [],
        timeout: 2000
      }
      break
    case 'status':
      base.title = 'Status'
      base.dataSource = { type: 'subscription', subject: 'service.status' }
      base.statusConfig = {
        mappings: [
          { id: '1', value: 'online', color: 'var(--color-success)', label: 'Online', blink: false },
          { id: '2', value: 'error', color: 'var(--color-error)', label: 'Error', blink: true }
        ],
        defaultColor: 'var(--color-info)',
        defaultLabel: 'Unknown',
        showStale: true,
        stalenessThreshold: 60000,
        staleColor: 'var(--muted)',
        staleLabel: 'Stale'
      }
      break
    case 'markdown':
      base.title = '' // Empty title often looks better for static text
      base.markdownConfig = {
        content: '### Hello World\n\nThis is a **markdown** widget.\n\nYou can use variables like {{device_id}}.'
      }
      break
    case 'pocketbase':
      base.title = 'PocketBase Query'
      base.pocketbaseConfig = {
        collection: 'audit_logs',
        limit: 10,
        sort: '-created',
        refreshInterval: 0
      }
      break
    case 'kvtable':
      base.title = 'KV Table'
      base.kvtableConfig = {
        kvBucket: '',
        keyPattern: '>',
        columns: [
          { id: 'col_1', label: 'Key', path: '__key_suffix__', format: 'text' }
        ],
        defaultSortDirection: 'desc',
        maxRows: 500
      }
      break
    case 'scanner':
      base.title = 'Scanner'
      base.scannerConfig = {
        kvEnabled: true,
        kvBucket: 'badges',
        kvKeyTemplate: '{value}',
        pbEnabled: false,
        pbCollection: '',
        pbFilter: '',
        pbFields: '',
        publishEnabled: false,
        publishSubjectTemplate: 'scans.{purpose}.{scanner}',
        deviceLabel: '',
        scanPurpose: 'verify',
        location: '',
        dedupWindowMs: 3000,
        lookupTimeoutMs: 5000,
        allowManualEntry: true,
      }
      break
  }
  
  return base
}

export function createDefaultDashboard(name: string): Dashboard {
  const now = Date.now()
  return {
    id: `dashboard_${now}_${Math.random().toString(36).substr(2, 9)}`,
    name,
    description: '',
    created: now,
    modified: now,
    widgets: [],
    variables: [],
    isLocked: false,
    storage: 'local',
    columnCount: 12
  }
}

export function createDefaultMarker(): MapMarker {
  return {
    id: `marker_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
    label: 'New Marker',
    lat: 0,
    lon: 0,
    items: []
  }
}

export function createDefaultItem(type: MapItemType = 'publish'): MapMarkerItem {
  const id = `item_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`
  
  if (type === 'switch') {
    return {
      id,
      type: 'switch',
      label: 'Toggle',
      switchConfig: {
        mode: 'kv',
        kvBucket: 'device-states',
        kvKey: 'device.switch',
        onPayload: { state: 'on' },
        offPayload: { state: 'off' },
        labels: { on: 'ON', off: 'OFF' }
      }
    }
  } else if (type === 'text') {
    return {
      id,
      type: 'text',
      label: 'Value',
      textConfig: {
        subject: 'data.subject',
        unit: '',
        useJetStream: false,
        deliverPolicy: 'last',
        timeWindow: '10m'
      }
    }
  } else if (type === 'kv') {
    return {
      id,
      type: 'kv',
      label: 'KV Value',
      kvConfig: {
        kvBucket: 'my-bucket',
        kvKey: 'my-key'
      }
    }
  }
  
  return {
    id,
    type: 'publish',
    label: 'Send',
    subject: 'marker.action',
    payload: '{}',
    actionType: 'publish',
    timeout: 1000
  }
}
