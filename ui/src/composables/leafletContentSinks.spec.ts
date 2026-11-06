import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

// Leaflet 1.9 assigns a string tooltip or popup straight to innerHTML, and every
// map label here is user- or device-written data, so a string is stored XSS.
// Leaflet cannot render under vitest, so this scans the source: every content
// call must take a text node or a caller-built element.

const SRC = join(__dirname, '..')

const SINK = /\.(bindTooltip|bindPopup|setTooltipContent|setPopupContent)\s*\(\s*([^,)\r\n]*)/g
const ALLOWED_ARG = /^(textElement\(|popup$)/

// Not /g: a global regex carries lastIndex between .test() calls.
const DIV_ICON = /L\.divIcon\s*\(/
const STRING_HTML = /\bhtml\s*:\s*[`'"]/

function sourceFiles(dir: string): string[] {
  const out: string[] = []
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) out.push(...sourceFiles(p))
    else if (/\.(ts|vue)$/.test(name) && !name.endsWith('.spec.ts')) out.push(p)
  }
  return out
}

describe('Leaflet content sinks', () => {
  it('never hand Leaflet a string', () => {
    const offenders: string[] = []
    let calls = 0
    for (const file of sourceFiles(SRC)) {
      const text = readFileSync(file, 'utf8')
      for (const m of text.matchAll(SINK)) {
        calls++
        const arg = m[2].trim()
        if (!ALLOWED_ARG.test(arg)) {
          const line = text.slice(0, m.index).split(/\r?\n/).length
          offenders.push(`${relative(SRC, file)}:${line}  .${m[1]}(${arg}`)
        }
      }
    }
    // Zero means the regex no longer matches the code and guards nothing.
    expect(calls).toBeGreaterThan(0)
    expect(offenders, 'pass textElement(...) or a built element; see useLeafletMap.ts').toEqual([])
  })

  // divIcon html is the same sink. Only this one audited helper may call it,
  // and it passes an Element, which Leaflet appends instead of using innerHTML.
  // String html is refused everywhere, the helper included.
  const ICON_HELPER = join('utils', 'leafletIcon.ts')

  it('does not accept an html: option (divIcon and friends are the same sink)', () => {
    const files = sourceFiles(SRC)

    const divIconCallers = files
      .filter(f => DIV_ICON.test(readFileSync(f, 'utf8')))
      .map(f => relative(SRC, f))
    expect(divIconCallers).toEqual([ICON_HELPER])

    // The helper has to still BE the thing it is allowed for.
    expect(readFileSync(join(SRC, ICON_HELPER), 'utf8'))
      .toMatch(/html: el as unknown as string/)

    const stringHtml = files
      .filter(f => STRING_HTML.test(readFileSync(f, 'utf8')))
      .map(f => relative(SRC, f))
    expect(stringHtml, 'build an element and pass it to elementIcon()').toEqual([])
  })
})
