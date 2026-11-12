import type { ButtonPadCell, ThresholdRule,MapMarker, SwitchWidgetConfig, SliderWidgetConfig, GaugeWidgetConfig, StatusMapping, TableColumn, DynamicMarkerPopupField, ScannerRule, ChartType, ChartSeries, FloorPlanTypeBinding } from './dashboard'

export interface WidgetFormState {
  // Common
  title: string
  subject: string
  subjects: string[]
  jsonPath: string
  timestampPath: string
  bufferSize: number
  
  // Data Source Type
  dataSourceType: 'subscription' | 'kv' | 'none'
  
  // KV Widget
  kvBucket: string
  kvKey: string
  
  // Button Widget
  buttonLabel: string
  buttonPayload: string
  buttonColor: string
  buttonActionType: 'publish' | 'request'
  buttonTimeout: number

  // Button Pad Widget
  buttonpadRows: number
  buttonpadCols: number
  buttonpadCells: ButtonPadCell[]

  // Thresholds
  thresholds: ThresholdRule[]

  // Chart Widget
  chartType: ChartType
  chartSeries: ChartSeries[]
  chartWindow: string
  chartThresholds: ThresholdRule[]
  chartYMin: string           // '' = fitted to the data
  chartYMax: string

  // Switch and slider: the config object itself, edited in place by
  // SwitchFields and SliderFields. No flat copy of its fields, and none of the
  // common subject or KV fields above are used for these two.
  switchConfig: SwitchWidgetConfig
  sliderConfig: SliderWidgetConfig
  
  // Stat Widget
  statFormat: string
  statUnit: string
  statDecimals: string        // '' = automatic
  statShowTrend: boolean
  statTrendWindow: number
  statTrendUpIsBad: boolean
  
  // Gauge: the config object itself, edited in place by GaugeFields.
  gaugeConfig: GaugeWidgetConfig
  
  // Map Widget
  mapCenterLat: number
  mapCenterLon: number
  mapZoom: number
  mapMarkers: MapMarker[]
  mapDynamicEnabled: boolean
  mapDynamicBucket: string
  mapDynamicKeyPattern: string
  mapDynamicLatPath: string
  mapDynamicLonPath: string
  mapDynamicLabelPath: string
  mapDynamicPopupFields: DynamicMarkerPopupField[]
  mapEnableClustering: boolean
  mapFitBoundsOnLoad: boolean

  // Floor Plan Widget
  // No positions: they are edited on the location's own plan.
  floorplanLocationId: string
  floorplanBindings: FloorPlanTypeBinding[]
  floorplanShowUnbound: boolean
  
  // Console Widget
  consoleFontSize: number
  consoleShowTimestamp: boolean

  // Publisher Widget
  publisherDefaultSubject: string
  publisherDefaultPayload: string
  publisherTimeout: number
  publisherThingId: string
  publisherThingTypeOperationId: string
  
  // Status Widget
  statusMappings: StatusMapping[]
  statusDefaultColor: string
  statusDefaultLabel: string
  statusShowStale: boolean
  statusStalenessThreshold: number
  statusStaleColor: string
  statusStaleLabel: string

  // Markdown Widget
  markdownContent: string

  // KV Table Widget
  kvTableBucket: string
  kvTableKeyPattern: string
  kvTableColumns: TableColumn[]
  kvTableDefaultSortColumn: string
  kvTableDefaultSortDirection: 'asc' | 'desc'
  kvTableMaxRows: number

  // Stream Table Widget (subjects/JetStream/bufferSize use the shared fields)
  streamTableColumns: TableColumn[]
  streamTableDefaultSortColumn: string
  streamTableDefaultSortDirection: 'asc' | 'desc'

  // Scanner Widget
  scannerKvEnabled: boolean
  scannerKvBucket: string
  scannerKvKeyTemplate: string
  scannerRules: ScannerRule[]
  scannerPbEnabled: boolean
  scannerPbCollection: string
  scannerPbFilter: string
  scannerPbFields: string
  scannerPublishEnabled: boolean
  scannerPublishSubjectTemplate: string
  scannerDeviceLabel: string
  scannerPurpose: 'muster' | 'verify' | 'other'
  scannerLocation: string
  scannerDedupWindowMs: number
  scannerLookupTimeoutMs: number
  scannerAllowManualEntry: boolean

