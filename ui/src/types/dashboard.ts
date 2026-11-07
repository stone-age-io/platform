import { TWIN_BUCKET, TWIN_DESIRED_BUCKET } from '@/utils/twin'

/**
 * Widget types as a runtime list with the union derived from it, so tests can
 * iterate every type and `Record<WidgetType, …>` still fails to compile on a gap.
 */
export const WIDGET_TYPES = [
  'chart',
  'text',
  'button',
  'buttonpad',
  'kv',
  'kvtable',
  'streamtable',
  'switch',
  'slider',
  'stat',
  'gauge',
  'map',
  'floorplan',
  'console',
  'publisher',
  'status',
  'markdown',
  'scanner',
] as const

export type WidgetType = (typeof WIDGET_TYPES)[number]

export type DataSourceType = 'subscription' | 'consumer' | 'kv'
export type ChartType = 'line' | 'bar' | 'timeline'

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
  // Display text for a matched value. Only the state timeline reads it.
  label?: string
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

  // Keep `$...` and `_INBOX` traffic out of a subscription whose first token
  // is a wildcard (utils/systemSubjects.ts). Absent means off, for old dashboards.
  hideSystemSubjects?: boolean
}

// No time window here: that is ChartWidgetConfig.window, since only a chart
// ages messages out.
export interface BufferConfig {
  maxCount: number
}

// --- Widget-Specific Configurations ---
// One line on a chart. Each series reads the full payload (BufferedMessage.raw)
// and takes a point unless `subject` is set and does not match exactly, or
// `path` finds nothing. `subject` separates devices that publish the same shape.
export interface ChartSeries {
  label: string
  path: string
  subject?: string   // exact match after variable substitution; blank = every message
}

export interface ChartWidgetConfig {
  chartType: ChartType
  // Absent on older charts: they draw one series from the widget-level jsonPath
  // until the config form next saves them as a one-entry list.
  series?: ChartSeries[]
  // Duration string ("30m"). Empty = keep the last N messages (the buffer size).
  // When set it is also the JetStream replay window.
  window?: string
  // Timeline: first match colours and labels a segment; unmatched values get a
  // colour hashed from the value. Line and bar: each numeric rule is a dashed
  // line at that value, and the operator is ignored.
  thresholds?: ThresholdRule[]
  // Line and bar: fixed y-axis ends. Absent = fitted to the data.
  yMin?: number
  yMax?: number
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

/**
 * One key of a button pad. `row` and `col` are 0-based and stored on the cell,
 * so changing the column count does not shuffle the keys. A position with no
 * cell is a gap (a D-pad's corners).
 */
export interface ButtonPadCell {
  row: number
  col: number
  label: string
  subject: string    // supports {{variables}}
  payload: string    // supports {{variables}}
  color?: string
}

/**
 * A fixed rows x cols grid of publish-only buttons. It draws the same shape on
 * desktop and on a phone, which is the reason it exists: the phone layout pairs
 * separate widgets two to a row and cannot keep a shape. Deliberately no
 * request/reply, hold-to-repeat, spans or state.
 */
export interface ButtonPadWidgetConfig {
  rows: number
  cols: number
  cells: ButtonPadCell[]
}

export const BUTTON_PAD_MAX = 8

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
  format?: string            // "{value}°C"; when set, `unit` is not appended
  unit?: string
  decimals?: number          // absent = picked from the value's size
  showTrend?: boolean
  trendWindow?: number
  trendUpIsBad?: boolean     // a rising freezer temperature is red, not green
  thresholds?: ThresholdRule[]
}

export interface GaugeWidgetConfig {
  // 'bar' is a horizontal level (a tank, a battery). Absent = arc.
  style?: 'arc' | 'bar'
  min: number
  max: number
  unit?: string
  decimals?: number          // absent = picked from the value's size, as the stat card does
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
  // When both are set, the subject resolves from the Thing's context, its
  // type's prefix and the operation's suffix.
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

// --- Scanner Widget Config ---
export type ScanPurpose = 'muster' | 'verify' | 'other'

export interface ScannerWidgetConfig {
  // KV lookup with the caller's own NATS credential. Any JSON shape; rules read it.
  kvEnabled?: boolean
  kvBucket?: string
  kvKeyTemplate?: string     // e.g. "{value}" — {value} replaced with scanned content

  // PASS/FAIL rules on the looked-up record; none means any found record passes.
  // Computed in the browser and advisory only: nothing downstream may treat the
  // published `passed` as an authorization decision.
  rules?: ScannerRule[]

