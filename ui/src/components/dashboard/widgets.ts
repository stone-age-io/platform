// Every static fact about a widget type, in one object. Background in ui/CLAUDE.md
// ("Widgets live in one object").
//
// A map, not a plugin system: no import.meta.glob, manifest, self-registration
// or schema-driven forms. `Record<WidgetType, WidgetDefinition>` is the
// checklist: a type with no entry, or an entry missing a required field, is a
// compile error.
//
// Order is load-bearing: the Add Widget palette renders in declaration order,
// grouped by `category` in WIDGET_CATEGORIES order.

import { defineAsyncComponent, type Component } from 'vue'
import {
  ChartColumn, ChartLine, CircleDot, Database, FileText, Gauge,
  LayoutPanelTop, Map as MapIcon, MousePointerClick, RadioTower, ScanQrCode, Send,
  SlidersHorizontal, Table, Terminal, ToggleRight, Type,
} from '@lucide/vue'
import type { WidgetType } from '@/types/dashboard'

// Light widgets are imported eagerly. The heavy ones below are deferred, so a
// dashboard without a chart never downloads ECharts.
import TextWidget from '@/components/widgets/TextWidget.vue'
import ButtonWidget from '@/components/widgets/ButtonWidget.vue'
import SwitchWidget from '@/components/widgets/SwitchWidget.vue'
import SliderWidget from '@/components/widgets/SliderWidget.vue'
import StatCardWidget from '@/components/widgets/StatCardWidget.vue'
import StatusWidget from '@/components/widgets/StatusWidget.vue'

const ChartWidget = defineAsyncComponent(() => import('@/components/widgets/ChartWidget.vue'))
const GaugeWidget = defineAsyncComponent(() => import('@/components/widgets/GaugeWidget.vue'))
const MapWidget = defineAsyncComponent(() => import('@/components/widgets/MapWidget.vue'))
const FloorPlanWidget = defineAsyncComponent(() => import('@/components/widgets/FloorPlanWidget.vue'))
const KvWidget = defineAsyncComponent(() => import('@/components/widgets/KvWidget.vue'))
const ConsoleWidget = defineAsyncComponent(() => import('@/components/widgets/ConsoleWidget.vue'))
const PublisherWidget = defineAsyncComponent(() => import('@/components/widgets/PublisherWidget.vue'))
const MarkdownWidget = defineAsyncComponent(() => import('@/components/widgets/MarkdownWidget.vue'))
const KvTableWidget = defineAsyncComponent(() => import('@/components/widgets/KvTableWidget.vue'))
const StreamTableWidget = defineAsyncComponent(() => import('@/components/widgets/StreamTableWidget.vue'))
const ScannerWidget = defineAsyncComponent(() => import('@/components/widgets/ScannerWidget.vue'))

// Config panels are all deferred: they are needed only in the configure modal,
// and ConfigMap alone pulls in the marker editors.
const ConfigText = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigText.vue'))
const ConfigChart = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigChart.vue'))
const ConfigStat = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigStat.vue'))
const ConfigGauge = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigGauge.vue'))
const ConfigButton = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigButton.vue'))
const ConfigKv = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigKv.vue'))
const ConfigSwitch = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigSwitch.vue'))
const ConfigSlider = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigSlider.vue'))
const ConfigMap = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigMap.vue'))
const ConfigFloorPlan = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigFloorPlan.vue'))
const ConfigConsole = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigConsole.vue'))
const ConfigPublisher = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigPublisher.vue'))
const ConfigStatus = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigStatus.vue'))
const ConfigMarkdown = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigMarkdown.vue'))
const ConfigKvTable = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigKvTable.vue'))
const ConfigStreamTable = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigStreamTable.vue'))
const ConfigScanner = defineAsyncComponent(() => import('@/components/dashboard/config/ConfigScanner.vue'))

/** Palette groupings, rendered in this order. */
export const WIDGET_CATEGORIES = ['Data Display', 'Controls', 'Layout', 'Tools'] as const
export type WidgetCategory = (typeof WIDGET_CATEGORIES)[number]

