<script setup lang="ts">
// Printable, operator-branded QR labels for things and locations, sized to real
// label stock.
//
// It takes a LIST. A detail view passes one record; a list view passes its whole
// filtered set. There is deliberately no "bulk mode" — one record is a list of
// one, so there is a single code path and no pair of layouts to keep in step.
//
// The payload is the bare `code` — no host, no organization, no kind token
// (ADR 0002 in platform-docs). Three consequences shape this component:
//
//   Maximum error correction is affordable. A short code at EC level H (30%
//   recovery) is a version-1 or version-2 symbol; the URL form of the same
//   identifier needs version 6, roughly four times the modules on an
//   identically sized sticker. A label lives on a device in a plant room and
//   gets scratched, painted and wiped, so that headroom goes to recovery.
//
//   The human-readable line is not decoration. It is the fallback for the label
//   that will not scan — greasy, torn, or in a closet too dark to focus in —
//   where a tech reads the code aloud or types it into the scanner's manual
//   field. It gets equal billing.
//
//   The customer's name is deliberately NOT printed. A sticker in a public
//   hallway is readable by anyone walking past, and a tenant name beside a
//   device naming convention is free reconnaissance. It is shown on screen,
//   where the operator already knows whose device they are looking at.
//
// The operator's brand IS printed, and it is the one piece of context that
// earns its space: whoever finds a broken device needs to know who services it.
//
// ── Sizing ───────────────────────────────────────────────────────────────────
//
// Everything below is in MILLIMETRES, not pixels. CSS mm maps to physical mm at
// print, so the artwork comes off the printer at the size of the stock instead
// of at whatever the browser happens to map 96 px/inch to. Screen preview is a
// side effect of the same units and is close enough to judge.
//
// Both sizes are common plain thermal stock, and the layout reserves NO RFID
// inlay keep-out. It used to. The band cost the 2″ × 1″ label a third of its
// text column — 19.4 mm for brand, code and name, which wraps a 13-character
// code — to accommodate media this platform cannot use: there is no encoder, no
// reader, and no field on a thing to hold an EPC. A 2″ × 1″ UHF smart label
// barely exists in any case, since a UHF dipole needs length and that stock
// starts around 4″ × 2″; 50 mm smart labels are HF/NFC and a different reader
// entirely. If RFID ever arrives here, it comes back measured against a real
// inlay's datasheet rather than a conservative guess.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import QRCode from 'qrcode'
import { useBrandingStore } from '@/stores/branding'
import { useEscapeKey } from '@/composables/useEscapeKey'

interface LabelSize {
  key: '2x1'
  label: string
  /** Stock dimensions, millimetres. */
  w: number
  h: number
  /** Printer registration margin held clear on every edge. */
  pad: number
  /** Square QR side. Height-bound. */
  qr: number
  /** Point sizes — print-native units, so they survive the mm layout. */
  brandPt: number
  codePt: number
  namePt: number
}

// Geometry is checked rather than eyeballed: with a 4-module quiet zone, the
// worst-case symbol among realistic codes is 33 modules across. At these QR
// sizes that is 0.61 mm per module on the small stock and 1.21 mm on the large
// — both above the ~0.5 mm a phone camera needs.
const SIZES = [
  {
    key: '2x1',
    label: '2″ × 1″',
    w: 50.8,
    h: 25.4,
    pad: 1.5,
    qr: 20,
    brandPt: 4,
    codePt: 8,
    namePt: 5,
  },
  {
    key: '4x2',
    label: '4″ × 2″',
    w: 101.6,
    h: 50.8,
    pad: 3,
    qr: 40,
    brandPt: 7,
    codePt: 16,
    namePt: 9,
  },
] as unknown as LabelSize[]

interface LabelRecord {
  code: string
  name: string
  kind: 'thing' | 'location'
}

const props = defineProps<{
  records: LabelRecord[]
  organizationName?: string
}>()
const emit = defineEmits<{ close: [] }>()

const branding = useBrandingStore()
const labels = ref<(LabelRecord & { dataUrl: string })[]>([])
const error = ref('')
const sizeKey = ref<string>(SIZES[0].key)

// A code is optional on every record that can carry a label, so a filtered set
// will contain some that cannot. Print the rest and name the ones left out —
// dropping them silently means a tech walks a site with a short stack.
const skipped = computed(() => props.records.filter((r) => !r.code).map((r) => r.name || 'Untitled'))