  // JetStream
  useJetStream: boolean
  deliverPolicy: 'all' | 'last' | 'new' | 'last_per_subject' | 'by_start_time'
  jetstreamTimeWindow: string

  hideSystemSubjects: boolean
}

export function createEmptyFormState(): WidgetFormState {
  return {
    title: '',
    subject: '',
    subjects: [],
    jsonPath: '',
    timestampPath: '',
    bufferSize: 100,
    dataSourceType: 'subscription',
    kvBucket: '',
    kvKey: '',
    buttonLabel: '',
    buttonPayload: '',
    buttonColor: '',
    buttonActionType: 'publish',
    buttonTimeout: 1000,
    buttonpadRows: 3,
    buttonpadCols: 3,
    buttonpadCells: [],
    thresholds: [],
    chartType: 'line',
    chartSeries: [],
    chartWindow: '',
    chartThresholds: [],
    chartYMin: '',
    chartYMax: '',
    switchConfig: { mode: 'kv', onPayload: { state: 'on' }, offPayload: { state: 'off' } },
    sliderConfig: { mode: 'core', min: 0, max: 100, step: 1, defaultValue: 50 },
    statFormat: '',
    statUnit: '',
    statDecimals: '',
    statShowTrend: true,
    statTrendWindow: 10,
    statTrendUpIsBad: false,
    gaugeConfig: { min: 0, max: 100, zones: [] },
    mapCenterLat: 39.8283,
    mapCenterLon: -98.5795,
    mapZoom: 4,
    mapMarkers: [],
    mapDynamicEnabled: false,
    mapDynamicBucket: '',
    mapDynamicKeyPattern: '>',
    mapDynamicLatPath: '$.lat',
    mapDynamicLonPath: '$.lon',
    mapDynamicLabelPath: '__key_suffix__',
    mapDynamicPopupFields: [],
    mapEnableClustering: false,
    mapFitBoundsOnLoad: false,
    floorplanLocationId: '',
    floorplanBindings: [],
    floorplanShowUnbound: true,
    consoleFontSize: 12,
    consoleShowTimestamp: true,
    publisherDefaultSubject: '',
    publisherDefaultPayload: '',
    publisherTimeout: 2000,
    publisherThingId: '',
    publisherThingTypeOperationId: '',
    statusMappings: [],
    statusDefaultColor: 'var(--color-info)',
    statusDefaultLabel: 'Unknown',
    statusShowStale: true,
    statusStalenessThreshold: 60000,
    statusStaleColor: 'var(--muted)',
    statusStaleLabel: 'Stale',
    markdownContent: '', 
    
    // Scanner Defaults
    scannerKvEnabled: true,
    scannerKvBucket: '',
    scannerKvKeyTemplate: '{value}',
    // No default rules: the record shape is unknown. Older widgets without
    // `rules` get the legacy revoked/expires_at pair in useWidgetForm's hydrate.
    scannerRules: [],
    scannerPbEnabled: false,
    scannerPbCollection: '',
    scannerPbFilter: '',
    scannerPbFields: '',
    scannerPublishEnabled: false,
    scannerPublishSubjectTemplate: 'scans.{purpose}.{scanner}',
    scannerDeviceLabel: '',
    scannerPurpose: 'verify',
    scannerLocation: '',
    scannerDedupWindowMs: 3000,
    scannerLookupTimeoutMs: 5000,
    scannerAllowManualEntry: true,

    // KV Table Defaults
    kvTableBucket: '',
    kvTableKeyPattern: '>',
    kvTableColumns: [],
    kvTableDefaultSortColumn: '',
    kvTableDefaultSortDirection: 'desc',
    kvTableMaxRows: 500,

    // Stream Table Defaults
    streamTableColumns: [],
    streamTableDefaultSortColumn: '',
    streamTableDefaultSortDirection: 'desc',

    useJetStream: false,
    deliverPolicy: 'last',
    jetstreamTimeWindow: '10m',

    hideSystemSubjects: false
  }
}