  pbEnabled?: boolean
  pbCollection?: string
  pbFilter?: string          // e.g. 'public_key = "{value}"'
  pbFields?: string

  // Subject is templatable; the payload is a fixed JSON shape.
  publishEnabled?: boolean
  publishSubjectTemplate?: string   // tokens: {value} {scanner} {scanner_kind} {device_label} {purpose} {location} {passed} {reason} {ts}

  // Legacy fallback when publishSubjectTemplate is absent.
  publishSubject?: string

  // Fill tokens in the subject/payload templates.
  deviceLabel?: string
  scanPurpose?: ScanPurpose
  location?: string          // free-form id or label

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
  reason?: string         // shown as the FAIL label; falls back to `${field} ${op}`
}

// --- KV Table Widget Types ---
export type TableColumnFormat = 'text' | 'number' | 'relative-time' | 'datetime'

export type ConditionalRuleOp = 'eq' | 'gt' | 'lt' | 'gte' | 'lte' | 'contains'
export type ConditionalRuleStyle = 'success' | 'warning' | 'error' | 'info'

export interface ConditionalRule {
  op: ConditionalRuleOp
  value: string         // compared as number for gt/lt/gte/lte, string for eq/contains
  style: ConditionalRuleStyle
}

export interface TableColumn {
  id: string
  label: string
  path: string         // JSONPath (e.g., "$.temperature") or meta-path ("__key_suffix__")
  format: TableColumnFormat
  formatOptions?: string  // e.g., date-fns format string for 'datetime'
  rules?: ConditionalRule[]  // first-match-wins coloring on the cell
}

export interface KvTableWidgetConfig {
  kvBucket: string
  keyPattern: string    // e.g., "{{location_code}}.>"
  columns: TableColumn[]
  defaultSortColumn?: string   // column id
  defaultSortDirection?: 'asc' | 'desc'
  maxRows?: number             // hard cap on rendered rows (default 500)
}

// --- Stream Table Widget ---
// Subscription-driven sibling of KvTable: rows are messages from a NATS subject
// ring-buffer. Subjects/JetStream/bufferSize live on widget.dataSource.
export interface StreamTableWidgetConfig {
  columns: TableColumn[]
  defaultSortColumn?: string
  defaultSortDirection?: 'asc' | 'desc'
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
  format?: TableColumnFormat  // reuse: text, number, relative-time, datetime
}

export interface DynamicMarkerSource {
  kvBucket: string          // supports {{variable}}
  keyPattern: string        // e.g. "vehicles.>" — supports {{variable}}
  latPath: string           // JSONPath e.g. "$.lat"
  lonPath: string           // JSONPath e.g. "$.lon"
  labelPath: string         // JSONPath or "__key_suffix__"
  popupFields?: DynamicMarkerPopupField[]
}

// Adding a member is safe; removing one breaks every saved dashboard using it.
export type MapItemType = 'publish' | 'switch' | 'text' | 'kv' | 'slider' | 'gauge'

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

/**
 * A setpoint row, with the same `mode` split as the switch item. Publishes on
 * release, not per frame, so a drag is one write.
 */
export interface MapItemSliderConfig {
  mode: 'kv' | 'core'
  kvBucket?: string
  kvKey?: string
  publishSubject?: string
  /** Where the current value is read in core mode. Defaults to the publish subject. */
  stateSubject?: string
  min: number
  max: number
  step: number
  defaultValue: number
  unit?: string
  /** `{{value}}` is replaced. Absent publishes the bare number. */
  valueTemplate?: string
  jsonPath?: string
  confirmOnChange?: boolean
  confirmMessage?: string
}

/** A read-only level row. Bar only: an arc is unreadable in a 280px panel. */
export interface MapItemGaugeConfig {
  mode: 'kv' | 'core'
  kvBucket?: string
  kvKey?: string
  subject?: string
  jsonPath?: string
  min: number
  max: number
  unit?: string
  decimals?: number
  zones?: { min: number; max: number; color: string }[]
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
  sliderConfig?: MapItemSliderConfig
  gaugeConfig?: MapItemGaugeConfig
}

export interface MarkerPositionConfig {
  mode: 'static' | 'dynamic'

  // Dynamic mode only
  subject?: string
  latJsonPath?: string // e.g. "$.lat" or "$[0]"
  lonJsonPath?: string // e.g. "$.lon" or "$[1]"