export interface WidgetDefinition {
  /** Rendered on the dashboard. */
  component: Component
  /** The panel in the configure modal. Deferred; see above. */
  config: Component

  // --- Add Widget palette ---
  label: string
  /** A lucide icon. The same one is the widget's empty-state icon where it has one. */
  icon: Component
  description: string
  category: WidgetCategory

  // --- Locked mobile dashboards ---
  // A locked mobile dashboard hides widget headers; these two flags are the
  // exceptions.
  /** Keep the header (and so the title) on a locked mobile dashboard. */
  mobileTitled: boolean
  /** Offer a full-screen button there too — for widgets too dense to read inline. */
  mobileExpandable: boolean
  /**
   * On a locked phone dashboard: 'full' takes the whole row, 'half' pairs with
   * a neighbour. A KV Value sized past 2x2 on the desktop is the one exception,
   * and DashboardGrid checks it there rather than here.
   */
  mobileWidth: 'full' | 'half'
  /**
   * Minimum height on a phone. A widget body is a size container, so it has
   * no height of its own to grow into: this IS its height.
   * 'auto' 80px (a status line), 'tile' 150px (one number), 'tall' 300px (a
   * chart or a log), 'data' 400px (tables and maps).
   */
  mobileHeight: 'auto' | 'tile' | 'tall' | 'data'

  // --- Configure modal ---
  /**
   * Show the shared subject / JSONPath / buffer panel.
   *
   * False means the widget either publishes rather than subscribes (button,
   * switch, slider, publisher), or draws its own data-source UI because its
   * source is a KV bucket or a prefix (kv, kvtable, map, status, markdown,
   * scanner).
   */
  standardDataSource: boolean
  /** Take a list of subjects in that panel rather than one. */
  multiSubject: boolean
}

