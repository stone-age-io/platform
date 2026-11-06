import { JSONPath } from 'jsonpath-plus'

export interface ValidationResult {
  valid: boolean
  error?: string
}

/** Form-input validators for widget and connection settings. */
export function useValidation() {

  /**
   * NATS subject pattern: non-empty, no whitespace or control characters, no
   * empty token, `>` only as the last token, `*` alone in its token.
   *
   * Deliberately no character allowlist: any list is a guess at NATS's
   * character set, and the old one rejected every `$SYS`, `$JS` and `$KV`
   * subject. These are the client's and nats-server's own rules, nothing more.
   *
   * `*` alone-in-token is stricter than nats-server (which treats `foo*` as a
   * literal), on purpose: a mistyped wildcard is far likelier than a literal.
   */
  function validateSubject(subject: string): ValidationResult {
    if (!subject || subject.trim() === '') {
      return { valid: false, error: 'Subject cannot be empty' }
    }

    // Callers store the trimmed value, so only inner whitespace is an error.
    subject = subject.trim()

    if (/\s/.test(subject)) {
      return { valid: false, error: 'Subject cannot contain whitespace' }
    }

    // The client allows these, but they are never intended and invisible in a field.
    if (/[\x00-\x1f\x7f]/.test(subject)) {
      return { valid: false, error: 'Subject cannot contain control characters' }
    }

    if (subject.includes('..')) {
      return { valid: false, error: 'Subject cannot contain consecutive dots (..)' }
    }

    if (subject.startsWith('.') || subject.endsWith('.')) {
      return { valid: false, error: 'Subject cannot start or end with a dot' }
    }

    // Wildcard placement. Skipped for a {{variable}}, which might resolve to anything.
    if (!subject.includes('{{')) {
      const parts = subject.split('.')
      const gtIndex = parts.findIndex(p => p.includes('>'))
      if (gtIndex !== -1) {
        if (gtIndex !== parts.length - 1) {
          return { valid: false, error: '> wildcard must be the last token in subject' }
        }
        if (parts[gtIndex] !== '>') {
          return { valid: false, error: '> wildcard must be alone in its token (e.g., "foo.>" not "foo.bar>"' }
        }
      }

      for (const part of parts) {
        if (part.includes('*') && part !== '*') {
          return { valid: false, error: '* wildcard must be alone in its token (e.g., "foo.*" not "foo*")' }
        }
      }
    }
    
    return { valid: true }
  }

  /**
   * JSONPath syntax, checked by running it on a sample object. Empty means no
   * extraction and `$` the whole payload; both are valid.
   */
  function validateJsonPath(path: string): ValidationResult {
    if (!path || path.trim() === '') {
      return { valid: true }
    }

    path = path.trim()

    if (path === '$') {
      return { valid: true }
    }

    // The parser cannot read {{variables}}, so these are not checked.
    if (path.includes('{{')) {
      return { valid: true }
    }

    try {
      const testObj = {
        value: 42,
        nested: { field: 'test' },
        array: [1, 2, 3],
        sensors: [
          { temp: 20, humidity: 50 },
          { temp: 21, humidity: 51 }
        ]
      }
      
      JSONPath({ path, json: testObj, wrap: false })
      return { valid: true }
    } catch (err: any) {
      return { 
        valid: false, 
        error: `Invalid JSONPath: ${err.message || 'Syntax error'}` 
      }
    }
  }

  /** Buffer size in messages: a whole number from 10 to 10,000. */
  function validateBufferSize(size: number): ValidationResult {
    if (typeof size !== 'number' || isNaN(size)) {
      return { valid: false, error: 'Buffer size must be a number' }
    }

    if (size < 10) {
      return { valid: false, error: 'Buffer size must be at least 10 messages' }
    }
    
    if (size > 10000) {
      return { valid: false, error: 'Buffer size cannot exceed 10,000 messages (memory concerns)' }
    }

    if (!Number.isInteger(size)) {
      return { valid: false, error: 'Buffer size must be a whole number' }
    }
    
    return { valid: true }
  }

  function validateKvBucket(bucket: string): ValidationResult {
    if (!bucket || bucket.trim() === '') {
      return { valid: false, error: 'Bucket name cannot be empty' }
    }

    bucket = bucket.trim()

    if (bucket.length > 255) {
      return { valid: false, error: 'Bucket name cannot exceed 255 characters' }
    }

    // { } for {{variables}}, and A-Z because variable names may be uppercase.
    const validPattern = /^[a-zA-Z0-9_\-{}]+$/
    if (!validPattern.test(bucket)) {
      return { 
        valid: false, 
        error: 'Bucket name contains invalid characters. Allowed: a-z, 0-9, -, _, { }' 
      }
    }

    if (bucket.startsWith('-')) {
      return { valid: false, error: 'Bucket name cannot start with a dash' }
    }
    
    return { valid: true }
  }

  function validateKvKey(key: string): ValidationResult {
    if (!key || key.trim() === '') {
      return { valid: false, error: 'Key cannot be empty' }
    }

    key = key.trim()

    if (key.length > 255) {
      return { valid: false, error: 'Key cannot exceed 255 characters' }
    }

    // {{variables}} resolve at render time, so they cannot be checked here.
    if (key.includes('{{')) {
      return { valid: true }
    }

    // Mirrors @nats-io/kv `validateKey` exactly: /^[-/=.\w]+$/ and no leading or
    // trailing dot. Wildcards are rejected: every caller is a concrete read or
    // write, never a watch filter.
    if (!/^[-/=.\w]+$/.test(key)) {
      return {
        valid: false,
        error: 'Key contains invalid characters. Allowed: letters, numbers, . - _ / =',
      }
    }

    if (key.startsWith('.') || key.endsWith('.')) {
      return { valid: false, error: 'Key cannot start or end with a dot' }
    }

    return { valid: true }
  }

  /** JSON payload. Empty is valid (treated as an empty object). */
  function validateJson(jsonString: string): ValidationResult {
    if (!jsonString || jsonString.trim() === '') {
      return { valid: true }
    }

    // `{"val": {{value}}}` is only JSON once resolved, so skip any {{variable}}.
    if (jsonString.includes('{{')) {
      return { valid: true }
    }
    
    try {
      JSON.parse(jsonString)
      return { valid: true }
    } catch (err: any) {
      return { 
        valid: false, 
        error: `Invalid JSON: ${err.message || 'Syntax error'}` 
      }
    }
  }

  function validateWebSocketUrl(url: string): ValidationResult {
    if (!url || url.trim() === '') {
      return { valid: false, error: 'URL cannot be empty' }
    }

    url = url.trim()

    if (!url.startsWith('ws://') && !url.startsWith('wss://')) {
      return { 
        valid: false, 
        error: 'URL must start with ws:// (WebSocket) or wss:// (Secure WebSocket)' 
      }
    }

    try {
      const parsed = new URL(url)

      if (parsed.protocol !== 'ws:' && parsed.protocol !== 'wss:') {
        return { valid: false, error: 'Invalid protocol. Use ws:// or wss://' }
      }

      if (!parsed.hostname) {
        return { valid: false, error: 'URL must include a hostname or IP address' }
      }
      
      return { valid: true }
    } catch (err) {
      return { valid: false, error: 'Invalid URL format' }
    }
  }

  function validateWidgetTitle(title: string): ValidationResult {
    if (!title || title.trim() === '') {
      return { valid: false, error: 'Title cannot be empty' }
    }
    
    title = title.trim()
    
    if (title.length > 100) {
      return { valid: false, error: 'Title cannot exceed 100 characters' }
    }
    
    return { valid: true }
  }

  return {
    validateSubject,
    validateJsonPath,
    validateBufferSize,
    validateKvBucket,
    validateKvKey,
    validateJson,
    validateWebSocketUrl,
    validateWidgetTitle,
  }
}
