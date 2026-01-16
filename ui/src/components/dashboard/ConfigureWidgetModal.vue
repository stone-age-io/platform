<template>
  <Teleport to="body">
    <div v-if="modelValue && widgetId" class="modal-overlay" @click.self="close">
      <!-- CHANGED: .modal -> .nd-modal -->
      <div class="nd-modal" :class="{ 'modal-large': widgetType === 'map' }">
        <div class="modal-header">
          <h3>Configure Widget</h3>
          <button class="close-btn" @click="close">✕</button>
        </div>
        <div class="modal-body">
          <!-- Common Title -->
          <ConfigCommon :form="form" :errors="errors" />
          
          <!-- Data Source (Shared by visualization widgets) -->
          <ConfigDataSource 
            v-if="showDataSourceConfig"
            :form="form" 
            :errors="errors" 
            :allow-multiple="widgetType === 'console'"
          />

          <!-- Widget Specific Config (Dynamic) -->
          <component 
            v-if="activeConfigComponent"
            :is="activeConfigComponent"
            :form="form"
            :errors="errors"
          />
          
          <div class="modal-actions">
            <button class="btn-secondary" @click="close">
              Cancel
            </button>
            <button class="btn-primary" @click="save">
              Save Changes
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'
import { useValidation } from '@/composables/useValidation'
import { useWidgetOperations } from '@/composables/useWidgetOperations'
import type { WidgetType, ThresholdRule, MapMarker, StatusMapping } from '@/types/dashboard'
import type { WidgetFormState } from '@/types/config'

// Import sub-components
import ConfigCommon from './config/ConfigCommon.vue'
import ConfigDataSource from './config/ConfigDataSource.vue'
import ConfigText from './config/ConfigText.vue'
import ConfigChart from './config/ConfigChart.vue'
import ConfigStat from './config/ConfigStat.vue'
import ConfigGauge from './config/ConfigGauge.vue'
import ConfigButton from './config/ConfigButton.vue'
import ConfigKv from './config/ConfigKv.vue'
import ConfigSwitch from './config/ConfigSwitch.vue'
import ConfigSlider from './config/ConfigSlider.vue'
import ConfigMap from './config/ConfigMap.vue'
import ConfigConsole from './config/ConfigConsole.vue'
import ConfigPublisher from './config/ConfigPublisher.vue'
import ConfigStatus from './config/ConfigStatus.vue'
import ConfigMarkdown from './config/ConfigMarkdown.vue'
import ConfigPocketBase from './config/ConfigPocketBase.vue'