export const WIDGETS: Record<WidgetType, WidgetDefinition> = {
  // --- Data Display ---
  text: {
    component: TextWidget, config: ConfigText,
    label: 'Text', icon: Type, description: 'Display latest value', category: 'Data Display',
    mobileTitled: false, mobileExpandable: false,
    mobileWidth: 'half', mobileHeight: 'auto',
    standardDataSource: true, multiSubject: false,
  },
  stat: {
    component: StatCardWidget, config: ConfigStat,
    label: 'Stat Card', icon: ChartColumn, description: 'KPI with trend', category: 'Data Display',
    mobileTitled: true, mobileExpandable: false,
    mobileWidth: 'half', mobileHeight: 'tile',
    standardDataSource: true, multiSubject: false,
  },
  gauge: {
    component: GaugeWidget, config: ConfigGauge,
    label: 'Gauge', icon: Gauge, description: 'Circular meter', category: 'Data Display',
    mobileTitled: true, mobileExpandable: false,
    mobileWidth: 'half', mobileHeight: 'tile',
    standardDataSource: true, multiSubject: false,
  },
  status: {
    component: StatusWidget, config: ConfigStatus,
    label: 'Status', icon: CircleDot, description: 'State mapping & watchdog', category: 'Data Display',
    mobileTitled: false, mobileExpandable: false,
    mobileWidth: 'half', mobileHeight: 'auto',
    standardDataSource: false, multiSubject: false,
  },
  chart: {
    component: ChartWidget, config: ConfigChart,
    label: 'Chart', icon: ChartLine, description: 'Line chart over time', category: 'Data Display',
    mobileTitled: true, mobileExpandable: false,
    mobileWidth: 'full', mobileHeight: 'tall',
    standardDataSource: true, multiSubject: true,
  },
  kv: {
    component: KvWidget, config: ConfigKv,
    label: 'KV Value', icon: Database, description: 'Single KV entry', category: 'Data Display',
    mobileTitled: false, mobileExpandable: false,
    mobileWidth: 'half', mobileHeight: 'auto',
    standardDataSource: false, multiSubject: false,
  },
  kvtable: {
    component: KvTableWidget, config: ConfigKvTable,
    label: 'KV Table', icon: Table, description: 'Live KV bucket table', category: 'Data Display',
    mobileTitled: true, mobileExpandable: true,
    mobileWidth: 'full', mobileHeight: 'data',
    standardDataSource: false, multiSubject: false,
  },
  streamtable: {
    component: StreamTableWidget, config: ConfigStreamTable,
    label: 'Stream Table', icon: RadioTower, description: 'Live message stream as a table', category: 'Data Display',
    mobileTitled: true, mobileExpandable: true,
    mobileWidth: 'full', mobileHeight: 'data',
    standardDataSource: true, multiSubject: true,
  },

  // --- Controls ---
  button: {
    component: ButtonWidget, config: ConfigButton,
    label: 'Button', icon: MousePointerClick, description: 'Publish messages', category: 'Controls',
    mobileTitled: false, mobileExpandable: false,
    mobileWidth: 'half', mobileHeight: 'auto',
    standardDataSource: false, multiSubject: false,
  },
  switch: {
    component: SwitchWidget, config: ConfigSwitch,
    label: 'Switch', icon: ToggleRight, description: 'Toggle control', category: 'Controls',
    mobileTitled: false, mobileExpandable: false,
    mobileWidth: 'half', mobileHeight: 'auto',
    standardDataSource: false, multiSubject: false,
  },
  slider: {
    component: SliderWidget, config: ConfigSlider,
    label: 'Slider', icon: SlidersHorizontal, description: 'Range control', category: 'Controls',
    mobileTitled: false, mobileExpandable: false,
    mobileWidth: 'full', mobileHeight: 'auto',
    standardDataSource: false, multiSubject: false,
  },
  publisher: {
    component: PublisherWidget, config: ConfigPublisher,
    label: 'Publisher', icon: Send, description: 'Send ad-hoc messages', category: 'Controls',
    mobileTitled: false, mobileExpandable: false,
    mobileWidth: 'full', mobileHeight: 'tall',
    standardDataSource: false, multiSubject: false,
  },

  // --- Layout ---
  markdown: {
    component: MarkdownWidget, config: ConfigMarkdown,
    label: 'Markdown', icon: FileText, description: 'Static text & images', category: 'Layout',
    mobileTitled: true, mobileExpandable: true,
    mobileWidth: 'full', mobileHeight: 'tall',
    standardDataSource: false, multiSubject: false,
  },
  map: {
    component: MapWidget, config: ConfigMap,
    label: 'Map', icon: MapIcon, description: 'Geographic location', category: 'Layout',
    mobileTitled: true, mobileExpandable: true,
    mobileWidth: 'full', mobileHeight: 'data',
    standardDataSource: false, multiSubject: false,
  },
  floorplan: {
    component: FloorPlanWidget, config: ConfigFloorPlan,
    label: 'Floor Plan', icon: LayoutPanelTop, description: "A location's plan, live", category: 'Layout',
    mobileTitled: true, mobileExpandable: true,
    mobileWidth: 'full', mobileHeight: 'data',
    // It subscribes per BINDING rather than per widget, so the shared
    // subject/JSONPath panel would configure a source nothing reads.
    standardDataSource: false, multiSubject: false,
  },
  console: {
    component: ConsoleWidget, config: ConfigConsole,
    label: 'Console', icon: Terminal, description: 'Live log stream', category: 'Layout',
    mobileTitled: true, mobileExpandable: true,
    mobileWidth: 'full', mobileHeight: 'tall',
    standardDataSource: true, multiSubject: true,
  },

  // --- Tools ---
  scanner: {
    component: ScannerWidget, config: ConfigScanner,
    label: 'Scanner', icon: ScanQrCode, description: 'QR code lookup', category: 'Tools',
    mobileTitled: true, mobileExpandable: true,
    mobileWidth: 'full', mobileHeight: 'data',
    standardDataSource: false, multiSubject: false,
  },
}
