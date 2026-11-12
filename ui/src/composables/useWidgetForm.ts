// Form state, hydration, validation and save for the widget configure modal.

import { ref, computed, watch, type Ref } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'
import { useValidation } from '@/composables/useValidation'
import { useWidgetOperations } from '@/composables/useWidgetOperations'
import { createEmptyFormState } from '@/types/config'
import { parseDurationMs } from '@/utils/duration'
import { WIDGETS } from '@/components/dashboard/widgets'
import { padSize, padCells, inGrid } from '@/utils/buttonPad'
import type { WidgetFormState } from '@/types/config'
import { BUTTON_PAD_MAX, type WidgetType, type WidgetConfig, type WidgetConfigOf } from '@/types/dashboard'

// ============================================================================
// TYPES
// ============================================================================

type Validator = ReturnType<typeof useValidation>

// Each handler is typed with its own widget type, not the union, so its config
// field is known to be present.
interface WidgetTypeHandler<T extends WidgetType> {
  /** Widget-specific fields into the flat form state. */
  hydrate?: (widget: WidgetConfigOf<T>, state: WidgetFormState) => void
  validate?: (form: WidgetFormState, errors: Record<string, string>, v: Validator) => void
  /** Form state back into a widget-specific update. */
  buildUpdates?: (form: WidgetFormState, widget: WidgetConfigOf<T>) => Partial<WidgetConfigOf<T>>
}

type WidgetTypeHandlers = { [T in WidgetType]?: WidgetTypeHandler<T> }

/**
 * The one cast: TypeScript cannot correlate `typeHandlers[widget.type]` with
 * `widget`, so the handler is widened to the union here. Handler bodies stay
 * checked against their own narrow type.
 */
function handlerFor(type: WidgetType): WidgetTypeHandler<WidgetType> | undefined {
  return typeHandlers[type] as WidgetTypeHandler<WidgetType> | undefined
}

export interface UseWidgetFormOptions {
  widgetId: Ref<string | null>
  onSaved: () => void
  onClose: () => void
}

// The per-type config components are in `WIDGETS` (components/dashboard/widgets.ts).

// ============================================================================
// SHARED HELPERS
// ============================================================================

/** A blank form field is undefined; anything else not a number is NaN, for validate to catch. */
function optionalNumber(text: string): number | undefined {
  const t = String(text).trim()
  return t ? Number(t) : undefined
}

/** Widgets store their subject in different places. */
function hydrateSubject(widget: WidgetConfig, state: WidgetFormState): void {
  if (widget.type === 'button') {
    state.subject = widget.buttonConfig?.publishSubject || ''
  } else if (widget.type === 'switch' || widget.type === 'slider') {
    // Their subjects are in form.switchConfig and form.sliderConfig.
  } else {
    state.subject = widget.dataSource.subject || ''
    if (widget.dataSource.subjects && widget.dataSource.subjects.length > 0) {
      state.subjects = [...widget.dataSource.subjects]
    } else if (state.subject) {
      state.subjects = [state.subject]
    }
  }
}

/** Widgets store their KV bucket and key in different places. */
function hydrateKvFields(widget: WidgetConfig, state: WidgetFormState): void {
  if (widget.type === 'kv') {
    state.kvBucket = widget.dataSource.kvBucket || ''
    state.kvKey = widget.dataSource.kvKey || ''
  } else if ((widget.type === 'status' || widget.type === 'markdown') && widget.dataSource.type === 'kv') {
    state.kvBucket = widget.dataSource.kvBucket || ''
    state.kvKey = widget.dataSource.kvKey || ''
  }
}

/** Uses the shared data source panel (subject, jsonPath, buffer). */
function usesStandardDataSource(type: WidgetType): boolean {
  return WIDGETS[type].standardDataSource
}

/** Takes a list of subjects (tag input). */
function usesMultipleSubjects(type: WidgetType): boolean {
  return WIDGETS[type].multiSubject
}