const size = computed(() => SIZES.find((s) => s.key === sizeKey.value) ?? SIZES[0])

// Absolute placement inside a fixed-size box: the most predictable thing a
// print engine can be handed.
//
// A 1 mm gutter is wider than it reads: the symbol carries its own 4-module
// quiet zone INSIDE the image (see `margin` below), adding ~2.4 mm of white on
// the small stock and ~4.8 mm on the large before any glyph starts.
const TEXT_GAP = 1
const textLeft = computed(() => size.value.pad + size.value.qr + TEXT_GAP)

const labelStyle = computed(() => ({
  width: `${size.value.w}mm`,
  height: `${size.value.h}mm`,
}))
const qrStyle = computed(() => ({
  left: `${size.value.pad}mm`,
  width: `${size.value.qr}mm`,
  height: `${size.value.qr}mm`,
}))
const textStyle = computed(() => ({
  left: `${textLeft.value}mm`,
  right: `${size.value.pad}mm`,
}))

async function render() {
  error.value = ''
  const withCode = props.records.filter((r) => r.code)
  try {
    labels.value = await Promise.all(
      withCode.map(async (r) => ({
        ...r,
        dataUrl: await QRCode.toDataURL(r.code, {
          errorCorrectionLevel: 'H',
          // 4 is the quiet zone ISO/IEC 18004 requires, and the library's
          // default. Carrying it inside the image rather than relying on
          // surrounding white space means the clearance is guaranteed by the
          // symbol itself, whatever the label layout does around it.
          margin: 4,
          // Rendered far larger than it prints so the symbol is crisp at any
          // DPI; the mm box below is what sets the physical size.
          width: 1024,
          color: { dark: '#000000', light: '#ffffff' },
        }),
      })),
    )
  } catch (err: any) {
    labels.value = []
    error.value = err?.message || 'Failed to render the QR code'
  }
}

// `records` is an array literal in the parent template, so its identity changes
// on every parent re-render — and a detail view re-renders whenever its live
// NATS data ticks. Watching the array would re-encode every symbol on each tick,
// so watch the codes themselves, which change only when the set does.
const codesKey = computed(() => props.records.map((r) => r.code).join(' '))
watch(codesKey, render)

// @page cannot be expressed in a scoped style block or interpolated from a
// template, so the page box is written imperatively and torn down with the
// modal. Without it the browser prints the labels onto whatever paper size is
// selected, complete with its default margins.
let pageStyleEl: HTMLStyleElement | null = null
function applyPageSize() {
  if (!pageStyleEl) {
    pageStyleEl = document.createElement('style')
    pageStyleEl.id = 'qr-label-page-size'
    document.head.appendChild(pageStyleEl)
  }
  pageStyleEl.textContent = `@page { size: ${size.value.w}mm ${size.value.h}mm; margin: 0; }`
}

onMounted(() => {
  render()
  applyPageSize()
})
watch(size, applyPageSize)
onBeforeUnmount(() => {
  pageStyleEl?.remove()
  pageStyleEl = null
})

// window is not in template scope under <script setup>.
function print() {
  window.print()
}

// Escape closes these; see useEscapeKey for why the dialogs do not get it
// from the browser and which ones are deliberately left out.
// Mounted only while open, so the open state is simply "yes".
useEscapeKey(() => true, () => emit('close'))
</script>

