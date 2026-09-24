import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

// Leaflet 1.9 assigns a STRING tooltip or popup straight to innerHTML
// (DivOverlay._updateContent). Every map label in this console is data someone
// else wrote -- a location name any member can set, a KV value a device can set,
// a marker label from an imported dashboard -- so a string reaching one of these
// calls is stored XSS, and it has been: member-named locations ran script in
// the session of any owner who opened the map.
//
// Nothing renders Leaflet under vitest (it needs a real layout engine and
// MapLibre needs WebGL), so this reads the source instead: every content call
// must take a text node or a caller-built element, in one of the forms below.
// A new call site that does not match fails here with a pointer to why.

const SRC = join(__dirname, '..')

const SINK = /\.(bindTooltip|bindPopup|setTooltipContent|setPopupContent)\s*\(\s*([^,)\r\n]*)/g
const ALLOWED_ARG = /^(textElement\(|popup$)/

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
    // If this drops to zero the regex has stopped matching the code, and the
    // test would pass while guarding nothing.
    expect(calls).toBeGreaterThan(0)
    expect(offenders, 'pass textElement(...) or a built element; see useLeafletMap.ts').toEqual([])
  })

  it('does not accept an html: option (divIcon and friends are the same sink)', () => {
    const offenders = sourceFiles(SRC).filter(f => /L\.divIcon\s*\(|\bhtml\s*:\s*[`'"]/.test(readFileSync(f, 'utf8')))
    expect(offenders.map(f => relative(SRC, f))).toEqual([])
  })
})
