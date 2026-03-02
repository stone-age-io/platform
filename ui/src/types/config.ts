// ui/src/types/config.ts
import type { ThresholdRule, MapMarker, StatusMapping, KvTableColumn } from './dashboard'

export interface WidgetFormState {
  // Common
  title: string
  subject: string
  subjects: string[]
  jsonPath: string
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
  
  // Thresholds
  thresholds: ThresholdRule[]
  
  // Switch Widget
  switchMode: 'kv' | 'core'
  switchDefaultState: 'on' | 'off'
  switchStateSubject: string
  switchOnPayload: string
  switchOffPayload: string
  switchLabelOn: string
  switchLabelOff: string
  switchConfirm: boolean
  
  // Slider Widget
  sliderMode: 'kv' | 'core'
  sliderStateSubject: string
  sliderValueTemplate: string
  sliderMin: number
  sliderMax: number
  sliderStep: number
  sliderDefault: number
  sliderUnit: string
  sliderConfirm: boolean
  
  // Stat Widget
  statFormat: string
  statUnit: string
  statShowTrend: boolean
  statTrendWindow: number
  
  // Gauge Widget
  gaugeMin: number
  gaugeMax: number
  gaugeUnit: string
  gaugeZones: Array<{ min: number; max: number; color: string }>
  
  // Map Widget
  mapCenterLat: number
  mapCenterLon: number
  mapZoom: number
  mapMarkers: MapMarker[]
  
  // Console Widget
  consoleFontSize: number
  consoleShowTimestamp: boolean

  // Publisher Widget
  publisherDefaultSubject: string
  publisherDefaultPayload: string
  publisherTimeout: number
  
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

  // PocketBase Widget (NEW)
  pbCollection: string
  pbFilter: string
  pbSort: string
  pbFields: string
  pbLimit: number
  pbRefreshInterval: number

  // KV Table Widget
  kvTableBucket: string
  kvTableKeyPattern: string
  kvTableColumns: KvTableColumn[]
  kvTableDefaultSortColumn: string
  kvTableDefaultSortDirection: 'asc' | 'desc'

  // JetStream
  useJetStream: boolean
  deliverPolicy: 'all' | 'last' | 'new' | 'last_per_subject' | 'by_start_time'
  jetstreamTimeWindow: string 
}

export function createEmptyFormState(): WidgetFormState {
  return {
    title: '',
    subject: '',
    subjects: [],
    jsonPath: '',
    bufferSize: 100,
    dataSourceType: 'subscription',
    kvBucket: '',
    kvKey: '',
    buttonLabel: '',
    buttonPayload: '',
    buttonColor: '',
    buttonActionType: 'publish',
    buttonTimeout: 1000,
    thresholds: [],
    switchMode: 'kv',
    switchDefaultState: 'off',
    switchStateSubject: '',
    switchOnPayload: '{"state": "on"}',
    switchOffPayload: '{"state": "off"}',
    switchLabelOn: 'ON',
    switchLabelOff: 'OFF',
    switchConfirm: false,
    sliderMode: 'core',
    sliderStateSubject: '',
    sliderValueTemplate: '{{value}}',
    sliderMin: 0,
    sliderMax: 100,
    sliderStep: 1,
    sliderDefault: 50,
    sliderUnit: '',
    sliderConfirm: false,
    statFormat: '',
    statUnit: '',
    statShowTrend: true,
    statTrendWindow: 10,
    gaugeMin: 0,
    gaugeMax: 100,
    gaugeUnit: '',
    gaugeZones: [],
    mapCenterLat: 39.8283,
    mapCenterLon: -98.5795,
    mapZoom: 4,
    mapMarkers: [],
    consoleFontSize: 12,
    consoleShowTimestamp: true,
    publisherDefaultSubject: '',
    publisherDefaultPayload: '',
    publisherTimeout: 2000,
    statusMappings: [],
    statusDefaultColor: 'var(--color-info)',
    statusDefaultLabel: 'Unknown',
    statusShowStale: true,
    statusStalenessThreshold: 60000,
    statusStaleColor: 'var(--muted)',
    statusStaleLabel: 'Stale',
    markdownContent: '', 
    
    // PocketBase Defaults
    pbCollection: '',
    pbFilter: '',
    pbSort: '-created',
    pbFields: '',
    pbLimit: 10,
    pbRefreshInterval: 0,

    // KV Table Defaults
    kvTableBucket: '',
    kvTableKeyPattern: '>',
    kvTableColumns: [],
    kvTableDefaultSortColumn: '',
    kvTableDefaultSortDirection: 'desc',

    useJetStream: false,
    deliverPolicy: 'last',
    jetstreamTimeWindow: '10m'
  }
}
