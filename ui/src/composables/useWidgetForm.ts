// ui/src/composables/useWidgetForm.ts
//
// Extracted from ConfigureWidgetModal.vue — all form state management,
// hydration, validation, and save logic for widget configuration.

import { ref, computed, watch, type Ref, type Component } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'
import { useValidation } from '@/composables/useValidation'
import { useWidgetOperations } from '@/composables/useWidgetOperations'
import { createEmptyFormState } from '@/types/config'
import type { WidgetFormState } from '@/types/config'
import type { WidgetType, WidgetConfig } from '@/types/dashboard'

// Config sub-components
import ConfigText from '@/components/dashboard/config/ConfigText.vue'
import ConfigChart from '@/components/dashboard/config/ConfigChart.vue'
import ConfigStat from '@/components/dashboard/config/ConfigStat.vue'
import ConfigGauge from '@/components/dashboard/config/ConfigGauge.vue'
import ConfigButton from '@/components/dashboard/config/ConfigButton.vue'
import ConfigKv from '@/components/dashboard/config/ConfigKv.vue'
import ConfigSwitch from '@/components/dashboard/config/ConfigSwitch.vue'
import ConfigSlider from '@/components/dashboard/config/ConfigSlider.vue'
import ConfigMap from '@/components/dashboard/config/ConfigMap.vue'
import ConfigConsole from '@/components/dashboard/config/ConfigConsole.vue'
import ConfigPublisher from '@/components/dashboard/config/ConfigPublisher.vue'
import ConfigStatus from '@/components/dashboard/config/ConfigStatus.vue'
import ConfigMarkdown from '@/components/dashboard/config/ConfigMarkdown.vue'
import ConfigPocketBase from '@/components/dashboard/config/ConfigPocketBase.vue'

// ============================================================================
// TYPES
// ============================================================================

type Validator = ReturnType<typeof useValidation>

interface WidgetTypeHandler {
  /** Extract widget-specific fields from WidgetConfig into flat form state */
  hydrate?: (widget: WidgetConfig, state: WidgetFormState) => void
  /** Add widget-specific validation errors */
  validate?: (form: WidgetFormState, errors: Record<string, string>, v: Validator) => void
  /** Build widget-specific update payload from form state */
  buildUpdates?: (form: WidgetFormState, widget: WidgetConfig) => Partial<WidgetConfig>
}

export interface UseWidgetFormOptions {
  widgetId: Ref<string | null>
  onSaved: () => void
  onClose: () => void
}

// ============================================================================
// COMPONENT MAPPING
// ============================================================================

