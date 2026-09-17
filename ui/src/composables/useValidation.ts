import { JSONPath } from 'jsonpath-plus'

/**
 * Validation Result
 * Grug say: Simple object. Valid or not valid. If not valid, tell why.
 */
export interface ValidationResult {
  valid: boolean
  error?: string
}

/**
 * Validation Composable
 * 
 * Grug say: Check if user input is good. Don't let bad data in system.
 * Better to catch problems early than debug later.
 * 
 * Validates:
 * - NATS subject patterns
 * - JSONPath expressions
 * - Buffer sizes
 * - KV bucket/key names
 * - JSON payloads
 */
export function useValidation() {
  
  // ============================================================================
  // NATS SUBJECT VALIDATION
  // ============================================================================
  
  /**
   * Validate NATS subject pattern
   *
   * Rules, and they are the ones NATS actually has:
   * - Cannot be empty
   * - Cannot contain whitespace or control characters
   * - Cannot have an empty token (consecutive dots, or a leading/trailing dot)
   * - `>` must be the last token; `*` must be alone in its token
   *
   * THERE IS DELIBERATELY NO CHARACTER ALLOWLIST. There used to be
   * (`/^[a-zA-Z0-9._\-*>{}]+$/`) and it rejected every NATS system subject,
   * because it had no `$`:
   *
   *   $SYS.REQ.ACCOUNT.PING.CONNZ   the site-connectivity recipe
   *   $JS.API.STREAM.LIST           every JetStream API call
   *   $KV.<bucket>.<key>            every KV subject
   *
   * All three are ordinary subjects a console session is allow-listed to
   * publish to (see internal/demoseed/contract.go), so the form was refusing
   * input the server would have accepted -- and the message named a list of
   * permitted characters, which reads like a syntax rule rather than a bug.
   *
   * The wider lesson is that the allowlist was a DENY-LIST wearing the other
   * costume: it could only ever be a guess at the character set NATS uses, and
   * a guess that is wrong rejects legal input silently and forever. The client
   * (@nats-io/nats-core, `ConnectionImpl._check`) enforces exactly two things:
   * non-empty, and no whitespace. nats-server adds no empty tokens and `>`
   * last. Those are checked here and nothing else is invented.
   *
   * `*` alone-in-token is stricter than nats-server, which treats `foo*` as a
   * literal token rather than an error. Kept deliberately: a literal asterisk
   * in a subject is vanishingly rare and a mistyped wildcard is not.
   */
  function validateSubject(subject: string): ValidationResult {
    // Empty check
    if (!subject || subject.trim() === '') {
      return { valid: false, error: 'Subject cannot be empty' }
    }

    // Callers store the trimmed value (useWidgetForm), so validate that -- the
    // whitespace check below is then about whitespace INSIDE the subject,
    // which is the error that actually reaches the client.
    subject = subject.trim()

    // The client's own rule, and the one the character allowlist used to
    // enforce as a side effect. Losing it while widening would have traded one
    // bug for a worse one.
    if (/\s/.test(subject)) {
      return { valid: false, error: 'Subject cannot contain whitespace' }
    }

    // Not rejected by the client, but never intended by a human, and invisible
    // in a form field if pasted.
    // eslint-disable-next-line no-control-regex
    if (/[\x00-\x1f\x7f]/.test(subject)) {
      return { valid: false, error: 'Subject cannot contain control characters' }
    }

    // Check for consecutive dots
    if (subject.includes('..')) {
      return { valid: false, error: 'Subject cannot contain consecutive dots (..)' }
    }

    // Cannot start or end with dot
    if (subject.startsWith('.') || subject.endsWith('.')) {
      return { valid: false, error: 'Subject cannot start or end with a dot' }
    }

    // > wildcard must be at end and must be last token
    // Note: We skip this check if variables are present, as {{var}} might resolve to anything
    if (!subject.includes('{{')) {
      const parts = subject.split('.')
      const gtIndex = parts.findIndex(p => p.includes('>'))
      if (gtIndex !== -1) {
        // Found > wildcard
        if (gtIndex !== parts.length - 1) {
          return { valid: false, error: '> wildcard must be the last token in subject' }
        }
        if (parts[gtIndex] !== '>') {
          return { valid: false, error: '> wildcard must be alone in its token (e.g., "foo.>" not "foo.bar>"' }
        }
      }
      
      // * wildcard must be alone in its token
      for (const part of parts) {
        if (part.includes('*') && part !== '*') {
          return { valid: false, error: '* wildcard must be alone in its token (e.g., "foo.*" not "foo*")' }
        }
      }
    }
    
    return { valid: true }
  }
  
  // ============================================================================
  // JSONPATH VALIDATION
  // ============================================================================
  
  /**
   * Validate JSONPath expression
   * 
   * Tries to execute the path on a test object to check syntax
   */
  function validateJsonPath(path: string): ValidationResult {
    // Empty is valid (means no extraction)
    if (!path || path.trim() === '') {
      return { valid: true }
    }
    
    path = path.trim()
    
    // $ alone is valid (means whole object)
    if (path === '$') {
      return { valid: true }
    }

    // Grug fix: If path has variables {{...}}, we cannot validate it strictly
    // because the parser will likely choke on the braces. Trust the user.
    if (path.includes('{{')) {
      return { valid: true }
    }
    
    // Try to execute on test object
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
  
  // ============================================================================
  // BUFFER SIZE VALIDATION
  // ============================================================================
  
  /**
   * Validate buffer size (message count)
   * 
   * Rules:
   * - Must be a number
   * - Must be between 10 and 10,000
   */
  function validateBufferSize(size: number): ValidationResult {
    // Check if it's a number
    if (typeof size !== 'number' || isNaN(size)) {
      return { valid: false, error: 'Buffer size must be a number' }
    }
    
    // Check range
    if (size < 10) {
      return { valid: false, error: 'Buffer size must be at least 10 messages' }
    }
    
    if (size > 10000) {
      return { valid: false, error: 'Buffer size cannot exceed 10,000 messages (memory concerns)' }
    }
    
    // Check if integer
    if (!Number.isInteger(size)) {
      return { valid: false, error: 'Buffer size must be a whole number' }
    }
    
    return { valid: true }
  }
  
  // ============================================================================
  // KV VALIDATION
  // ============================================================================
  
  /**
   * Validate KV bucket name
   * 
   * Rules (from NATS spec):
   * - Cannot be empty
   * - Only lowercase letters, numbers, dashes, underscores
   * - Cannot start with dash
   * - Max 255 characters
   */
  function validateKvBucket(bucket: string): ValidationResult {
    if (!bucket || bucket.trim() === '') {
      return { valid: false, error: 'Bucket name cannot be empty' }
    }
    
    bucket = bucket.trim()
    
    // Length check
    if (bucket.length > 255) {
      return { valid: false, error: 'Bucket name cannot exceed 255 characters' }
    }
    
    // Character check
    // Grug fix: Allow { } for variables. 
    // Also allow A-Z because variable names inside {{...}} might be uppercase (e.g. {{DeviceID}})
    const validPattern = /^[a-zA-Z0-9_\-{}]+$/
    if (!validPattern.test(bucket)) {
      return { 
        valid: false, 
        error: 'Bucket name contains invalid characters. Allowed: a-z, 0-9, -, _, { }' 
      }
    }
    
    // Cannot start with dash (unless it's a variable start, but that's unlikely to be just '-')
    if (bucket.startsWith('-')) {
      return { valid: false, error: 'Bucket name cannot start with a dash' }
    }
    
    return { valid: true }
  }
  
  /**
   * Validate KV key
   * 
   * Rules:
   * - Cannot be empty
   * - Max 255 characters
   * - Cannot contain certain special chars (NATS restriction)
   */
  function validateKvKey(key: string): ValidationResult {
    if (!key || key.trim() === '') {
      return { valid: false, error: 'Key cannot be empty' }
    }

    key = key.trim()

    // Length check
    if (key.length > 255) {
      return { valid: false, error: 'Key cannot exceed 255 characters' }
    }

    // Variables cannot be checked: {{site}} resolves at render time and the
    // resolved value is what the client sees. Same trade as validateSubject.
    if (key.includes('{{')) {
      return { valid: true }
    }

    // THE OPPOSITE MISTAKE TO validateSubject, found looking for more of them.
    // This listed five forbidden characters, which let through everything else
    // -- `$`, `:`, `#`, `@`, `+` and the rest -- all of which @nats-io/kv
    // refuses at runtime. The form said the key was fine and the write failed
    // later, with an error further from the field that caused it.
    //
    // A KV key IS a subject token, so it is narrower than a subject: the
    // client's rule is /^[-/=.\w]+$/ plus no leading or trailing dot
    // (@nats-io/kv, `validateKey`). Mirrored here rather than approximated.
    //
    // Wildcards stay rejected, and that is right for this field even though
    // the client permits them in a SEARCH key: every caller here
    // (useWidgetForm) is a concrete read or write, and useNatsKv builds its own
    // watch filter internally rather than taking one from the form.
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
  
  // ============================================================================
  // JSON PAYLOAD VALIDATION
  // ============================================================================
  
  /**
   * Validate JSON string
   * 
   * Tries to parse it to check if valid JSON
   * Empty is valid (will be treated as empty object)
   */
  function validateJson(jsonString: string): ValidationResult {
    // Empty is valid
    if (!jsonString || jsonString.trim() === '') {
      return { valid: true }
    }

    // Grug fix: If it contains variables, it might not be valid JSON yet.
    // e.g. {"id": "{{device_id}}"} is valid JSON, but {"val": {{value}}} is NOT valid JSON until resolved.
    // We try to be lenient here.
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
  
  // ============================================================================
  // URL VALIDATION
  // ============================================================================
  
  /**
   * Validate WebSocket URL
   * 
   * Rules:
   * - Must start with ws:// or wss://
   * - Must have valid hostname/IP
   * - Port is optional
   */
  function validateWebSocketUrl(url: string): ValidationResult {
    if (!url || url.trim() === '') {
      return { valid: false, error: 'URL cannot be empty' }
    }
    
    url = url.trim()
    
    // Must start with ws:// or wss://
    if (!url.startsWith('ws://') && !url.startsWith('wss://')) {
      return { 
        valid: false, 
        error: 'URL must start with ws:// (WebSocket) or wss:// (Secure WebSocket)' 
      }
    }
    
    // Try to parse as URL
    try {
      const parsed = new URL(url)
      
      // Check protocol
      if (parsed.protocol !== 'ws:' && parsed.protocol !== 'wss:') {
        return { valid: false, error: 'Invalid protocol. Use ws:// or wss://' }
      }
      
      // Must have hostname
      if (!parsed.hostname) {
        return { valid: false, error: 'URL must include a hostname or IP address' }
      }
      
      return { valid: true }
    } catch (err) {
      return { valid: false, error: 'Invalid URL format' }
    }
  }
  
  // ============================================================================
  // WIDGET TITLE VALIDATION
  // ============================================================================
  
  /**
   * Validate widget title
   * 
   * Rules:
   * - Cannot be empty
   * - Max 100 characters
   */
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
  
  // ============================================================================
  // RETURN PUBLIC API
  // ============================================================================
  
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