function validateSubscriptionFields(
  form: WidgetFormState,
  errors: Record<string, string>,
  widgetType: WidgetType,
  v: Validator
): void {
  if (usesMultipleSubjects(widgetType)) {
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
    if (form.dataSourceType === 'subscription') {
      if (form.subject) {
        const r = v.validateSubject(form.subject)
        if (!r.valid) errors.subject = r.error!
      } else {
        errors.subject = 'Subject is required in subscription mode'
      }
    }
  } else {
    const r = v.validateSubject(form.subject)
    if (!r.valid) errors.subject = r.error!
  }

  if (form.jsonPath) {
    const r = v.validateJsonPath(form.jsonPath)
    if (!r.valid) errors.jsonPath = r.error!
  }

  // Only the shared data source panel shows this field.
  if (form.timestampPath) {
    const r = v.validateJsonPath(form.timestampPath)
    if (!r.valid) errors.timestampPath = r.error!
  }

  const r = v.validateBufferSize(form.bufferSize)
  if (!r.valid) errors.bufferSize = r.error!
}

function needsSubscriptionValidation(widgetType: WidgetType, form: WidgetFormState): boolean {
  if (usesStandardDataSource(widgetType)) return true
  if (widgetType === 'markdown') return true
  if (widgetType === 'status' && form.dataSourceType === 'subscription') return true
  return false
}

/** What a switch and a slider share: KV mode, or a pair of subjects. */
interface ControlSource {
  mode: 'kv' | 'core'
  kvBucket?: string
  kvKey?: string
  publishSubject?: string
  stateSubject?: string
}

function validateControlSource(c: ControlSource, errors: Record<string, string>, v: Validator): void {
  if (c.mode === 'kv') {
    const b = v.validateKvBucket(c.kvBucket || '')
    if (!b.valid) errors.kvBucket = b.error!
    const k = v.validateKvKey(c.kvKey || '')
    if (!k.valid) errors.kvKey = k.error!
  } else {
    const r = v.validateSubject(c.publishSubject || '')
    if (!r.valid) errors.publishSubject = r.error!
  }
}

/** Trimmed, and only the fields the chosen mode uses, so a stale subject is not saved under KV mode. */
function controlSource(c: ControlSource): ControlSource {
  return c.mode === 'kv'
    ? { mode: 'kv', kvBucket: c.kvBucket?.trim() || '', kvKey: c.kvKey?.trim() || '', publishSubject: '', stateSubject: undefined }
    : {
      mode: 'core',
      kvBucket: undefined,
      kvKey: undefined,
      publishSubject: c.publishSubject?.trim() || '',
      stateSubject: c.stateSubject?.trim() || undefined,
    }
}

// ============================================================================
// PER-TYPE HANDLERS
// ============================================================================