<template>
  <!--
    Teleported to <body> so that the rest of the application is a SIBLING of this
    dialog rather than an ancestor. That is what lets the print rules below drop
    it with display:none. The single-label version could only use
    visibility:hidden — a hidden element still occupies its box — so its one
    label had to be yanked to the page origin with position:absolute. Absolute
    boxes do not fragment across pages predictably, and paginating is the whole
    point of printing more than one.
  -->
  <Teleport to="body">
    <dialog class="modal modal-open qr-label-dialog">
      <div class="modal-box max-w-lg">
        <div class="qr-print-hide">
          <h3 class="font-bold text-lg mb-1">
            {{ labels.length > 1 ? `Labels (${labels.length})` : 'Label' }}
          </h3>
          <p class="text-sm text-base-content/60 mb-4">
            Scanned in the console or the service desk field app.
            <span v-if="organizationName">{{ organizationName }}.</span>
          </p>

          <div v-if="error" class="alert alert-warning py-2 text-sm mb-3">{{ error }}</div>

          <div v-else-if="!labels.length" class="alert alert-warning py-2 text-sm mb-3">
            {{
              records.length > 1
                ? `None of these ${records.length} records has a code, so there is nothing to print.`
                : 'This record has no code, so it cannot carry a label. Add one first.'
            }}
          </div>

          <div v-if="labels.length && skipped.length" class="alert alert-warning py-2 text-sm mb-3 block">
            <div class="font-semibold">{{ skipped.length }} of {{ records.length }} skipped — no code:</div>
            <div class="text-xs mt-1">{{ skipped.join(', ') }}</div>
          </div>

          <div v-if="labels.length" class="join mb-3">
            <button
              v-for="s in SIZES"
              :key="s.key"
              class="btn btn-sm join-item"
              :class="sizeKey === s.key ? 'btn-active' : ''"
              @click="sizeKey = s.key"
            >
              {{ s.label }}
            </button>
          </div>
        </div>

        <!--
          Colours are pinned rather than themed: this goes onto white stock, so a
          dark-theme label would print as a black rectangle.
        -->
        <div v-if="labels.length" class="qr-label-pages flex flex-col items-start gap-2 overflow-auto max-h-[60vh]">
          <div
            v-for="l in labels"
            :key="l.code"
            class="qr-label-page relative bg-white text-black shrink-0"
            :style="labelStyle"
          >
            <img :src="l.dataUrl" alt="" class="absolute top-1/2 -translate-y-1/2" :style="qrStyle" />

            <div class="absolute top-1/2 -translate-y-1/2 overflow-hidden" :style="textStyle">
              <div class="flex items-center gap-1 leading-none">
                <img v-if="branding.logoUrl" :src="branding.logoUrl" alt="" class="object-contain"
                     :style="{ height: `${size.brandPt}pt`, width: `${size.brandPt}pt` }" />
                <span class="uppercase tracking-wide font-semibold truncate"
                      :style="{ fontSize: `${size.brandPt}pt` }">{{ branding.appName }}</span>
              </div>
              <div class="font-mono font-bold leading-tight break-all"
                   :style="{ fontSize: `${size.codePt}pt`, marginTop: '0.6mm' }">{{ l.code }}</div>
              <div class="leading-tight break-words"
                   :style="{ fontSize: `${size.namePt}pt`, marginTop: '0.3mm' }">{{ l.name }}</div>
              <!--
                Things dominate an inventory, so only the rarer kind is marked —
                an unmarked label reads as a device, which is the common case.
              -->
              <div v-if="l.kind === 'location'" class="uppercase tracking-wide leading-none"
                   :style="{ fontSize: `${size.brandPt}pt`, marginTop: '0.4mm' }">Site</div>
            </div>
          </div>
        </div>

        <div class="modal-action mt-4 qr-print-hide">
          <button class="btn btn-ghost btn-sm" @click="emit('close')">Close</button>
          <button class="btn btn-primary btn-sm" :disabled="!labels.length" @click="print">Print</button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop qr-print-hide">
        <button @click.prevent="emit('close')">close</button>
      </form>
    </dialog>
  </Teleport>
</template>

<!--
  Deliberately NOT scoped. The print rules have to reach past this component to
  drop the rest of the application, which a scoped attribute selector cannot do.
  They are inert until this modal mounts, and it only mounts while it is open.
  The @page box itself is written from script — see applyPageSize.
-->
<style>
@media print {
  /* The dialog is teleported to <body>, so the app is a sibling and can be
     removed from the layout outright rather than merely made invisible. */
  body > *:not(.qr-label-dialog) {
    display: none !important;
  }

  /* Strip the modal to a plain block, so each label is an ordinary in-flow child
     of the page — the one pagination case every print engine agrees on. */
  .qr-label-dialog,
  .qr-label-dialog .modal-box,
  .qr-label-pages {
    display: block !important;
    position: static !important;
    width: auto !important;
    max-width: none !important;
    height: auto !important;
    max-height: none !important;
    overflow: visible !important;
    margin: 0 !important;
    padding: 0 !important;
    border: none !important;
    border-radius: 0 !important;
    background: none !important;
    box-shadow: none !important;
  }

  /* Controls, headings and the skipped-records notice are not artwork. */
  .qr-print-hide {
    display: none !important;
  }

  /* The stock IS the page, so no offset, no border, and one label per sheet. */
  .qr-label-page {
    margin: 0 !important;
    border: none !important;
    break-after: page;
  }
  .qr-label-page:last-child {
    break-after: auto;
  }
}
</style>
