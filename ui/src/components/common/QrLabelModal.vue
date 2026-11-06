<script setup lang="ts">
// Printable QR labels for things and locations, sized to real label stock.
//
// It takes a list: a detail view passes one record, a list view its whole
// filtered set. One code path, no separate bulk layout.
//
// The payload is the bare `code` (ADR 0002 in platform-docs). That keeps the
// symbol small enough for EC level H on a sticker that gets scratched and
// painted. The printed code is the fallback when a label will not scan.
//
// Everything printed is the organization's own data, never the operator brand.
// The top line is the org code (its name only when it has none): a thing code
// is unique only within its org, and the org code is immutable. The type is not
// printed; the name already says it and the small stock has no line to spare.
//
// Sizing is in millimetres, which map to physical mm at print. Both sizes are
// plain thermal stock with no RFID inlay keep-out: the platform cannot encode
// RFID, and the band cost the small label a third of its text column.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import QRCode from 'qrcode'
import { useAuthStore } from '@/stores/auth'
import { useEscapeKey } from '@/composables/useEscapeKey'
import { fitCodePt } from '@/utils/labelFit'

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
  /** Point sizes (print-native). The org line and the Site marker share orgPt. */
  orgPt: number
  /** The code is sized per label to fill the column; see fitCodePt. */
  codeMinPt: number
  codeMaxPt: number
  namePt: number
}

// The worst realistic symbol is 33 modules with quiet zone: 0.61 mm per module
// on the small stock, 1.21 mm on the large, above the ~0.5 mm a phone needs.
// The small QR stays at 20 mm; anything more comes out of the text column.
//
// Type sizes are set so the tallest possible stack (org line, a two-line code at
// its minimum, a two-line name, the Site marker) fits inside the padding.
const SIZES = [
  {
    key: '2x1',
    label: '2″ × 1″',
    w: 50.8,
    h: 25.4,
    pad: 1.5,
    qr: 20,
    orgPt: 6,
    codeMinPt: 8,
    codeMaxPt: 14,
    namePt: 7,
  },
  {
    key: '4x2',
    label: '4″ × 2″',
    w: 101.6,
    h: 50.8,
    pad: 3,
    qr: 40,
    orgPt: 9,
    codeMinPt: 14,
    codeMaxPt: 28,
    namePt: 12,
  },
] as unknown as LabelSize[]

interface LabelRecord {
  code: string
  name: string
  kind: 'thing' | 'location'
}

const props = defineProps<{
  records: LabelRecord[]
}>()
const emit = defineEmits<{ close: [] }>()

// Every caller lists records from the active organization, so read it here
// rather than as a prop each caller could forget.
const authStore = useAuthStore()
const orgLine = computed(() => authStore.currentOrg?.code || authStore.currentOrg?.name || '')
const labels = ref<(LabelRecord & { dataUrl: string })[]>([])
const error = ref('')
const sizeKey = ref<string>(SIZES[0].key)

// Records without a code are skipped and named; a silent drop is only found at the site.
const skipped = computed(() => props.records.filter((r) => !r.code).map((r) => r.name || 'Untitled'))

const size = computed(() => SIZES.find((s) => s.key === sizeKey.value) ?? SIZES[0])

// Absolute placement in a fixed-size box is what print engines handle most
// predictably. The 1 mm gutter adds to the quiet zone already inside the image.
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
const textWidth = computed(() => size.value.w - textLeft.value - size.value.pad)
// The block is centred, so without a cap an overlong one would push the org line
// off the top. Capped, it clips from the bottom (name and Site marker first).
const textStyle = computed(() => ({
  left: `${textLeft.value}mm`,
  right: `${size.value.pad}mm`,
  maxHeight: `${size.value.h - 2 * size.value.pad}mm`,
}))
const codePt = (code: string) => fitCodePt(code, textWidth.value, size.value.codeMinPt, size.value.codeMaxPt)

async function render() {
  error.value = ''
  const withCode = props.records.filter((r) => r.code)
  try {
    labels.value = await Promise.all(
      withCode.map(async (r) => ({
        ...r,
        dataUrl: await QRCode.toDataURL(r.code, {
          errorCorrectionLevel: 'H',
          // The ISO/IEC 18004 quiet zone, inside the image so the layout cannot
          // eat into it.
          margin: 4,
          // Oversized for crispness at any DPI; the mm box sets the printed size.
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

// `records` is a new array on every parent re-render (each live-data tick on a
// detail view), so watch the codes, or every symbol is re-encoded per tick.
const codesKey = computed(() => props.records.map((r) => r.code).join(' '))
watch(codesKey, render)

// @page cannot live in a scoped style or be interpolated from a template, so the
// per-size page box is written imperatively and removed with the modal. Without
// it the browser prints onto the selected paper size with default margins.
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

// Mounted only while open, so the open state is always true.
useEscapeKey(() => true, () => emit('close'))
</script>

<template>
  <!--
    Teleported to <body> so the rest of the app is a sibling, not an ancestor,
    and the print rules can drop it with display:none. visibility:hidden would
    keep its box, forcing the labels to position:absolute, which does not
    paginate predictably.
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
            <span v-if="authStore.currentOrg?.name">{{ authStore.currentOrg.name }}.</span>
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

        <!-- Colours pinned, not themed: a dark-theme label would print black. -->
        <div v-if="labels.length" class="qr-label-pages flex flex-col items-start gap-2 overflow-auto max-h-[60vh]">
          <div
            v-for="l in labels"
            :key="l.code"
            class="qr-label-page relative bg-white text-black shrink-0"
            :style="labelStyle"
          >
            <img :src="l.dataUrl" alt="" class="absolute top-1/2 -translate-y-1/2" :style="qrStyle" />

            <div class="absolute top-1/2 -translate-y-1/2 overflow-hidden" :style="textStyle">
              <!-- Not uppercased: org codes are lowercase and may be typed. -->
              <div v-if="orgLine" class="qr-label-org font-semibold leading-none truncate"
                   :style="{ fontSize: `${size.orgPt}pt` }">{{ orgLine }}</div>
              <!-- break-words, not break-all: wrap at hyphens, mid-token only if forced. -->
              <div class="qr-label-code font-mono font-bold leading-tight break-words"
                   :style="{ fontSize: `${codePt(l.code)}pt`, marginTop: '0.6mm' }">{{ l.code }}</div>
              <div class="leading-tight break-words line-clamp-2"
                   :style="{ fontSize: `${size.namePt}pt`, marginTop: '0.3mm' }">{{ l.name }}</div>
              <!-- Only the rarer kind is marked; unmarked reads as a device. -->
              <div v-if="l.kind === 'location'" class="uppercase tracking-wide leading-none"
                   :style="{ fontSize: `${size.orgPt}pt`, marginTop: '0.4mm' }">Site</div>
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
  Deliberately not scoped: the print rules must reach past this component to
  drop the rest of the app. They apply only while the modal is mounted (open).
  The @page box is written from script; see applyPageSize.
-->
<style>
@media print {
  /* Teleported, so the app is a sibling and can be removed outright. */
  body > *:not(.qr-label-dialog) {
    display: none !important;
  }

  /* Strip the modal to plain blocks so each label is an in-flow child of the
     page, the one pagination case every print engine agrees on. */
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

  .qr-print-hide {
    display: none !important;
  }

  /* The stock is the page: one label per sheet. */
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
