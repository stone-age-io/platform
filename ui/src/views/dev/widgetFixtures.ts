// ui/src/views/dev/widgetFixtures.ts
//
// One fixture per widget type, for the /dev/widgets gallery.
//
// WHY A Record<WidgetType, …> AND NOT A LIST. A widget type with no fixture is
// a compile error, which is the only reason this file is worth having: a
// gallery that silently omitted the widget somebody just added would be worse
// than no gallery, because it reads as coverage. Same reasoning as
// `DEFAULT_WIDGET_SIZES` and `WIDGETS`.
//
// WHAT A FIXTURE IS NOT. It is not a test and nothing asserts on it. It is the
// smallest plausible configuration plus enough buffered messages that the
// widget draws its normal state rather than its empty one, so a human can look
// at sixteen widgets in one scroll without a NATS server, a login or a
// dashboard.
//
// Widgets that read NATS directly (KV lookups, the scanner's camera) have no
// messages to give them: they render their disconnected state here, and that
// is the honest result rather than a mock of a server. Their `note` says so.

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
  // Assigned rather than spread: spreading a narrowed widget with a Partial of
  // itself widens `type` back to the union and the result stops being a
  // WidgetConfig at all.
  Object.assign(widget, overrides)
  // Stable ids, so a hot reload refills the buffer it already filled rather
  // than leaking a new one per edit.
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
        // Without subjects the widget draws "Configure one or more subjects"
        // rather than its rows — it reports an unconfigured data source before
        // it looks at the buffer.
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

    switch: make('switch', 'KV-backed toggle. Without a connection it shows its default state.', {
      title: 'Lobby Lights',
    }),

    slider: make('slider', 'Publishes on release. Core-NATS mode, so no KV read is attempted.', {
      title: 'Setpoint',
    }),

    publisher: make('publisher', 'Ad-hoc publish form with history. Needs a connection to send.', {
      title: 'Publish',
    }),

    markdown: make('markdown', 'Static text. {{variable}} tokens resolve against a dashboard, which is absent here.', {
      title: 'Notes',
      markdownConfig: {
        content:
          '### Site S01\n\nMaintenance window is **Sunday 02:00-04:00 UTC**.\n\n- Panel is in the north riser\n- Key is at the front desk\n',
      },
    }),

    map: make('map', 'Static markers with items. The basemap needs network; the markers draw regardless.', {
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
            ],
          },
          { id: 'm2', label: 'Warehouse', lat: 39.1431, lon: -94.5203, items: [] },
        ],
      },
    }),

    kv: make('kv', 'Reads one KV key. Empty without a NATS connection, which is the correct empty state.', {
      title: 'Device State',
      // The default has no bucket, which makes the widget say "config
      // required" — true, but it hides the state this fixture is for.
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