const configComponents: Record<string, Component> = {
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

// ============================================================================
// SHARED HELPERS
// ============================================================================

/** Extract subject/subjects from widget config — different widgets store it in different places */
function hydrateSubject(widget: WidgetConfig, state: WidgetFormState): void {
  if (widget.type === 'button') {
    state.subject = widget.buttonConfig?.publishSubject || ''
  } else if (widget.type === 'switch') {
    state.subject = widget.switchConfig?.publishSubject || ''
  } else if (widget.type === 'slider') {
    state.subject = widget.sliderConfig?.publishSubject || ''
  } else {
    state.subject = widget.dataSource.subject || ''
    if (widget.dataSource.subjects && widget.dataSource.subjects.length > 0) {
      state.subjects = [...widget.dataSource.subjects]
    } else if (state.subject) {
      state.subjects = [state.subject]
    }
  }
}

/** Extract kvBucket/kvKey from widget config — multiple types use KV in different ways */
function hydrateKvFields(widget: WidgetConfig, state: WidgetFormState): void {
  if (widget.type === 'switch' && widget.switchConfig?.mode === 'kv') {
    state.kvBucket = widget.switchConfig.kvBucket || ''
    state.kvKey = widget.switchConfig.kvKey || ''
  } else if (widget.type === 'slider' && widget.sliderConfig?.mode === 'kv') {
    state.kvBucket = widget.sliderConfig.kvBucket || ''
    state.kvKey = widget.sliderConfig.kvKey || ''
  } else if (widget.type === 'kv') {
    state.kvBucket = widget.dataSource.kvBucket || ''
    state.kvKey = widget.dataSource.kvKey || ''
  } else if ((widget.type === 'status' || widget.type === 'markdown') && widget.dataSource.type === 'kv') {
    state.kvBucket = widget.dataSource.kvBucket || ''
    state.kvKey = widget.dataSource.kvKey || ''
  }
}

/** Types that use the standard shared data source config (subject + jsonPath + buffer) */
const STANDARD_DATA_SOURCE_TYPES: WidgetType[] = ['text', 'chart', 'stat', 'gauge', 'console']

/** Validate subscription-related fields shared across visualization widgets */
function validateSubscriptionFields(
  form: WidgetFormState,
  errors: Record<string, string>,
  widgetType: WidgetType,
  v: Validator
): void {
  // Console uses multi-subject
  if (widgetType === 'console') {
    if (form.subjects.length === 0) {
      errors.subjects = 'At least one subject is required'
    }
    for (const sub of form.subjects) {
      const res = v.validateSubject(sub)
      if (!res.valid) {
        errors.subjects = `Invalid subject "${sub}": ${res.error}`
        break
      }
    }
  } else if (widgetType === 'markdown') {
    // Markdown: validate subject only in subscription mode
    if (form.dataSourceType === 'subscription') {
      if (form.subject) {
        const r = v.validateSubject(form.subject)
        if (!r.valid) errors.subject = r.error!
      } else {
        errors.subject = 'Subject is required in subscription mode'
      }
    }
  } else {
    // Standard single-subject
    const r = v.validateSubject(form.subject)
    if (!r.valid) errors.subject = r.error!
  }

  // JSONPath (optional for all)
  if (form.jsonPath) {
    const r = v.validateJsonPath(form.jsonPath)
    if (!r.valid) errors.jsonPath = r.error!
  }

  // Buffer size
  const r = v.validateBufferSize(form.bufferSize)
  if (!r.valid) errors.bufferSize = r.error!
}

/** Should this widget type go through shared subscription validation? */
function needsSubscriptionValidation(widgetType: WidgetType, form: WidgetFormState): boolean {
  if (STANDARD_DATA_SOURCE_TYPES.includes(widgetType)) return true
  if (widgetType === 'markdown') return true
  if (widgetType === 'status' && form.dataSourceType === 'subscription') return true
  return false
}

// ============================================================================
// PER-TYPE HANDLERS
// ============================================================================

const typeHandlers: Partial<Record<WidgetType, WidgetTypeHandler>> = {

  // --- text ---
  text: {
    hydrate(widget, state) {
      state.thresholds = widget.textConfig?.thresholds ? [...widget.textConfig.thresholds] : []
    },
    buildUpdates(form, widget) {
      return {
        textConfig: { ...widget.textConfig, thresholds: [...form.thresholds] },
      }
    },
  },

  // --- chart (no type-specific config) ---
  chart: {},

  // --- stat ---
  stat: {
    hydrate(widget, state) {
      state.statFormat = widget.statConfig?.format || ''
      state.statUnit = widget.statConfig?.unit || ''
      state.statShowTrend = widget.statConfig?.showTrend ?? true
      state.statTrendWindow = widget.statConfig?.trendWindow || 10
      state.thresholds = widget.statConfig?.thresholds ? [...widget.statConfig.thresholds] : []
    },
    buildUpdates(form) {
      return {
        statConfig: {
          format: form.statFormat.trim() || undefined,
          unit: form.statUnit.trim() || undefined,
          showTrend: form.statShowTrend,
          trendWindow: form.statTrendWindow,
          thresholds: [...form.thresholds],
        },
      }
    },
  },

  // --- gauge ---
  gauge: {
    hydrate(widget, state) {
      state.gaugeMin = widget.gaugeConfig?.min || 0
      state.gaugeMax = widget.gaugeConfig?.max || 100
      state.gaugeUnit = widget.gaugeConfig?.unit || ''
      state.gaugeZones = widget.gaugeConfig?.zones ? [...widget.gaugeConfig.zones] : []
    },
    buildUpdates(form) {
      return {
        gaugeConfig: {
          min: form.gaugeMin,
          max: form.gaugeMax,
          unit: form.gaugeUnit.trim() || undefined,
          zones: [...form.gaugeZones],
        },
      }
    },
  },

  // --- console ---
  console: {
    hydrate(widget, state) {
      state.consoleFontSize = widget.consoleConfig?.fontSize ?? 12
      state.consoleShowTimestamp = widget.consoleConfig?.showTimestamp ?? true
    },
    buildUpdates(form) {
      return {
        consoleConfig: {
          fontSize: form.consoleFontSize,
          showTimestamp: form.consoleShowTimestamp,
        },
      }
    },
  },

  // --- button ---
  button: {
    hydrate(widget, state) {
      state.buttonLabel = widget.buttonConfig?.label || ''
      state.buttonPayload = widget.buttonConfig?.payload || ''
      state.buttonColor = widget.buttonConfig?.color || ''
      state.buttonActionType = widget.buttonConfig?.actionType || 'publish'
      state.buttonTimeout = widget.buttonConfig?.timeout || 1000
    },
    validate(form, errors, v) {
      const r = v.validateSubject(form.subject)
      if (!r.valid) errors.subject = r.error!
      if (!form.buttonLabel.trim()) errors.buttonLabel = 'Label required'
      if (form.buttonPayload) {
        const j = v.validateJson(form.buttonPayload)
        if (!j.valid) errors.buttonPayload = j.error!
      }
    },
    buildUpdates(form) {
      return {
        buttonConfig: {
          label: form.buttonLabel.trim(),
          publishSubject: form.subject.trim(),
          payload: form.buttonPayload.trim() || '{}',
          color: form.buttonColor || undefined,
          actionType: form.buttonActionType,
          timeout: form.buttonTimeout,
        },
      }
    },
  },

  // --- kv ---
  kv: {
    hydrate(widget, state) {
      state.thresholds = widget.kvConfig?.thresholds ? [...widget.kvConfig.thresholds] : []
    },
    validate(form, errors, v) {
      const b = v.validateKvBucket(form.kvBucket)
      if (!b.valid) errors.kvBucket = b.error!
      const k = v.validateKvKey(form.kvKey)
      if (!k.valid) errors.kvKey = k.error!
      if (form.jsonPath) {
        const j = v.validateJsonPath(form.jsonPath)
        if (!j.valid) errors.jsonPath = j.error!
      }
    },
    buildUpdates(form, widget) {
      return {
        dataSource: {
          type: 'kv' as const,
          kvBucket: form.kvBucket.trim(),
          kvKey: form.kvKey.trim(),
        },
        jsonPath: form.jsonPath.trim() || undefined,
        kvConfig: { ...widget.kvConfig, thresholds: [...form.thresholds] },
      }
    },
  },

  // --- switch ---
  switch: {
    hydrate(widget, state) {
      state.switchMode = widget.switchConfig?.mode || 'kv'
      state.switchDefaultState = widget.switchConfig?.defaultState || 'off'
      state.switchStateSubject = widget.switchConfig?.stateSubject || ''
      state.switchOnPayload = JSON.stringify(widget.switchConfig?.onPayload || { state: 'on' })
      state.switchOffPayload = JSON.stringify(widget.switchConfig?.offPayload || { state: 'off' })
      state.switchLabelOn = widget.switchConfig?.labels?.on || 'ON'
      state.switchLabelOff = widget.switchConfig?.labels?.off || 'OFF'
      state.switchConfirm = widget.switchConfig?.confirmOnChange || false
    },
    validate(form, errors, v) {
      if (form.switchMode === 'kv') {
        const b = v.validateKvBucket(form.kvBucket)
        if (!b.valid) errors.kvBucket = b.error!
        const k = v.validateKvKey(form.kvKey)
        if (!k.valid) errors.kvKey = k.error!
      } else {
        const r = v.validateSubject(form.subject)
        if (!r.valid) errors.subject = r.error!
      }
    },
    buildUpdates(form) {
      const config: any = {
        mode: form.switchMode,
        onPayload: JSON.parse(form.switchOnPayload),
        offPayload: JSON.parse(form.switchOffPayload),
        labels: {
          on: form.switchLabelOn.trim(),
          off: form.switchLabelOff.trim(),
        },
        confirmOnChange: form.switchConfirm,
      }
      if (form.switchMode === 'kv') {
        config.kvBucket = form.kvBucket.trim()
        config.kvKey = form.kvKey.trim()
        config.publishSubject = ''
      } else {
        config.publishSubject = form.subject.trim()
        config.defaultState = form.switchDefaultState
        config.stateSubject = form.switchStateSubject.trim() || undefined
      }
      return { switchConfig: config }
    },
  },

  // --- slider ---
  slider: {
    hydrate(widget, state) {
      state.sliderMode = widget.sliderConfig?.mode || 'core'
      state.sliderStateSubject = widget.sliderConfig?.stateSubject || ''
      state.sliderValueTemplate = widget.sliderConfig?.valueTemplate || '{{value}}'
      state.sliderMin = widget.sliderConfig?.min || 0
      state.sliderMax = widget.sliderConfig?.max || 100
      state.sliderStep = widget.sliderConfig?.step || 1
      state.sliderDefault = widget.sliderConfig?.defaultValue || 50
      state.sliderUnit = widget.sliderConfig?.unit || ''
      state.sliderConfirm = widget.sliderConfig?.confirmOnChange || false
    },
    validate(form, errors, v) {
      if (form.sliderMode === 'kv') {
        const b = v.validateKvBucket(form.kvBucket)
        if (!b.valid) errors.kvBucket = b.error!
        const k = v.validateKvKey(form.kvKey)
        if (!k.valid) errors.kvKey = k.error!
      } else {
        const r = v.validateSubject(form.subject)
        if (!r.valid) errors.subject = r.error!
      }
      if (form.jsonPath) {
        const j = v.validateJsonPath(form.jsonPath)
        if (!j.valid) errors.jsonPath = j.error!
      }
    },
    buildUpdates(form) {
      const config: any = {
        mode: form.sliderMode,
        valueTemplate: form.sliderValueTemplate.trim() || '{{value}}',
        min: form.sliderMin,
        max: form.sliderMax,
        step: form.sliderStep,
        defaultValue: form.sliderDefault,
        unit: form.sliderUnit.trim(),
        confirmOnChange: form.sliderConfirm,
      }
      if (form.sliderMode === 'kv') {
        config.kvBucket = form.kvBucket.trim()
        config.kvKey = form.kvKey.trim()
        config.publishSubject = ''
      } else {
        config.publishSubject = form.subject.trim()
        config.stateSubject = form.sliderStateSubject.trim() || undefined
      }
      return {
        jsonPath: form.jsonPath.trim() || undefined,
        sliderConfig: config,
      }
    },
  },

  // --- map ---
  map: {
    hydrate(widget, state) {
      if (widget.mapConfig) {
        state.mapCenterLat = widget.mapConfig.center?.lat ?? 39.8283
        state.mapCenterLon = widget.mapConfig.center?.lon ?? -98.5795
        state.mapZoom = widget.mapConfig.zoom ?? 4
        state.mapMarkers = JSON.parse(JSON.stringify(widget.mapConfig.markers || []))
      }
    },
    buildUpdates(form) {
      return {
        mapConfig: {
          center: { lat: form.mapCenterLat, lon: form.mapCenterLon },
          zoom: form.mapZoom,
          markers: JSON.parse(JSON.stringify(form.mapMarkers)),
        },
      }
    },
  },

  // --- publisher ---
  publisher: {
    hydrate(widget, state) {
      state.publisherDefaultSubject = widget.publisherConfig?.defaultSubject || ''
      state.publisherDefaultPayload = widget.publisherConfig?.defaultPayload || ''
      state.publisherTimeout = widget.publisherConfig?.timeout || 2000
    },
    buildUpdates(form, widget) {
      const currentHistory = widget.publisherConfig?.history || []
      return {
        publisherConfig: {
          defaultSubject: form.publisherDefaultSubject.trim(),
          defaultPayload: form.publisherDefaultPayload,
          history: currentHistory,
          timeout: form.publisherTimeout,
        },
      }
    },
  },

  // --- status ---
  status: {
    hydrate(widget, state) {
      state.dataSourceType = widget.dataSource.type === 'kv' ? 'kv' : 'subscription'
      if (widget.statusConfig) {
        state.statusMappings = JSON.parse(JSON.stringify(widget.statusConfig.mappings || []))
        state.statusDefaultColor = widget.statusConfig.defaultColor || 'var(--color-info)'
        state.statusDefaultLabel = widget.statusConfig.defaultLabel || 'Unknown'
        state.statusShowStale = widget.statusConfig.showStale ?? true
        state.statusStalenessThreshold = widget.statusConfig.stalenessThreshold || 60000
        state.statusStaleColor = widget.statusConfig.staleColor || 'var(--muted)'
        state.statusStaleLabel = widget.statusConfig.staleLabel || 'Stale'
      }
    },
    validate(form, errors, v) {
      // KV mode validation (subscription mode handled by shared validateSubscriptionFields)
      if (form.dataSourceType === 'kv') {
        const b = v.validateKvBucket(form.kvBucket)
        if (!b.valid) errors.kvBucket = b.error!
        const k = v.validateKvKey(form.kvKey)
        if (!k.valid) errors.kvKey = k.error!
        if (form.jsonPath) {
          const j = v.validateJsonPath(form.jsonPath)
          if (!j.valid) errors.jsonPath = j.error!
        }
      }
    },
    buildUpdates(form) {
      const updates: any = {}
      if (form.dataSourceType === 'kv') {
        updates.dataSource = {
          type: 'kv',
          kvBucket: form.kvBucket.trim(),
          kvKey: form.kvKey.trim(),
        }
      } else {
        updates.dataSource = {
          type: 'subscription',
          subject: form.subject.trim(),
          useJetStream: form.useJetStream,
          deliverPolicy: form.deliverPolicy,
          timeWindow: form.jetstreamTimeWindow,
        }
      }
      updates.jsonPath = form.jsonPath.trim() || undefined
      updates.buffer = { maxCount: form.bufferSize }
      updates.statusConfig = {
        mappings: [...form.statusMappings],
        defaultColor: form.statusDefaultColor,
        defaultLabel: form.statusDefaultLabel,
        showStale: form.statusShowStale,
        stalenessThreshold: form.statusStalenessThreshold,
        staleColor: form.statusStaleColor,
        staleLabel: form.statusStaleLabel,
      }
      return updates
    },
  },

  // --- markdown ---
  markdown: {
    hydrate(widget, state) {
      if (widget.dataSource.type === 'kv') {
        state.dataSourceType = 'kv'
      } else if (widget.dataSource.subject) {
        state.dataSourceType = 'subscription'
      } else {
        state.dataSourceType = 'none'
      }
      state.markdownContent = widget.markdownConfig?.content || ''
    },
    validate(form, errors, v) {
      // KV mode validation (subscription mode handled by shared validateSubscriptionFields)
      if (form.dataSourceType === 'kv') {
        const b = v.validateKvBucket(form.kvBucket)
        if (!b.valid) errors.kvBucket = b.error!
        const k = v.validateKvKey(form.kvKey)
        if (!k.valid) errors.kvKey = k.error!
        if (form.jsonPath) {
          const j = v.validateJsonPath(form.jsonPath)
          if (!j.valid) errors.jsonPath = j.error!
        }
      }
    },
    buildUpdates(form) {
      const updates: any = {}
      if (form.dataSourceType === 'kv') {
        updates.dataSource = {
          type: 'kv',
          kvBucket: form.kvBucket.trim(),
          kvKey: form.kvKey.trim(),
        }
      } else if (form.dataSourceType === 'subscription') {
        updates.dataSource = {
          type: 'subscription',
          subject: form.subject.trim(),
          useJetStream: form.useJetStream,
          deliverPolicy: form.deliverPolicy,
          timeWindow: form.jetstreamTimeWindow,
        }
      } else {
        updates.dataSource = {
          type: 'subscription',
          subject: '',
        }
      }
      updates.jsonPath = form.jsonPath.trim() || undefined
      updates.markdownConfig = {
        content: form.markdownContent,
      }
      return updates
    },
  },

  // --- pocketbase ---
  pocketbase: {
    hydrate(widget, state) {
      if (widget.pocketbaseConfig) {
        state.pbCollection = widget.pocketbaseConfig.collection
        state.pbFilter = widget.pocketbaseConfig.filter || ''
        state.pbSort = widget.pocketbaseConfig.sort || '-created'
        state.pbFields = widget.pocketbaseConfig.fields || ''
        state.pbLimit = widget.pocketbaseConfig.limit || 10
        state.pbRefreshInterval = widget.pocketbaseConfig.refreshInterval || 0
      }
    },
    validate(form, errors) {
      if (!form.pbCollection) errors.pbCollection = 'Collection is required'
    },
    buildUpdates(form) {
      return {
        pocketbaseConfig: {
          collection: form.pbCollection,
          filter: form.pbFilter,
          sort: form.pbSort,
          fields: form.pbFields,
          limit: form.pbLimit,
          refreshInterval: form.pbRefreshInterval,
        },
      }
    },
  },
}

// ============================================================================
// COMPOSABLE
// ============================================================================

export function useWidgetForm(options: UseWidgetFormOptions) {
  const dashboardStore = useDashboardStore()
  const validator = useValidation()
  const { updateWidgetConfiguration } = useWidgetOperations()

  const form = ref<WidgetFormState>(createEmptyFormState())
  const errors = ref<Record<string, string>>({})

  // --- Computed ---

  const widgetType = computed<WidgetType | null>(() => {
    if (!options.widgetId.value) return null
    const widget = dashboardStore.getWidget(options.widgetId.value)
    return widget?.type || null
  })

  const activeConfigComponent = computed(() => {
    if (!widgetType.value) return null
    return configComponents[widgetType.value] || null
  })

  const showDataSourceConfig = computed(() => {
    if (!widgetType.value) return false
    // Status, markdown, pocketbase manage their own data source UI
    if (widgetType.value === 'status' || widgetType.value === 'markdown' || widgetType.value === 'pocketbase') return false
    return STANDARD_DATA_SOURCE_TYPES.includes(widgetType.value)
  })

  // --- Hydration ---

  watch(() => options.widgetId.value, (widgetId) => {
    if (!widgetId) return

    const widget = dashboardStore.getWidget(widgetId)
    if (!widget) return

    // Start from clean defaults
    const state = createEmptyFormState()

    // Common fields
    state.title = widget.title
    state.jsonPath = widget.jsonPath || ''
    state.bufferSize = widget.buffer.maxCount
    state.useJetStream = widget.dataSource.useJetStream || false
    state.deliverPolicy = widget.dataSource.deliverPolicy || 'last'
    state.jetstreamTimeWindow = widget.dataSource.timeWindow || '10m'

    // Shared cross-type field extraction
    hydrateSubject(widget, state)
    hydrateKvFields(widget, state)

    // Type-specific hydration
    const handler = typeHandlers[widget.type]
    if (handler?.hydrate) {
      handler.hydrate(widget, state)
    }

    form.value = state
    errors.value = {}
  }, { immediate: true })

  // --- Validation ---

  function validate(): boolean {
    errors.value = {}
    const widget = dashboardStore.getWidget(options.widgetId.value!)
    if (!widget) return false

    // Common: title (markdown allows empty title for cleaner look)
    const titleResult = validator.validateWidgetTitle(form.value.title)
    if (!titleResult.valid && widget.type !== 'markdown') {
      errors.value.title = titleResult.error!
    }

    // Shared subscription validation for applicable types
    if (needsSubscriptionValidation(widget.type, form.value)) {
      validateSubscriptionFields(form.value, errors.value, widget.type, validator)
    }

    // Type-specific validation
    const handler = typeHandlers[widget.type]
    if (handler?.validate) {
      handler.validate(form.value, errors.value, validator)
    }

    return Object.keys(errors.value).length === 0
  }

  // --- Save ---

  function save(): void {
    if (!options.widgetId.value) return
    if (!validate()) return

    const widget = dashboardStore.getWidget(options.widgetId.value)
    if (!widget) return

    // Common update
    const updates: any = { title: form.value.title.trim() }

    // Standard data source update for visualization widgets
    if (STANDARD_DATA_SOURCE_TYPES.includes(widget.type)) {
      updates.dataSource = {
        ...widget.dataSource,
        subject: form.value.subject.trim(),
        subjects: form.value.subjects,
        useJetStream: form.value.useJetStream,
        deliverPolicy: form.value.deliverPolicy,
        timeWindow: form.value.jetstreamTimeWindow,
      }
      updates.jsonPath = form.value.jsonPath.trim() || undefined
      updates.buffer = { maxCount: form.value.bufferSize }
    }

    // Type-specific updates
    const handler = typeHandlers[widget.type]
    if (handler?.buildUpdates) {
      Object.assign(updates, handler.buildUpdates(form.value, widget))
    }

    // DEBUG: trace subject through save pipeline
    console.log('[WidgetForm] save()', {
      widgetType: widget.type,
      formSubject: form.value.subject,
      updatesDataSourceSubject: updates.dataSource?.subject,
      updatesDataSource: JSON.parse(JSON.stringify(updates.dataSource || {})),
    })

    updateWidgetConfiguration(options.widgetId.value, updates)
    options.onSaved()
    close()
  }

  // --- Close ---

  function close(): void {
    options.onClose()
    errors.value = {}
  }

  return {
    form,
    errors,
    widgetType,
    activeConfigComponent,
    showDataSourceConfig,
    save,
    close,
  }
}
