import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

// Leaflet does ALL of its positioning from its own stylesheet:
//
//   .leaflet-pane, .leaflet-marker-icon, ... { position: absolute; left: 0; top: 0 }
//
// Without it every pane computes to `position: static` and the panes stack in
// normal flow instead of overlaying. The map still draws, the image overlay
// still appears, nothing throws and no console line is printed -- but the
// marker pane lands one image-height BELOW the image, where the widget's
// `overflow: hidden` clips it, and the markers are simply gone.
//
// It stayed hidden because a stylesheet is global once ANY module imports it.
// useLeafletMap.ts always did, so useFloorPlan.ts worked on every screen that
// happened to mount a geographic map alongside it -- which the location detail
// view does. The first screen to use a floor plan on its own (the dashboard
// widget) was the first to see it, and by then "it works on the location page"
// made the composable look innocent.
//
// So: a file that creates a Leaflet map imports Leaflet's CSS itself, rather
// than relying on a neighbour to have done it.

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

    // If this drops to zero the regex has stopped matching the code and the
    // test would pass while guarding nothing.
    expect(creators.length).toBeGreaterThan(0)
    expect(
      missing,
      "add `import 'leaflet/dist/leaflet.css'`; without it the panes are static and markers are clipped out of view",
    ).toEqual([])
  })
})