  useJetStream?: boolean
  deliverPolicy?: DeliverPolicy
  timeWindow?: string
}

export interface MapMarker {
  id: string
  label: string
  lat: number
  lon: number

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

// --- Floor Plan Widget ---
// Markers are the Things in `locationId` with a `floorplan_position`; the widget
// stores no positions (plans/floorplan-widget.md).

export type FloorPlanStateMode = 'subject' | 'kv'

/** Where a marker's live state comes from, as a template resolved per Thing. */
export interface FloorPlanStateSource {
  mode: FloorPlanStateMode
  /** subject mode: joined onto the Thing Type's own `subject_prefix`. */
  subjectSuffix?: string
  /** kv mode: bucket plus a key template carrying {thing} and friends. */
  kvBucket?: string
  kvKeyTemplate?: string
  /** Read out of the payload once the message or entry arrives. */
  jsonPath?: string
}

/**
 * One binding, applied to every Thing of a Thing Type in the location, so sixty
 * doors are configured once. Templates are resolved per Thing by
 * utils/floorplanBindings.ts. The state fields mirror StatusWidgetConfig.
 */
export interface FloorPlanTypeBinding {
  id: string
  thingTypeId: string
  stateSource?: FloorPlanStateSource
  mappings?: StatusMapping[]
  defaultColor?: string
  defaultLabel?: string
  showStale?: boolean
  stalenessThreshold?: number
  staleColor?: string
  staleLabel?: string
  /** Panel rows, resolved per Thing when its panel opens. */
  items: MapMarkerItem[]
}

export interface FloorPlanWidgetConfig {
  /** The Location whose `floorplan` image is the canvas. */
  locationId: string
  bindings: FloorPlanTypeBinding[]
  /** Draw Things whose type has no binding. They get a marker and no panel rows. */
  showUnbound?: boolean
}

export const FLOORPLAN_LIMITS = {
  // Larger than MAP_LIMITS: these markers are inventory, and one subscription
  // per binding (not per marker) keeps 200 affordable.
  MAX_MARKERS: 200,
  MAX_BINDINGS: 10,
  MAX_ITEMS_PER_BINDING: 10,
} as const

// --- Main Widget Configuration ---

/** Everything a widget has regardless of its type. */
export interface WidgetConfigBase {
  id: string
  title: string
  x: number
  y: number
  w: number
  h: number
  dataSource: DataSourceConfig
  jsonPath?: string
  // Where the payload carries its own time (epoch s/ms/µs/ns or ISO 8601).
  // Unset: JetStream's stored time, else the time the browser received it.
  timestampPath?: string
  buffer: BufferConfig
}

/**
 * The one config object each widget type owns, keyed by `type`: a type in
 * WIDGET_TYPES with no entry here does not compile, and a narrowed widget gets
 * its config without a `!`.
 *
 * The field names are frozen: every saved dashboard (localStorage, KV, exported
 * JSON) carries these keys, so renaming them is a data migration.
 *
 * Narrowing is a compile-time claim about user-editable JSON, so readers must
 * still cope with a missing config object. `types/dashboard.spec.ts` pins the
 * construction side.
 */
export interface WidgetConfigByType {
  chart: { chartConfig: ChartWidgetConfig }
  text: { textConfig: TextWidgetConfig }
  button: { buttonConfig: ButtonWidgetConfig }
  buttonpad: { buttonpadConfig: ButtonPadWidgetConfig }
  kv: { kvConfig: KvWidgetConfig }
  kvtable: { kvtableConfig: KvTableWidgetConfig }
  streamtable: { streamtableConfig: StreamTableWidgetConfig }
  switch: { switchConfig: SwitchWidgetConfig }
  slider: { sliderConfig: SliderWidgetConfig }
  stat: { statConfig: StatCardWidgetConfig }
  gauge: { gaugeConfig: GaugeWidgetConfig }
  map: { mapConfig: MapWidgetConfig }
  floorplan: { floorplanConfig: FloorPlanWidgetConfig }
  console: { consoleConfig: ConsoleWidgetConfig }
  publisher: { publisherConfig: PublisherWidgetConfig }
  status: { statusConfig: StatusWidgetConfig }
  markdown: { markdownConfig: MarkdownWidgetConfig }
  scanner: { scannerConfig: ScannerWidgetConfig }
}

/** One widget, narrowed by `type`. */
export type WidgetConfig = {
  [T in WidgetType]: WidgetConfigBase & { type: T } & WidgetConfigByType[T]
}[WidgetType]

/** One widget of a known type, e.g. `defineProps<{ config: WidgetConfigOf<'gauge'> }>()`. */
export type WidgetConfigOf<T extends WidgetType> = Extract<WidgetConfig, { type: T }>

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
  buttonpad: { w: 4, h: 4 },
  kv: { w: 4, h: 3 },
  switch: { w: 2, h: 2 },
  slider: { w: 4, h: 2 },
  stat: { w: 3, h: 2 },
  gauge: { w: 3, h: 3 },
  map: { w: 6, h: 4 },
  floorplan: { w: 6, h: 5 },
  console: { w: 6, h: 4 },
  publisher: { w: 4, h: 4 },
  status: { w: 2, h: 2 },
  markdown: { w: 4, h: 4 },
  kvtable: { w: 6, h: 4 },
  streamtable: { w: 6, h: 4 },
  scanner: { w: 4, h: 4 },
}

export const DEFAULT_BUFFER_CONFIG: BufferConfig = {
  maxCount: 10,
}

/**
 * What each widget type starts as, beyond the shared fields. A `Record`, not a
 * switch, so every type must supply its config object. `title` is required;
 * any other base field is optional and overrides the shared default.
 */
type WidgetDefaults<T extends WidgetType> = { title: string } &
  Omit<WidgetConfigOf<T>, keyof WidgetConfigBase | 'type'> &
  Partial<WidgetConfigBase>

const WIDGET_DEFAULTS: { [T in WidgetType]: () => WidgetDefaults<T> } = {
  chart: () => ({
    title: 'Chart Widget',
    // No widget-level jsonPath: each series reads the whole payload.
    chartConfig: { chartType: 'line', series: [{ label: 'Value', path: '$.value' }] },
    // The shared 10 is a latest-value buffer, too few for a line or a window.
    buffer: { maxCount: 200 },
  }),

  text: () => ({
    title: 'Text Widget',
    jsonPath: '$.value',
    textConfig: { fontSize: 24, thresholds: [] },
  }),

  button: () => ({
    title: 'Button Widget',
    buttonConfig: {
      label: 'Send',
      publishSubject: 'cmd.thing.DEVICE_01.action',
      payload: '{}',
      actionType: 'publish',
      timeout: 1000,
    },
  }),

  // A D-pad: it shows the gaps, and the shared subject shows the usual setup.
  buttonpad: () => ({
    title: 'Button Pad',
    buttonpadConfig: {
      rows: 3,
      cols: 3,
      cells: [
        { row: 0, col: 1, label: '↑', subject: 'cmd.thing.DEVICE_01.key', payload: '{"key":"UP"}' },
        { row: 1, col: 0, label: '←', subject: 'cmd.thing.DEVICE_01.key', payload: '{"key":"LEFT"}' },
        { row: 1, col: 1, label: 'OK', subject: 'cmd.thing.DEVICE_01.key', payload: '{"key":"OK"}' },
        { row: 1, col: 2, label: '→', subject: 'cmd.thing.DEVICE_01.key', payload: '{"key":"RIGHT"}' },
        { row: 2, col: 1, label: '↓', subject: 'cmd.thing.DEVICE_01.key', payload: '{"key":"DOWN"}' },
      ],
    },
  }),

  kv: () => ({
    title: 'KV Widget',
    kvConfig: { displayFormat: 'json', thresholds: [] },
  }),

  switch: () => ({
    title: 'Switch Control',
    // A switch writes, so it starts on the desired-state twin bucket; any
    // bucket works.
    switchConfig: {
      mode: 'kv',
      kvBucket: TWIN_DESIRED_BUCKET,
      kvKey: 'thing.DEVICE_01.switch',
      publishSubject: 'cmd.thing.DEVICE_01.switch',
      onPayload: { state: 'on' },
      offPayload: { state: 'off' },
      labels: { on: 'ON', off: 'OFF' },
    },
  }),

  slider: () => ({
    title: 'Slider Control',
    sliderConfig: {
      mode: 'core',
      publishSubject: 'cmd.thing.DEVICE_01.level',
      min: 0,
      max: 100,
      step: 1,
      defaultValue: 50,
      valueTemplate: '{{value}}',
      unit: '%',
    },
  }),

  stat: () => ({
    title: 'Stat Card',
    jsonPath: '$.value',
    dataSource: { type: 'subscription', subject: 'metrics.value' },
    statConfig: { unit: '', showTrend: true, trendWindow: 10, thresholds: [] },
  }),

  gauge: () => ({
    title: 'Gauge Meter',
    jsonPath: '$.value',
    dataSource: { type: 'subscription', subject: 'sensor.value' },
    gaugeConfig: {
      min: 0,
      max: 100,
      unit: '',
      zones: [
        { min: 0, max: 60, color: 'var(--color-success)' },
        { min: 60, max: 80, color: 'var(--color-warning)' },
        { min: 80, max: 100, color: 'var(--color-error)' },
      ],
    },
  }),

  map: () => ({
    title: 'Map Widget',
    mapConfig: {
      center: { lat: 39.8283, lon: -98.5795 },
      zoom: 4,
      markers: [],
    },
  }),

  floorplan: () => ({
    title: 'Floor Plan',
    // Empty: the widget prompts for a location rather than guessing an id.
    floorplanConfig: {
      locationId: '',
      bindings: [],
      showUnbound: true,
    },
  }),

  console: () => ({
    title: 'Console Stream',
    dataSource: { type: 'subscription', subject: '>', subjects: ['>'], hideSystemSubjects: true },
    consoleConfig: { fontSize: 12, showTimestamp: true },
    buffer: { maxCount: 200 },
  }),

  publisher: () => ({
    title: 'Publisher',
    publisherConfig: {
      defaultSubject: 'test.subject',
      defaultPayload: '{\n  "action": "test"\n}',
      history: [],
      timeout: 2000,
    },
  }),

  status: () => ({
    title: 'Status',
    dataSource: { type: 'subscription', subject: 'service.status' },
    statusConfig: {
      mappings: [
        { id: '1', value: 'online', color: 'var(--color-success)', label: 'Online', blink: false },
        { id: '2', value: 'error', color: 'var(--color-error)', label: 'Error', blink: true },
      ],
      defaultColor: 'var(--color-info)',
      defaultLabel: 'Unknown',
      showStale: true,
      stalenessThreshold: 60000,
      staleColor: 'var(--muted)',
      staleLabel: 'Stale',
    },
  }),

  markdown: () => ({
    // Untitled: the content usually carries its own heading.
    title: '',
    markdownConfig: {
      content: '### Hello World\n\nThis is a **markdown** widget.\n\nYou can use variables like {{device_id}}.',
    },
  }),

  kvtable: () => ({
    title: 'KV Table',
    kvtableConfig: {
      kvBucket: TWIN_BUCKET,
      keyPattern: 'thing.>',
      // reads reported state; see utils/twin.ts
      columns: [{ id: 'col_1', label: 'Key', path: '__key_suffix__', format: 'text' }],
      defaultSortDirection: 'desc',
      maxRows: 500,
    },
  }),

  streamtable: () => ({
    title: 'Stream Table',
    dataSource: { type: 'subscription', subjects: [] },
    buffer: { maxCount: 200 },
    streamtableConfig: {
      columns: [
        { id: 'col_subject', label: 'Subject', path: '__subject__', format: 'text' },
        { id: 'col_timestamp', label: 'Time', path: '__timestamp__', format: 'relative-time' },
      ],
      defaultSortDirection: 'desc',
    },
  }),

  scanner: () => ({
    title: 'Scanner',
    scannerConfig: {
      kvEnabled: true,
      kvBucket: '',
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
    },
  }),
}

export function createDefaultWidget<T extends WidgetType>(
  type: T,
  position: { x: number; y: number },
): WidgetConfigOf<T> {
  const size = DEFAULT_WIDGET_SIZES[type]
  const id = `widget_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`

  // The compiler checks each WIDGET_DEFAULTS entry but cannot see that the
  // generic spread completes WidgetConfigOf<T>, hence the cast.
  return {
    id,
    type,
    x: position.x,
    y: position.y,
    w: size.w,
    h: size.h,
    dataSource: { type: 'subscription' },
    buffer: { ...DEFAULT_BUFFER_CONFIG },
    ...WIDGET_DEFAULTS[type](),
  } as WidgetConfigOf<T>
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
        kvBucket: TWIN_DESIRED_BUCKET,
        kvKey: 'thing.DEVICE_01.switch',
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
  } else if (type === 'slider') {
    return {
      id,
      type: 'slider',
      label: 'Setpoint',
      sliderConfig: {
        mode: 'kv',
        // A setpoint is desired state, like the switch item.
        kvBucket: TWIN_DESIRED_BUCKET,
        kvKey: 'thing.DEVICE_01.setpoint',
        min: 0,
        max: 100,
        step: 1,
        defaultValue: 50,
        unit: '',
      },
    }
  } else if (type === 'gauge') {
    return {
      id,
      type: 'gauge',
      label: 'Level',
      gaugeConfig: {
        mode: 'kv',
        kvBucket: TWIN_BUCKET,
        kvKey: 'thing.DEVICE_01.level',
        min: 0,
        max: 100,
        unit: '%',
      },
    }
  } else if (type === 'kv') {
    return {
      id,
      type: 'kv',
      label: 'KV Value',
      kvConfig: {
        kvBucket: TWIN_BUCKET,
        kvKey: 'thing.DEVICE_01.temp'
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
