import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

// Leaflet positions its panes from its own stylesheet. Without it the panes are
// static and the marker pane lands below the map, clipped out of view, with no
// error. A stylesheet is global once any module imports it, so a missing import
// hides whenever another map is on the same screen. Every file that creates a
// map imports the CSS itself.

const SRC = join(__dirname, '..')

const CREATES_MAP = /\bL\.map\s*\(/
const IMPORTS_CSS = /import\s+['"]leaflet\/dist\/leaflet\.css['"]/

function sourceFiles(dir: string): string[] {
  const out: string[] = []
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) out.push(...sourceFiles(p))
    else if (/\.(ts|vue)$/.test(name) && !name.endsWith('.spec.ts')) out.push(p)
  }
  return out
}

describe('Leaflet stylesheet', () => {
  it('is imported by every file that creates a map', () => {
    const creators: string[] = []
    const missing: string[] = []

    for (const file of sourceFiles(SRC)) {
      const text = readFileSync(file, 'utf8')
      if (!CREATES_MAP.test(text)) continue
      creators.push(relative(SRC, file))
      if (!IMPORTS_CSS.test(text)) missing.push(relative(SRC, file))
    }

    // Zero means the regex no longer matches the code and guards nothing.
    expect(creators.length).toBeGreaterThan(0)
    expect(
      missing,
      "add `import 'leaflet/dist/leaflet.css'`; without it the panes are static and markers are clipped out of view",
    ).toEqual([])
  })
})
