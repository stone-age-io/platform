// One fixture per widget type, for the /dev/widgets gallery (dev builds only).
//
// A Record<WidgetType, …>, not a list, so a widget type with no fixture is a
// compile error. Nothing asserts on a fixture: it is just enough config and
// buffered messages for the widget to draw its normal state. Widgets that read
// NATS directly render their disconnected state, and their `note` says so.

import { createDefaultWidget, type WidgetConfig, type WidgetConfigOf, type WidgetType } from '@/types/dashboard'

export interface FixtureMessage {
  timestamp: number
  value: any
  raw?: any
  subject?: string
}

export interface DevFixture {
  /** Shown above the widget, so an empty-looking panel is explained rather than suspicious. */
  note: string
  widget: WidgetConfig
  messages: FixtureMessage[]
}

/** A rising-then-falling value, so charts and trends have a shape rather than a flat line. */
function wave(i: number, n: number, min: number, max: number): number {
  const t = i / Math.max(1, n - 1)
  const mid = (min + max) / 2
  return Math.round((mid + Math.sin(t * Math.PI * 2) * (max - min) * 0.3) * 10) / 10
}

/** `count` messages ending now, one every `everyMs`. */
function series(
  count: number,
  everyMs: number,
  build: (i: number, count: number) => { value: any; raw?: any; subject?: string }
): FixtureMessage[] {
  const now = Date.now()
  const out: FixtureMessage[] = []
  for (let i = 0; i < count; i++) {
    out.push({ timestamp: now - (count - 1 - i) * everyMs, ...build(i, count) })
  }
  return out
}

function make<T extends WidgetType>(
  type: T,
  note: string,
  overrides: Partial<WidgetConfigOf<T>>,
  messages: FixtureMessage[] = []
): DevFixture {
  const widget = createDefaultWidget(type, { x: 0, y: 0 })
  // Not a spread: that widens `type` back to the union and breaks WidgetConfig.
  Object.assign(widget, overrides)
  // Stable ids, so a hot reload reuses its buffer instead of leaking one.
  widget.id = `dev_${type}`
  return { note, widget, messages }
}