interface Props {
  modelValue: boolean
  widgetId: string | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const dashboardStore = useDashboardStore()
const validator = useValidation()
const { updateWidgetConfiguration } = useWidgetOperations()

// Component Mapping
const configComponents: Record<string, any> = {
  text: ConfigText,
  chart: ConfigChart,
  stat: ConfigStat,
  gauge: ConfigGauge,
  button: ConfigButton,
  kv: ConfigKv,
  switch: ConfigSwitch,
  slider: ConfigSlider,
  map: ConfigMap,
  console: ConfigConsole,
  publisher: ConfigPublisher,
  status: ConfigStatus,
  markdown: ConfigMarkdown,
  pocketbase: ConfigPocketBase,
}

const form = ref<WidgetFormState>({
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
  useJetStream: false,
  deliverPolicy: 'last',
  jetstreamTimeWindow: '10m',
  pbCollection: '',
  pbFilter: '',
  pbSort: '-created',
  pbFields: '',
  pbLimit: 10,
  pbRefreshInterval: 0
})

const errors = ref<Record<string, string>>({})

const widgetType = computed<WidgetType | null>(() => {
  if (!props.widgetId) return null
  const widget = dashboardStore.getWidget(props.widgetId)
  return widget?.type || null
})

// Dynamic Component Logic
const activeConfigComponent = computed(() => {
  if (!widgetType.value) return null
  return configComponents[widgetType.value] || null
})

// Logic for showing the generic Data Source config block
const showDataSourceConfig = computed(() => {
  const typesWithDataSource = ['text', 'chart', 'stat', 'gauge', 'console']
  if (widgetType.value === 'status' || widgetType.value === 'markdown' || widgetType.value === 'pocketbase') return false
  return typesWithDataSource.includes(widgetType.value || '')
})

watch(() => props.widgetId, (widgetId) => {
  if (!widgetId) return
  
  const widget = dashboardStore.getWidget(widgetId)
  if (!widget) return
  
  let currentSubject = ''
  let currentSubjects: string[] = []

  if (widget.type === 'button') {
    currentSubject = widget.buttonConfig?.publishSubject || ''
  } else if (widget.type === 'switch') {
    currentSubject = widget.switchConfig?.publishSubject || ''
  } else if (widget.type === 'slider') {
    currentSubject = widget.sliderConfig?.publishSubject || ''
  } else {
    currentSubject = widget.dataSource.subject || ''
    if (widget.dataSource.subjects && widget.dataSource.subjects.length > 0) {
      currentSubjects = [...widget.dataSource.subjects]
    } else if (currentSubject) {
      currentSubjects = [currentSubject]
    }
  }

  let currentThresholds: ThresholdRule[] = []
  if (widget.type === 'text') {
    currentThresholds = widget.textConfig?.thresholds ? [...widget.textConfig.thresholds] : []
  } else if (widget.type === 'kv') {
    currentThresholds = widget.kvConfig?.thresholds ? [...widget.kvConfig.thresholds] : []
  } else if (widget.type === 'stat') {
    currentThresholds = widget.statConfig?.thresholds ? [...widget.statConfig.thresholds] : []
  }

  let currentKvBucket = ''
  let currentKvKey = ''
  if (widget.type === 'switch' && widget.switchConfig?.mode === 'kv') {
    currentKvBucket = widget.switchConfig.kvBucket || ''
    currentKvKey = widget.switchConfig.kvKey || ''
  } else if (widget.type === 'slider' && widget.sliderConfig?.mode === 'kv') {
    currentKvBucket = widget.sliderConfig.kvBucket || ''
    currentKvKey = widget.sliderConfig.kvKey || ''
  } else if (widget.type === 'kv') {
    currentKvBucket = widget.dataSource.kvBucket || ''
    currentKvKey = widget.dataSource.kvKey || ''
  } else if ((widget.type === 'status' || widget.type === 'markdown') && widget.dataSource.type === 'kv') {
    currentKvBucket = widget.dataSource.kvBucket || ''
    currentKvKey = widget.dataSource.kvKey || ''
  }

  let mapCenterLat = 39.8283
  let mapCenterLon = -98.5795
  let mapZoom = 4
  let mapMarkers: MapMarker[] = []

  if (widget.type === 'map' && widget.mapConfig) {
    mapCenterLat = widget.mapConfig.center?.lat ?? 39.8283
    mapCenterLon = widget.mapConfig.center?.lon ?? -98.5795
    mapZoom = widget.mapConfig.zoom ?? 4
    mapMarkers = JSON.parse(JSON.stringify(widget.mapConfig.markers || []))
  }

  let consoleFontSize = 12
  let consoleShowTimestamp = true

  if (widget.type === 'console' && widget.consoleConfig) {
    consoleFontSize = widget.consoleConfig.fontSize ?? 12
    consoleShowTimestamp = widget.consoleConfig.showTimestamp ?? true
  }

  let publisherDefaultSubject = ''
  let publisherDefaultPayload = ''
  let publisherTimeout = 2000

  if (widget.type === 'publisher' && widget.publisherConfig) {
    publisherDefaultSubject = widget.publisherConfig.defaultSubject || ''
    publisherDefaultPayload = widget.publisherConfig.defaultPayload || ''
    publisherTimeout = widget.publisherConfig.timeout || 2000
  }

  // Status Config
  let statusMappings: StatusMapping[] = []
  let statusDefaultColor = 'var(--color-info)'
  let statusDefaultLabel = 'Unknown'
  let statusShowStale = true
  let statusStalenessThreshold = 60000
  let statusStaleColor = 'var(--muted)'
  let statusStaleLabel = 'Stale'
  let dataSourceType: 'subscription' | 'kv' | 'none' = 'subscription'

  if (widget.type === 'status') {
    dataSourceType = widget.dataSource.type === 'kv' ? 'kv' : 'subscription'
    if (widget.statusConfig) {
      statusMappings = JSON.parse(JSON.stringify(widget.statusConfig.mappings || []))
      statusDefaultColor = widget.statusConfig.defaultColor || 'var(--color-info)'
      statusDefaultLabel = widget.statusConfig.defaultLabel || 'Unknown'
      statusShowStale = widget.statusConfig.showStale ?? true
      statusStalenessThreshold = widget.statusConfig.stalenessThreshold || 60000
      statusStaleColor = widget.statusConfig.staleColor || 'var(--muted)'
      statusStaleLabel = widget.statusConfig.staleLabel || 'Stale'
    }
  }

  // Markdown Config
  let markdownContent = ''
  if (widget.type === 'markdown') {
    if (widget.dataSource.type === 'kv') dataSourceType = 'kv'
    else if (widget.dataSource.subject) dataSourceType = 'subscription'
    else dataSourceType = 'none'
    
    markdownContent = widget.markdownConfig?.content || ''
  }

  // PocketBase Config
  let pbCollection = ''
  let pbFilter = ''
  let pbSort = '-created'
  let pbFields = ''
  let pbLimit = 10
  let pbRefreshInterval = 0

  if (widget.type === 'pocketbase' && widget.pocketbaseConfig) {
    pbCollection = widget.pocketbaseConfig.collection
    pbFilter = widget.pocketbaseConfig.filter || ''
    pbSort = widget.pocketbaseConfig.sort || '-created'
    pbFields = widget.pocketbaseConfig.fields || ''
    pbLimit = widget.pocketbaseConfig.limit || 10
    pbRefreshInterval = widget.pocketbaseConfig.refreshInterval || 0
  }

  form.value = {
    title: widget.title,
    subject: currentSubject,
    subjects: currentSubjects,
    jsonPath: widget.jsonPath || '',
    bufferSize: widget.buffer.maxCount,
    dataSourceType,
    kvBucket: currentKvBucket,
    kvKey: currentKvKey,
    buttonLabel: widget.buttonConfig?.label || '',
    buttonPayload: widget.buttonConfig?.payload || '',
    buttonColor: widget.buttonConfig?.color || '',
    buttonActionType: widget.buttonConfig?.actionType || 'publish',
    buttonTimeout: widget.buttonConfig?.timeout || 1000,
    thresholds: currentThresholds,
    switchMode: widget.switchConfig?.mode || 'kv',
    switchDefaultState: widget.switchConfig?.defaultState || 'off',
    switchStateSubject: widget.switchConfig?.stateSubject || '',
    switchOnPayload: JSON.stringify(widget.switchConfig?.onPayload || {state: 'on'}),
    switchOffPayload: JSON.stringify(widget.switchConfig?.offPayload || {state: 'off'}),
    switchLabelOn: widget.switchConfig?.labels?.on || 'ON',
    switchLabelOff: widget.switchConfig?.labels?.off || 'OFF',
    switchConfirm: widget.switchConfig?.confirmOnChange || false,
    sliderMode: widget.sliderConfig?.mode || 'core',
    sliderStateSubject: widget.sliderConfig?.stateSubject || '',
    sliderValueTemplate: widget.sliderConfig?.valueTemplate || '{{value}}',
    sliderMin: widget.sliderConfig?.min || 0,
    sliderMax: widget.sliderConfig?.max || 100,
    sliderStep: widget.sliderConfig?.step || 1,
    sliderDefault: widget.sliderConfig?.defaultValue || 50,
    sliderUnit: widget.sliderConfig?.unit || '',
    sliderConfirm: widget.sliderConfig?.confirmOnChange || false,
    statFormat: widget.statConfig?.format || '',
    statUnit: widget.statConfig?.unit || '',
    statShowTrend: widget.statConfig?.showTrend ?? true,
    statTrendWindow: widget.statConfig?.trendWindow || 10,
    gaugeMin: widget.gaugeConfig?.min || 0,
    gaugeMax: widget.gaugeConfig?.max || 100,
    gaugeUnit: widget.gaugeConfig?.unit || '',
    gaugeZones: widget.gaugeConfig?.zones ? [...widget.gaugeConfig.zones] : [],
    mapCenterLat,
    mapCenterLon,
    mapZoom,
    mapMarkers,
    consoleFontSize,
    consoleShowTimestamp,
    publisherDefaultSubject,
    publisherDefaultPayload,
    publisherTimeout,
    statusMappings,
    statusDefaultColor,
    statusDefaultLabel,
    statusShowStale,
    statusStalenessThreshold,
    statusStaleColor,
    statusStaleLabel,
    markdownContent, 
    useJetStream: widget.dataSource.useJetStream || false,
    deliverPolicy: widget.dataSource.deliverPolicy || 'last',
    jetstreamTimeWindow: widget.dataSource.timeWindow || '10m',
    pbCollection,
    pbFilter,
    pbSort,
    pbFields,
    pbLimit,
    pbRefreshInterval
  }
  
  errors.value = {}
}, { immediate: true })

function validate(): boolean {
  errors.value = {}
  const widget = dashboardStore.getWidget(props.widgetId!)
  if (!widget) return false
  
  const titleResult = validator.validateWidgetTitle(form.value.title)
  // Markdown widget allows empty title for cleaner look
  if (!titleResult.valid && widget.type !== 'markdown') errors.value.title = titleResult.error!
  
  // Standard Subscription Validation
  if (['text', 'chart', 'stat', 'gauge', 'console', 'markdown'].includes(widget.type) || (widget.type === 'status' && form.value.dataSourceType === 'subscription')) {
    
    if (widget.type === 'console') {
      if (form.value.subjects.length === 0) {
        errors.value.subjects = 'At least one subject is required'
      }
      for (const sub of form.value.subjects) {
        const res = validator.validateSubject(sub)
        if (!res.valid) {
          errors.value.subjects = `Invalid subject "${sub}": ${res.error}`
          break
        }
      }
    } else if (widget.type === 'markdown') {
      // Validate subject only if in subscription mode
      if (form.value.dataSourceType === 'subscription') {
        if (form.value.subject) {
          const subjectResult = validator.validateSubject(form.value.subject)
          if (!subjectResult.valid) errors.value.subject = subjectResult.error!
        } else {
          errors.value.subject = 'Subject is required in subscription mode'
        }
      }
    } else {
      const subjectResult = validator.validateSubject(form.value.subject)
      if (!subjectResult.valid) errors.value.subject = subjectResult.error!
    }
    
    if (form.value.jsonPath) {
      const jsonResult = validator.validateJsonPath(form.value.jsonPath)
      if (!jsonResult.valid) errors.value.jsonPath = jsonResult.error!
    }
    
    const bufferResult = validator.validateBufferSize(form.value.bufferSize)
    if (!bufferResult.valid) errors.value.bufferSize = bufferResult.error!

  } 
  
  if (widget.type === 'button') {
    const subjectResult = validator.validateSubject(form.value.subject)
    if (!subjectResult.valid) errors.value.subject = subjectResult.error!
    if (!form.value.buttonLabel.trim()) errors.value.buttonLabel = 'Label required'
    if (form.value.buttonPayload) {
      const jsonResult = validator.validateJson(form.value.buttonPayload)
      if (!jsonResult.valid) errors.value.buttonPayload = jsonResult.error!
    }
  } else if (widget.type === 'kv' || ((widget.type === 'status' || widget.type === 'markdown') && form.value.dataSourceType === 'kv')) {
    const bucketResult = validator.validateKvBucket(form.value.kvBucket)
    if (!bucketResult.valid) errors.value.kvBucket = bucketResult.error!
    const keyResult = validator.validateKvKey(form.value.kvKey)
    if (!keyResult.valid) errors.value.kvKey = keyResult.error!
    if (form.value.jsonPath) {
      const jsonResult = validator.validateJsonPath(form.value.jsonPath)
      if (!jsonResult.valid) errors.value.jsonPath = jsonResult.error!
    }
  } else if (widget.type === 'switch') {
    if (form.value.switchMode === 'kv') {
      const bucketResult = validator.validateKvBucket(form.value.kvBucket)
      if (!bucketResult.valid) errors.value.kvBucket = bucketResult.error!
      const keyResult = validator.validateKvKey(form.value.kvKey)
      if (!keyResult.valid) errors.value.kvKey = keyResult.error!
    } else {
      const subjectResult = validator.validateSubject(form.value.subject)
      if (!subjectResult.valid) errors.value.subject = subjectResult.error!
    }
  } else if (widget.type === 'slider') {
    if (form.value.sliderMode === 'kv') {
      const bucketResult = validator.validateKvBucket(form.value.kvBucket)
      if (!bucketResult.valid) errors.value.kvBucket = bucketResult.error!
      const keyResult = validator.validateKvKey(form.value.kvKey)
      if (!keyResult.valid) errors.value.kvKey = keyResult.error!
    } else {
      const subjectResult = validator.validateSubject(form.value.subject)
      if (!subjectResult.valid) errors.value.subject = subjectResult.error!
    }
    if (form.value.jsonPath) {
      const jsonResult = validator.validateJsonPath(form.value.jsonPath)
      if (!jsonResult.valid) errors.value.jsonPath = jsonResult.error!
    }
  } else if (widget.type === 'pocketbase') {
    if (!form.value.pbCollection) errors.value.pbCollection = 'Collection is required'
  }
  
  return Object.keys(errors.value).length === 0
}

function save() {
  if (!props.widgetId) return
  if (!validate()) return
  
  const widget = dashboardStore.getWidget(props.widgetId)
  if (!widget) return
  
  const updates: any = { title: form.value.title.trim() }
  
  if (['text', 'chart', 'stat', 'gauge', 'console'].includes(widget.type)) {
    updates.dataSource = { 
      ...widget.dataSource, 
      subject: form.value.subject.trim(),
      subjects: form.value.subjects,
      useJetStream: form.value.useJetStream,
      deliverPolicy: form.value.deliverPolicy,
      timeWindow: form.value.jetstreamTimeWindow
    }
    updates.jsonPath = form.value.jsonPath.trim() || undefined
    updates.buffer = { maxCount: form.value.bufferSize }
  }
  
  if (widget.type === 'text') {
    updates.textConfig = { ...widget.textConfig, thresholds: [...form.value.thresholds] }
  } else if (widget.type === 'button') {
    updates.buttonConfig = {
      label: form.value.buttonLabel.trim(),
      publishSubject: form.value.subject.trim(),
      payload: form.value.buttonPayload.trim() || '{}',
      color: form.value.buttonColor || undefined,
      actionType: form.value.buttonActionType,
      timeout: form.value.buttonTimeout
    }
  } else if (widget.type === 'kv') {
    updates.dataSource = {
      type: 'kv',
      kvBucket: form.value.kvBucket.trim(),
      kvKey: form.value.kvKey.trim(),
    }
    updates.jsonPath = form.value.jsonPath.trim() || undefined
    updates.kvConfig = { ...widget.kvConfig, thresholds: [...form.value.thresholds] }
  } else if (widget.type === 'switch') {
    updates.switchConfig = {
      mode: form.value.switchMode,
      ...(form.value.switchMode === 'kv' ? {
        kvBucket: form.value.kvBucket.trim(),
        kvKey: form.value.kvKey.trim(),
      } : {
        publishSubject: form.value.subject.trim(),
        defaultState: form.value.switchDefaultState,
        stateSubject: form.value.switchStateSubject.trim() || undefined,
      }),
      onPayload: JSON.parse(form.value.switchOnPayload),
      offPayload: JSON.parse(form.value.switchOffPayload),
      labels: {
        on: form.value.switchLabelOn.trim(),
        off: form.value.switchLabelOff.trim(),
      },
      confirmOnChange: form.value.switchConfirm,
    }
  } else if (widget.type === 'slider') {
    updates.jsonPath = form.value.jsonPath.trim() || undefined
    updates.sliderConfig = {
      mode: form.value.sliderMode,
      ...(form.value.sliderMode === 'kv' ? {
        kvBucket: form.value.kvBucket.trim(),
        kvKey: form.value.kvKey.trim(),
      } : {
        publishSubject: form.value.subject.trim(),
        stateSubject: form.value.sliderStateSubject.trim() || undefined,
      }),
      valueTemplate: form.value.sliderValueTemplate.trim() || '{{value}}',
      min: form.value.sliderMin,
      max: form.value.sliderMax,
      step: form.value.sliderStep,
      defaultValue: form.value.sliderDefault,
      unit: form.value.sliderUnit.trim(),
      confirmOnChange: form.value.sliderConfirm,
    }
  } else if (widget.type === 'stat') {
    updates.statConfig = {
      format: form.value.statFormat.trim() || undefined,
      unit: form.value.statUnit.trim() || undefined,
      showTrend: form.value.statShowTrend,
      trendWindow: form.value.statTrendWindow,
      thresholds: [...form.value.thresholds],
    }
  } else if (widget.type === 'gauge') {
    updates.gaugeConfig = {
      min: form.value.gaugeMin,
      max: form.value.gaugeMax,
      unit: form.value.gaugeUnit.trim() || undefined,
      zones: [...form.value.gaugeZones],
    }
  } else if (widget.type === 'map') {
    updates.mapConfig = {
      center: { lat: form.value.mapCenterLat, lon: form.value.mapCenterLon },
      zoom: form.value.mapZoom,
      markers: JSON.parse(JSON.stringify(form.value.mapMarkers)),
    }
  } else if (widget.type === 'console') {
    updates.consoleConfig = {
      fontSize: form.value.consoleFontSize,
      showTimestamp: form.value.consoleShowTimestamp
    }
  } else if (widget.type === 'publisher') {
    const currentHistory = widget.publisherConfig?.history || []
    updates.publisherConfig = {
      defaultSubject: form.value.publisherDefaultSubject.trim(),
      defaultPayload: form.value.publisherDefaultPayload,
      history: currentHistory,
      timeout: form.value.publisherTimeout
    }
  } else if (widget.type === 'status') {
    if (form.value.dataSourceType === 'kv') {
      updates.dataSource = {
        type: 'kv',
        kvBucket: form.value.kvBucket.trim(),
        kvKey: form.value.kvKey.trim(),
      }
    } else {
      updates.dataSource = { 
        type: 'subscription',
        subject: form.value.subject.trim(),
        useJetStream: form.value.useJetStream,
        deliverPolicy: form.value.deliverPolicy,
        timeWindow: form.value.jetstreamTimeWindow
      }
    }
    updates.jsonPath = form.value.jsonPath.trim() || undefined
    updates.buffer = { maxCount: form.value.bufferSize }
    
    updates.statusConfig = {
      mappings: [...form.value.statusMappings],
      defaultColor: form.value.statusDefaultColor,
      defaultLabel: form.value.statusDefaultLabel,
      showStale: form.value.statusShowStale,
      stalenessThreshold: form.value.statusStalenessThreshold,
      staleColor: form.value.statusStaleColor,
      staleLabel: form.value.statusStaleLabel
    }
  } else if (widget.type === 'markdown') {
    if (form.value.dataSourceType === 'kv') {
      updates.dataSource = {
        type: 'kv',
        kvBucket: form.value.kvBucket.trim(),
        kvKey: form.value.kvKey.trim(),
      }
    } else if (form.value.dataSourceType === 'subscription') {
      updates.dataSource = { 
        type: 'subscription',
        subject: form.value.subject.trim(),
        useJetStream: form.value.useJetStream,
        deliverPolicy: form.value.deliverPolicy,
        timeWindow: form.value.jetstreamTimeWindow
      }
    } else {
      updates.dataSource = {
        type: 'subscription',
        subject: ''
      }
    }
    updates.jsonPath = form.value.jsonPath.trim() || undefined
    updates.markdownConfig = {
      content: form.value.markdownContent
    }
  } else if (widget.type === 'pocketbase') {
    updates.pocketbaseConfig = {
      collection: form.value.pbCollection,
      filter: form.value.pbFilter,
      sort: form.value.pbSort,
      fields: form.value.pbFields,
      limit: form.value.pbLimit,
      refreshInterval: form.value.pbRefreshInterval
    }
  }
  
  updateWidgetConfiguration(props.widgetId, updates)
  emit('saved')
  close()
}

function close() {
  emit('update:modelValue', false)
  errors.value = {}
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(2px);
}

/* CHANGED: .modal -> .nd-modal */
.nd-modal {
  background: oklch(var(--b1));
  border: 1px solid oklch(var(--b3));
  border-radius: 8px;
  width: 90%;
  max-width: 600px;
  max-height: 80vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
}

.nd-modal.modal-large {
  max-width: 800px;
  max-height: 90vh;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid oklch(var(--b3));
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
}

.close-btn {
  background: none;
  border: none;
  color: oklch(var(--bc) / 0.5);
  font-size: 24px;
  cursor: pointer;
  transition: color 0.2s;
}

.close-btn:hover {
  color: oklch(var(--bc));
}

.modal-body {
  padding: 20px;
  overflow-y: auto;
}

.modal-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid oklch(var(--b3));
}

.btn-primary,
.btn-secondary {
  padding: 10px 20px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  border: none;
}

.btn-primary {
  background: oklch(var(--p));
  color: white;
}

.btn-primary:hover {
  background: oklch(var(--pf));
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.1);
  color: oklch(var(--bc));
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.15);
}
</style>
