// The one place a template token is substituted. Dashboard variables, Thing
// subject templates and the Scanner's tokens all go through `fillTemplate`;
// each caller only decides which names it supplies.
//
// A token is `{name}`, the same spelling rule-router's rules use. That is the
// one the console writes: placeholders, help text and defaults all show it.
//
// `{{ name }}` is still read, and always will be. Dashboards saved before this
// used it for variables, and some of those live in a browser's localStorage or
// an exported file, where no migration can reach them. Never write it.
//
// `{{name}}` is tried first and consumed whole, so a single-brace pass never
// rewrites the inside of a double-brace token it does not know.

const TOKEN = /\{\{\s*([\w.:-]+)\s*\}\}|\{([\w.:-]+)\}/g

export type TemplateValues = Record<string, string | undefined>

/**
 * True if the text holds a token. A form cannot judge a subject, key or JSON
 * payload that still has one, since the value is only known at render time.
 * JSON itself never matches: a token is a bare name between braces.
 */
export function hasToken(template: string): boolean {
  TOKEN.lastIndex = 0
  return TOKEN.test(template)
}

/**
 * Replace every token whose name has a value. A name with no value keeps its
 * token, so the reader sees what is missing, unless `missing` says what to
 * write instead.
 */
export function fillTemplate(template: string, values: TemplateValues, missing?: string): string {
  return template.replace(TOKEN, (token, doubleName, singleName) => {
    const name = doubleName ?? singleName
    // Own keys only: `{constructor}` must not find Object.prototype.
    const value = Object.prototype.hasOwnProperty.call(values, name) ? values[name] : undefined
    return value ?? missing ?? token
  })
}