const typeHandlers: WidgetTypeHandlers = {

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

  // --- chart ---
  chart: {
    hydrate(widget, state) {
      const cfg = widget.chartConfig
      // pie and gauge were removed; a chart imported with either draws as line.
      state.chartType = cfg?.chartType === 'bar' || cfg?.chartType === 'timeline' ? cfg.chartType : 'line'
      state.chartThresholds = cfg?.thresholds ? cfg.thresholds.map(r => ({ ...r })) : []
      state.chartWindow = cfg?.window || ''
      state.chartYMin = cfg?.yMin != null ? String(cfg.yMin) : ''
      state.chartYMax = cfg?.yMax != null ? String(cfg.yMax) : ''
      // A pre-multi-series chart becomes one series with the widget's jsonPath;
      // saving the form is the migration.
      state.chartSeries = cfg?.series?.length
        ? cfg.series.map(s => ({ ...s }))
        : [{ label: '', path: widget.jsonPath || '$' }]
    },
    validate(form, errors, v) {
      if (form.chartSeries.length === 0) {
        errors.chartSeries = 'Add at least one series'
      }
      for (const [i, s] of form.chartSeries.entries()) {
        const n = `Series ${i + 1}`
        const p = v.validateJsonPath(s.path.trim())
        if (!s.path.trim()) {
          errors.chartSeries = `${n}: a JSONPath is required`
        } else if (!p.valid) {
          errors.chartSeries = `${n}: ${p.error}`
        } else if (s.subject?.trim()) {
          const sub = s.subject.trim()
          const r = v.validateSubject(sub)
          if (!r.valid) {
            errors.chartSeries = `${n}: ${r.error}`
          } else if (/(^|\.)(\*|>)(\.|$)/.test(sub)) {
            // A series matches the subject exactly, so a wildcard would match nothing.
            errors.chartSeries = `${n}: the series subject must be exact (put wildcards in the data source)`
          }
        }
        if (errors.chartSeries) break
      }

      const window = form.chartWindow.trim()
      if (window && parseDurationMs(window) === null) {
        errors.chartWindow = 'Use a duration like 30s, 10m, 1h or 1h30m'
      }
      const yMin = optionalNumber(form.chartYMin)
      const yMax = optionalNumber(form.chartYMax)
      if (Number.isNaN(yMin) || Number.isNaN(yMax)) {
        errors.chartYRange = 'Y-axis ends must be numbers, or blank to fit the data'
      } else if (yMin !== undefined && yMax !== undefined && yMin >= yMax) {
        errors.chartYRange = 'Y-axis min must be below max'
      }

      // The chart hides the data source's Time Window input, so it needs its own.
      if (!window && form.useJetStream && form.deliverPolicy === 'by_start_time') {
        errors.chartWindow = 'Replaying "By Time Window" needs a time window'
      }
    },
    buildUpdates(form, widget) {
      return {
        chartConfig: {
          ...widget.chartConfig,
          chartType: form.chartType,
          series: form.chartSeries.map(s => ({
            label: s.label.trim(),
            path: s.path.trim(),
            subject: s.subject?.trim() || undefined,
          })),
          window: form.chartWindow.trim() || undefined,
          yMin: optionalNumber(form.chartYMin),
          yMax: optionalNumber(form.chartYMax),
          thresholds: form.chartThresholds.length ? form.chartThresholds.map(r => ({ ...r, label: r.label?.trim() || undefined })) : undefined,
        },
        // Each series reads the whole payload, so no widget-level path.
        jsonPath: undefined,
      }
    },
  },

  // --- stat ---
  stat: {
    hydrate(widget, state) {
      state.statFormat = widget.statConfig?.format || ''
      state.statUnit = widget.statConfig?.unit || ''
      state.statDecimals = widget.statConfig?.decimals != null ? String(widget.statConfig.decimals) : ''
      state.statShowTrend = widget.statConfig?.showTrend ?? true
      state.statTrendWindow = widget.statConfig?.trendWindow || 10
      state.statTrendUpIsBad = widget.statConfig?.trendUpIsBad ?? false
      state.thresholds = widget.statConfig?.thresholds ? [...widget.statConfig.thresholds] : []
    },
    validate(form, errors) {
      const d = String(form.statDecimals).trim()
      if (d && !/^[0-6]$/.test(d)) errors.statDecimals = 'A whole number from 0 to 6, or blank for automatic'
    },
    buildUpdates(form) {
      const d = String(form.statDecimals).trim()
      return {
        statConfig: {
          format: form.statFormat.trim() || undefined,
          unit: form.statUnit.trim() || undefined,
          decimals: d ? Number(d) : undefined,
          showTrend: form.statShowTrend,
          trendWindow: form.statTrendWindow,
          trendUpIsBad: form.statTrendUpIsBad || undefined,
          thresholds: [...form.thresholds],
        },
      }
    },
  },

  // --- gauge ---
  gauge: {
    hydrate(widget, state) {
      state.gaugeStyle = widget.gaugeConfig?.style === 'bar' ? 'bar' : 'arc'
      state.gaugeMin = widget.gaugeConfig?.min ?? 0
      state.gaugeMax = widget.gaugeConfig?.max ?? 100
      state.gaugeUnit = widget.gaugeConfig?.unit || ''
      state.gaugeDecimals = widget.gaugeConfig?.decimals != null ? String(widget.gaugeConfig.decimals) : ''
      state.gaugeZones = widget.gaugeConfig?.zones ? [...widget.gaugeConfig.zones] : []
    },
    validate(form, errors) {
      if (!Number.isFinite(form.gaugeMin) || !Number.isFinite(form.gaugeMax) || form.gaugeMax <= form.gaugeMin) {
        errors.gaugeRange = 'Max must be a number above min'
      }
      const d = String(form.gaugeDecimals).trim()
      if (d && !/^[0-6]$/.test(d)) errors.gaugeDecimals = 'A whole number from 0 to 6, or blank for automatic'
    },
    buildUpdates(form) {
      return {
        gaugeConfig: {
          style: form.gaugeStyle === 'bar' ? 'bar' : undefined,
          min: form.gaugeMin,
          max: form.gaugeMax,
          unit: form.gaugeUnit.trim() || undefined,
          decimals: String(form.gaugeDecimals).trim() ? Number(String(form.gaugeDecimals).trim()) : undefined,
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

  // --- buttonpad ---
  buttonpad: {
    hydrate(widget, state) {
      const { rows, cols } = padSize(widget.buttonpadConfig)
      state.buttonpadRows = rows
      state.buttonpadCols = cols
      // Copies: the panel edits cells in place, and Cancel must not change the widget.
      state.buttonpadCells = padCells(widget.buttonpadConfig).map(c => ({ ...c }))
    },
    validate(form, errors, v) {
      const ok = (n: number) => Number.isInteger(n) && n >= 1 && n <= BUTTON_PAD_MAX
      if (!ok(form.buttonpadRows) || !ok(form.buttonpadCols)) {
        errors.buttonpadSize = `Rows and columns must be whole numbers from 1 to ${BUTTON_PAD_MAX}`
        return
      }
      // Cells outside the grid are dropped on save, and the panel says so.
      const kept = form.buttonpadCells.filter(c => inGrid(c, form.buttonpadRows, form.buttonpadCols))
      if (kept.length === 0) {
        errors.buttonpadCells = 'Add at least one button'
        return
      }
      for (const c of kept) {
        const where = `Row ${c.row + 1}, column ${c.col + 1}`
        if (!c.label.trim()) {
          errors.buttonpadCells = `${where}: a label is required`
        } else {
          const s = v.validateSubject(c.subject.trim())
          if (!s.valid) {
            errors.buttonpadCells = `${where}: ${s.error}`
          } else if (c.payload.trim()) {
            const j = v.validateJson(c.payload)
            if (!j.valid) errors.buttonpadCells = `${where}: ${j.error}`
          }
        }
        if (errors.buttonpadCells) break
      }
    },
    buildUpdates(form) {
      const rows = form.buttonpadRows
      const cols = form.buttonpadCols
      return {
        buttonpadConfig: {
          rows,
          cols,
          cells: form.buttonpadCells
            .filter(c => inGrid(c, rows, cols))
            .sort((a, b) => a.row - b.row || a.col - b.col)
            .map(c => ({
              row: c.row,
              col: c.col,
              label: c.label.trim(),
              subject: c.subject.trim(),
              payload: c.payload.trim() || '{}',
              color: c.color || undefined,
            })),
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
  // The form holds the config object and SwitchFields edits it, so there is
  // nothing to hydrate field by field.
  switch: {
    hydrate(widget, state) {
      state.switchConfig = { ...state.switchConfig, ...widget.switchConfig }
    },
    validate(form, errors, v) {
      validateControlSource(form.switchConfig, errors, v)
    },
    buildUpdates(form) {
      const c = form.switchConfig
      return {
        switchConfig: {
          ...controlSource(c),
          // Core mode has no stored state to read, so it needs a default.
          defaultState: c.mode === 'core' ? c.defaultState || 'off' : undefined,
          onPayload: c.onPayload,
          offPayload: c.offPayload,
          labels: { on: c.labels?.on?.trim() || 'ON', off: c.labels?.off?.trim() || 'OFF' },
          confirmOnChange: !!c.confirmOnChange,
          confirmMessage: c.confirmMessage,
        },
      }
    },
  },

  // --- slider ---
  slider: {
    hydrate(widget, state) {
      // The widget keeps its JSONPath on the widget, a marker row in the config.
      // The form edits it in the config; buildUpdates puts it back.
      state.sliderConfig = { ...state.sliderConfig, ...widget.sliderConfig, jsonPath: widget.jsonPath || '' }
    },
    validate(form, errors, v) {
      validateControlSource(form.sliderConfig, errors, v)
      if (form.sliderConfig.jsonPath) {
        const j = v.validateJsonPath(form.sliderConfig.jsonPath)
        if (!j.valid) errors.jsonPath = j.error!
      }
    },
    buildUpdates(form) {
      const { jsonPath, ...c } = form.sliderConfig
      return {
        jsonPath: jsonPath?.trim() || undefined,
        sliderConfig: {
          ...c,
          ...controlSource(c),
          // A cleared number field reads as 0, and a step of 0 never moves.
          step: c.step > 0 ? c.step : 1,
          valueTemplate: c.valueTemplate?.trim() || undefined,
          unit: c.unit?.trim() || '',
          confirmOnChange: !!c.confirmOnChange,
        },
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
        state.mapEnableClustering = widget.mapConfig.enableClustering ?? false
        state.mapFitBoundsOnLoad = widget.mapConfig.fitBoundsOnLoad ?? false
        const dm = widget.mapConfig.dynamicMarkers
        state.mapDynamicEnabled = !!dm
        if (dm) {
          state.mapDynamicBucket = dm.kvBucket || ''
          state.mapDynamicKeyPattern = dm.keyPattern || '>'
          state.mapDynamicLatPath = dm.latPath || '$.lat'
          state.mapDynamicLonPath = dm.lonPath || '$.lon'
          state.mapDynamicLabelPath = dm.labelPath || '__key_suffix__'
          state.mapDynamicPopupFields = JSON.parse(JSON.stringify(dm.popupFields || []))
        }
      }
    },
    buildUpdates(form) {
      const config: any = {
        center: { lat: form.mapCenterLat, lon: form.mapCenterLon },
        zoom: form.mapZoom,
        markers: JSON.parse(JSON.stringify(form.mapMarkers)),
        enableClustering: form.mapEnableClustering,
        fitBoundsOnLoad: form.mapFitBoundsOnLoad,
      }
      if (form.mapDynamicEnabled && form.mapDynamicBucket) {
        config.dynamicMarkers = {
          kvBucket: form.mapDynamicBucket,
          keyPattern: form.mapDynamicKeyPattern,
          latPath: form.mapDynamicLatPath,
          lonPath: form.mapDynamicLonPath,
          labelPath: form.mapDynamicLabelPath,
          popupFields: form.mapDynamicPopupFields.length > 0
            ? JSON.parse(JSON.stringify(form.mapDynamicPopupFields))
            : undefined,
        }
      }
      return { mapConfig: config }
    },
  },

  // --- floorplan ---
  floorplan: {
    hydrate(widget, state) {
      const c = widget.floorplanConfig
      if (!c) return
      state.floorplanLocationId = c.locationId || ''
      state.floorplanShowUnbound = c.showUnbound ?? true
      // Deep copy: the editors mutate bindings in place, and a cancelled edit
      // must not change the live widget.
      state.floorplanBindings = JSON.parse(JSON.stringify(c.bindings || []))
    },
    validate(form, errors) {
      if (!form.floorplanLocationId) {
        errors.floorplanLocationId = 'Choose a location'
      }
    },
    buildUpdates(form) {
      return {
        floorplanConfig: {
          locationId: form.floorplanLocationId,
          showUnbound: form.floorplanShowUnbound,
          // Without a thing type a binding silently matches no marker.
          bindings: JSON.parse(
            JSON.stringify(form.floorplanBindings.filter(b => !!b.thingTypeId))
          ),
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
      state.publisherThingId = widget.publisherConfig?.thingId || ''
      state.publisherThingTypeOperationId = widget.publisherConfig?.thingTypeOperationId || ''
    },
    buildUpdates(form, widget) {
      const currentHistory = widget.publisherConfig?.history || []
      return {
        publisherConfig: {
          defaultSubject: form.publisherDefaultSubject.trim(),
          defaultPayload: form.publisherDefaultPayload,
          history: currentHistory,
          timeout: form.publisherTimeout,
          thingId: form.publisherThingId || undefined,
          thingTypeOperationId: form.publisherThingTypeOperationId || undefined,
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
      // Subscription mode is checked by validateSubscriptionFields.
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
      // Subscription mode is checked by validateSubscriptionFields.
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

  // --- kvtable ---
  kvtable: {
    hydrate(widget, state) {
      if (widget.kvtableConfig) {
        state.kvTableBucket = widget.kvtableConfig.kvBucket || ''
        state.kvTableKeyPattern = widget.kvtableConfig.keyPattern || '>'
        state.kvTableColumns = JSON.parse(JSON.stringify(widget.kvtableConfig.columns || []))
        state.kvTableDefaultSortColumn = widget.kvtableConfig.defaultSortColumn || ''
        state.kvTableDefaultSortDirection = widget.kvtableConfig.defaultSortDirection || 'desc'
        state.kvTableMaxRows = widget.kvtableConfig.maxRows ?? 500
      }
    },
    validate(form, errors) {
      if (!form.kvTableBucket.trim()) errors.kvTableBucket = 'Bucket is required'
      if (!form.kvTableKeyPattern.trim()) errors.kvTableKeyPattern = 'Key pattern is required'
      if (form.kvTableColumns.length === 0) {
        errors.kvTableColumns = 'At least one column is required'
      } else {
        for (const col of form.kvTableColumns) {
          if (!col.label.trim() || !col.path.trim()) {
            errors.kvTableColumns = 'All columns need a label and path'
            break
          }
        }
      }
    },
    buildUpdates(form) {
      return {
        kvtableConfig: {
          kvBucket: form.kvTableBucket.trim(),
          keyPattern: form.kvTableKeyPattern.trim(),
          columns: JSON.parse(JSON.stringify(form.kvTableColumns)),
          defaultSortColumn: form.kvTableDefaultSortColumn || undefined,
          defaultSortDirection: form.kvTableDefaultSortDirection,
          maxRows: form.kvTableMaxRows > 0 ? form.kvTableMaxRows : undefined,
        },
      }
    },
  },

  // --- streamtable ---
  streamtable: {
    hydrate(widget, state) {
      if (widget.streamtableConfig) {
        state.streamTableColumns = JSON.parse(JSON.stringify(widget.streamtableConfig.columns || []))
        state.streamTableDefaultSortColumn = widget.streamtableConfig.defaultSortColumn || ''
        state.streamTableDefaultSortDirection = widget.streamtableConfig.defaultSortDirection || 'desc'
      }
    },
    validate(form, errors) {
      // Subjects are validated by the shared subscription validator (multiSubject in WIDGETS).
      if (form.streamTableColumns.length === 0) {
        errors.streamTableColumns = 'At least one column is required'
      } else {
        for (const col of form.streamTableColumns) {
          if (!col.label.trim() || !col.path.trim()) {
            errors.streamTableColumns = 'All columns need a label and path'
            break
          }
        }
      }
    },
    buildUpdates(form) {
      return {
        streamtableConfig: {
          columns: JSON.parse(JSON.stringify(form.streamTableColumns)),
          defaultSortColumn: form.streamTableDefaultSortColumn || undefined,
          defaultSortDirection: form.streamTableDefaultSortDirection,
        },
      }
    },
  },

  // --- scanner ---
  scanner: {
    hydrate(widget, state) {
      if (widget.scannerConfig) {
        const c = widget.scannerConfig
        state.scannerKvEnabled = c.kvEnabled ?? true
        state.scannerKvBucket = c.kvBucket || ''
        state.scannerKvKeyTemplate = c.kvKeyTemplate || '{value}'
        // Widgets that predate `rules` get the two rules matching the old
        // hardcoded revoked/expires_at check.
        state.scannerRules = c.rules !== undefined
          ? JSON.parse(JSON.stringify(c.rules))
          : [
              { field: 'revoked', op: 'falsy', reason: 'Revoked' },
              { field: 'expires_at', op: 'future', reason: 'Expired' },
            ]
        state.scannerPbEnabled = c.pbEnabled ?? false
        state.scannerPbCollection = c.pbCollection || ''
        state.scannerPbFilter = c.pbFilter || ''
        state.scannerPbFields = c.pbFields || ''
        state.scannerPublishEnabled = c.publishEnabled ?? false
        state.scannerPublishSubjectTemplate =
          c.publishSubjectTemplate || c.publishSubject || 'scans.{purpose}.{scanner}'
        state.scannerDeviceLabel = c.deviceLabel || ''
        state.scannerPurpose = c.scanPurpose || 'verify'
        state.scannerLocation = c.location || ''
        state.scannerDedupWindowMs = c.dedupWindowMs ?? 3000
        state.scannerLookupTimeoutMs = c.lookupTimeoutMs ?? 5000
        state.scannerAllowManualEntry = c.allowManualEntry ?? true
      }
    },
    validate(form, errors) {
      if (form.scannerKvEnabled && !form.scannerKvBucket.trim()) {
        errors.scannerKvBucket = 'KV bucket is required when enabled'
      }
      if (form.scannerPbEnabled && !form.scannerPbCollection.trim()) {
        errors.scannerPbCollection = 'Collection is required when enabled'
      }
      if (!form.scannerKvEnabled && !form.scannerPbEnabled) {
        errors.scannerKvBucket = 'At least one lookup source must be enabled'
      }
      if (form.scannerKvEnabled) {
        const valueOps = new Set(['equals', 'not_equals', 'in', 'not_in'])
        for (let i = 0; i < form.scannerRules.length; i++) {
          const r = form.scannerRules[i]
          if (!r.field || !r.field.trim()) {
            errors[`scannerRules.${i}.field`] = 'Field is required'
          }
          if (valueOps.has(r.op)) {
            const v = r.value
            const empty = v === undefined || v === null || (typeof v === 'string' && v.trim() === '')
            if (empty) {
              errors[`scannerRules.${i}.value`] = 'Value is required for this operator'
            }
          }
        }
      }
      if (form.scannerPublishEnabled && !form.scannerPublishSubjectTemplate.trim()) {
        errors.scannerPublishSubjectTemplate = 'Subject template is required when publish is enabled'
      }
      if (form.scannerDedupWindowMs < 0) {
        errors.scannerDedupWindowMs = 'Dedup window must be 0 or greater'
      }
      if (form.scannerLookupTimeoutMs <= 0) {
        errors.scannerLookupTimeoutMs = 'Lookup timeout must be greater than 0'
      }
    },
    buildUpdates(form) {
      return {
        scannerConfig: {
          kvEnabled: form.scannerKvEnabled,
          kvBucket: form.scannerKvBucket.trim(),
          kvKeyTemplate: form.scannerKvKeyTemplate.trim() || '{value}',
          rules: JSON.parse(JSON.stringify(form.scannerRules)),
          pbEnabled: form.scannerPbEnabled,
          pbCollection: form.scannerPbCollection.trim(),
          pbFilter: form.scannerPbFilter.trim(),
          pbFields: form.scannerPbFields.trim(),
          publishEnabled: form.scannerPublishEnabled,
          publishSubjectTemplate: form.scannerPublishSubjectTemplate.trim(),
          deviceLabel: form.scannerDeviceLabel.trim(),
          scanPurpose: form.scannerPurpose,
          location: form.scannerLocation.trim(),
          dedupWindowMs: form.scannerDedupWindowMs,
          lookupTimeoutMs: form.scannerLookupTimeoutMs,
          allowManualEntry: form.scannerAllowManualEntry,
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
    return WIDGETS[widgetType.value]?.config || null
  })

  const showDataSourceConfig = computed(() => {
    if (!widgetType.value) return false
    return usesStandardDataSource(widgetType.value)
  })

  const isMultiSubject = computed(() =>
    !!widgetType.value && usesMultipleSubjects(widgetType.value)
  )

  // --- Hydration ---

  watch(() => options.widgetId.value, (widgetId) => {
    if (!widgetId) return

    const widget = dashboardStore.getWidget(widgetId)
    if (!widget) return

    const state = createEmptyFormState()

    state.title = widget.title
    state.jsonPath = widget.jsonPath || ''
    state.timestampPath = widget.timestampPath || ''
    state.bufferSize = widget.buffer.maxCount
    state.useJetStream = widget.dataSource.useJetStream || false
    state.deliverPolicy = widget.dataSource.deliverPolicy || 'last'
    state.jetstreamTimeWindow = widget.dataSource.timeWindow || '10m'
    state.hideSystemSubjects = widget.dataSource.hideSystemSubjects ?? false

    hydrateSubject(widget, state)
    hydrateKvFields(widget, state)

    const handler = handlerFor(widget.type)
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

    // Markdown may have no title.
    const titleResult = validator.validateWidgetTitle(form.value.title)
    if (!titleResult.valid && widget.type !== 'markdown') {
      errors.value.title = titleResult.error!
    }

    if (needsSubscriptionValidation(widget.type, form.value)) {
      validateSubscriptionFields(form.value, errors.value, widget.type, validator)
    }

    const handler = handlerFor(widget.type)
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

    const updates: any = { title: form.value.title.trim() }

    if (usesStandardDataSource(widget.type)) {
      const trimmedSubject = form.value.subject.trim()
      // Single-subject types derive `subjects` from the subject field.
      const resolvedSubjects = usesMultipleSubjects(widget.type)
        ? form.value.subjects
        : trimmedSubject ? [trimmedSubject] : []
      updates.dataSource = {
        ...widget.dataSource,
        subject: trimmedSubject,
        subjects: resolvedSubjects,
        useJetStream: form.value.useJetStream,
        deliverPolicy: form.value.deliverPolicy,
        timeWindow: form.value.jetstreamTimeWindow,
        hideSystemSubjects: form.value.hideSystemSubjects,
      }
      updates.jsonPath = form.value.jsonPath.trim() || undefined
      updates.timestampPath = form.value.timestampPath.trim() || undefined
      updates.buffer = { maxCount: form.value.bufferSize }
    }

    const handler = handlerFor(widget.type)
    if (handler?.buildUpdates) {
      Object.assign(updates, handler.buildUpdates(form.value, widget))
    }

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
    isMultiSubject,
    save,
    close,
  }
}