export function buildFixtures(): Record<WidgetType, DevFixture> {
  return {
    text: make(
      'text',
      'Latest value, with thresholds colouring it.',
      {
        title: 'Return Air Temp',
        textConfig: {
          fontSize: 32,
          thresholds: [
            { id: 't1', operator: '>', value: '26', color: 'var(--color-error)' },
            { id: 't2', operator: '>', value: '22', color: 'var(--color-warning)' },
          ],
        },
      },
      series(5, 5_000, (i, n) => {
        const v = wave(i, n, 18, 28)
        return { value: v, raw: { value: v } }
      })
    ),

    stat: make(
      'stat',
      'KPI with a trend arrow against the value ten messages back.',
      {
        title: 'Packets / sec',
        statConfig: { unit: '/s', showTrend: true, trendWindow: 10, thresholds: [] },
      },
      series(30, 2_000, (i, n) => {
        const v = wave(i, n, 400, 900)
        return { value: v, raw: { value: v } }
      })
    ),

    gauge: make(
      'gauge',
      'Zoned meter. The fixture value sits in the warning band.',
      { title: 'Compressor Load' },
      series(5, 5_000, () => ({ value: 72, raw: { value: 72 } }))
    ),

    status: make(
      'status',
      'Mapped state plus the staleness watchdog. The last message is recent, so it is not stale.',
      { title: 'Gateway' },
      series(3, 10_000, () => ({ value: 'online', raw: { value: 'online' } }))
    ),

    chart: make(
      'chart',
      'Two series read out of one payload, which is the case a per-series subject exists to separate.',
      {
        title: 'Supply vs Return',
        chartConfig: {
          chartType: 'line',
          series: [
            { label: 'Supply', path: '$.supply' },
            { label: 'Return', path: '$.return' },
          ],
          thresholds: [
            { id: 'th_high', operator: '>', value: '25', color: 'var(--color-error)', label: 'High return' },
          ],
          yMin: 5,
          yMax: 30,
        },
      },
      series(120, 15_000, (i, n) => {
        const raw = { supply: wave(i, n, 10, 16), return: wave(i, n, 20, 26) }
        return { value: raw.supply, raw }
      })
    ),

    console: make(
      'console',
      'Live log stream. Subjects and payloads are click-to-copy.',
      { title: 'All Traffic' },
      series(40, 1_500, (i) => {
        const code = ['AHU-1', 'AHU-2', 'VAV-07'][i % 3]
        const payload = { code, temp: 20 + (i % 7), ok: i % 11 !== 0 }
        return { subject: `telemetry.thing.${code}.state`, value: payload, raw: payload }
      })
    ),

    streamtable: make(
      'streamtable',
      'Ring buffer of messages as rows, newest first.',
      {
        title: 'Recent Messages',
        // Without subjects the widget shows "Configure one or more subjects"
        // and never reads the buffer.
        dataSource: { type: 'subscription', subjects: ['telemetry.thing.>'] },
        streamtableConfig: {
          columns: [
            { id: 'col_subject', label: 'Subject', path: '__subject__', format: 'text' },
            { id: 'col_code', label: 'Code', path: '$.code', format: 'text' },
            { id: 'col_temp', label: 'Temp', path: '$.temp', format: 'number' },
            { id: 'col_timestamp', label: 'Time', path: '__timestamp__', format: 'relative-time' },
          ],
          defaultSortDirection: 'desc',
        },
      },
      series(25, 3_000, (i) => {
        const code = ['AHU-1', 'AHU-2', 'VAV-07'][i % 3]
        const payload = { code, temp: 20 + (i % 7) }
        return { subject: `telemetry.thing.${code}.state`, value: payload, raw: payload }
      })
    ),

    button: make('button', 'Publishes on click. Nothing is connected here, so a click reports a failure.', {
      title: 'Reboot Gateway',
    }),

    // The default pad is a D-pad, which shows the gaps; this adds colour.
    buttonpad: make('buttonpad', 'A 3x3 D-pad with empty corners. Nothing is connected here, so the keys are disabled.', {
      title: 'Navigation',
      buttonpadConfig: {
        rows: 3,
        cols: 3,
        cells: [
          { row: 0, col: 0, label: 'Back', subject: 'tv.key', payload: '{"key":"BACK"}', color: 'var(--panel)' },
          { row: 0, col: 1, label: '↑', subject: 'tv.key', payload: '{"key":"UP"}' },
          { row: 0, col: 2, label: 'Home', subject: 'tv.key', payload: '{"key":"HOME"}', color: 'var(--panel)' },
          { row: 1, col: 0, label: '←', subject: 'tv.key', payload: '{"key":"LEFT"}' },
          { row: 1, col: 1, label: 'OK', subject: 'tv.key', payload: '{"key":"OK"}', color: 'var(--color-secondary)' },
          { row: 1, col: 2, label: '→', subject: 'tv.key', payload: '{"key":"RIGHT"}' },
          { row: 2, col: 1, label: '↓', subject: 'tv.key', payload: '{"key":"DOWN"}' },
        ],
      },
    }),

    switch: make('switch', 'KV-backed toggle. Without a connection it shows its default state.', {
      title: 'Lobby Lights',
    }),

    slider: make('slider', 'Publishes on release. Core-NATS mode, so no KV read is attempted.', {
      title: 'Setpoint',
    }),

    publisher: make('publisher', 'Ad-hoc publish form with history. Needs a connection to send.', {
      title: 'Publish',
    }),

    markdown: make('markdown', 'Static text. {variable} tokens resolve against a dashboard, which is absent here.', {
      title: 'Notes',
      markdownConfig: {
        content:
          '### Site S01\n\nMaintenance window is **Sunday 02:00-04:00 UTC**.\n\n- Panel is in the north riser\n- Key is at the front desk\n',
      },
    }),

    map: make('map', 'Static markers with items, including the setpoint and level rows. The basemap needs network; the markers draw regardless.', {
      title: 'Sites',
      mapConfig: {
        center: { lat: 39.0997, lon: -94.5786 },
        zoom: 11,
        enableClustering: false,
        fitBoundsOnLoad: true,
        markers: [
          {
            id: 'm1',
            label: 'HQ',
            lat: 39.0997,
            lon: -94.5786,
            color: 'var(--color-success)',
            items: [
              {
                id: 'i1',
                type: 'publish',
                label: 'Unlock',
                subject: 'cmd.thing.DOOR-1.unlock',
                payload: '{}',
                actionType: 'publish',
                timeout: 1000,
              },
              // Both read NATS directly, so offline they show their empty state.
              {
                id: 'i2',
                type: 'slider',
                label: 'Setpoint',
                sliderConfig: {
                  mode: 'kv',
                  kvBucket: 'twin_desired',
                  kvKey: 'thing.AHU-1.setpoint',
                  min: 15,
                  max: 28,
                  step: 0.5,
                  defaultValue: 21,
                  unit: 'C',
                },
              },
              {
                id: 'i3',
                type: 'gauge',
                label: 'Tank level',
                gaugeConfig: {
                  mode: 'kv',
                  kvBucket: 'twin',
                  kvKey: 'thing.TANK-1.level',
                  min: 0,
                  max: 100,
                  unit: '%',
                  zones: [
                    { min: 0, max: 20, color: 'var(--color-error)' },
                    { min: 20, max: 80, color: 'var(--color-success)' },
                    { min: 80, max: 100, color: 'var(--color-warning)' },
                  ],
                },
              },
            ],
          },
          { id: 'm2', label: 'Warehouse', lat: 39.1431, lon: -94.5203, items: [] },
        ],
      },
    }),

    floorplan: make('floorplan', 'Needs a location with an uploaded plan and a PocketBase session; shows its no-location state here.', {
      title: 'North Riser',
      floorplanConfig: {
        // Empty on purpose: a real id is "Location not found" in every other org.
        locationId: '',
        showUnbound: true,
        bindings: [
          {
            id: 'fpb_doors',
            thingTypeId: 'demo_door_controller',
            stateSource: { mode: 'subject', subjectSuffix: 'state', jsonPath: '$.state' },
            mappings: [
              { id: 'm1', value: 'secure', color: 'var(--color-success)', label: 'Secure', blink: false },
              { id: 'm2', value: 'forced', color: 'var(--color-error)', label: 'Forced', blink: true },
            ],
            defaultColor: 'var(--color-info)',
            showStale: true,
            stalenessThreshold: 60000,
            items: [
              {
                id: 'i1',
                type: 'text',
                label: 'State',
                textConfig: { subject: '{thing_type_code}.{thing}.state', jsonPath: '$.state' },
              },
              {
                id: 'i2',
                type: 'gauge',
                label: 'Battery',
                gaugeConfig: {
                  mode: 'kv',
                  kvBucket: 'twin',
                  kvKey: 'thing.{thing}.battery',
                  min: 0,
                  max: 100,
                  unit: '%',
                  zones: [{ min: 0, max: 20, color: 'var(--color-error)' }],
                },
              },
            ],
          },
        ],
      },
    }),

    kv: make('kv', 'Reads one KV key. Empty without a NATS connection, which is the correct empty state.', {
      title: 'Device State',
      // The default has no bucket and would show "config required" instead.
      dataSource: { type: 'kv', kvBucket: 'twin', kvKey: 'thing.AHU-1.state' },
    }),

    kvtable: make('kvtable', 'Watches a KV prefix. Empty without a NATS connection.', {
      title: 'Fleet',
    }),

    scanner: make('scanner', 'Needs a camera and a KV bucket. Shows its idle state here.', {
      title: 'Muster Scan',
    }),
  }
}
