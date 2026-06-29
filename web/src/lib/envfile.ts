// Read-only, key-preserving parser for envvar bundles.
//
// A bundle is .env-compatible text: KEY=value lines plus comments. Values may be
// `enc:v1:` envelopes (ciphertext). The browser has no private key, so it can
// only ever READ structure — which keys exist, whether each is encrypted, and
// the recipient/algorithm headers — without learning any plaintext. That is
// exactly the zero-knowledge "show which keys changed without revealing values"
// property from the plan. Mirrors internal/envfile + internal/crypto on the Go
// side (envelope prefix `enc:v1:`, headers DOTENV_PUBLIC_KEY_ALG / DOTENV_RECIPIENTS).

export const ENVELOPE_PREFIX = 'enc:v1:'
export const HEADER_ALG = 'DOTENV_PUBLIC_KEY_ALG'
export const HEADER_RECIPIENTS = 'DOTENV_RECIPIENTS'

export interface ParsedEntry {
  key: string
  /** Raw stored value (an `enc:v1:` envelope, or plaintext if not encrypted). */
  value: string
  encrypted: boolean
}

export interface Recipient {
  label: string
  publicKey: string
}

export interface ParsedBundle {
  entries: ParsedEntry[]
  alg: string | null
  recipients: Recipient[]
  /** Count of KEY=value pairs that carry an encrypted value. */
  encryptedCount: number
}

function unquote(s: string): string {
  if (s.length >= 2) {
    const a = s[0]
    const b = s[s.length - 1]
    if ((a === '"' && b === '"') || (a === "'" && b === "'")) {
      return s.slice(1, -1)
    }
  }
  return s
}

export function isEncrypted(value: string): boolean {
  return value.startsWith(ENVELOPE_PREFIX)
}

export function parseBundle(text: string): ParsedBundle {
  const entries: ParsedEntry[] = []
  let alg: string | null = null
  let recipients: Recipient[] = []

  for (const rawLine of text.split('\n')) {
    const trimmed = rawLine.trim()
    if (trimmed === '' || trimmed.startsWith('#')) continue
    const eq = rawLine.indexOf('=')
    if (eq < 0) continue
    const key = rawLine.slice(0, eq).trim()
    const value = unquote(rawLine.slice(eq + 1).trim())

    if (key === HEADER_ALG) {
      alg = value
      continue
    }
    if (key === HEADER_RECIPIENTS) {
      recipients = parseRecipients(value)
      continue
    }
    entries.push({ key, value, encrypted: isEncrypted(value) })
  }

  return {
    entries,
    alg,
    recipients,
    encryptedCount: entries.filter((e) => e.encrypted).length,
  }
}

function parseRecipients(header: string): Recipient[] {
  const out: Recipient[] = []
  for (const part of header.split(',')) {
    const p = part.trim()
    if (!p) continue
    const eq = p.indexOf('=')
    if (eq < 0) continue
    out.push({ label: p.slice(0, eq).trim(), publicKey: p.slice(eq + 1).trim() })
  }
  return out
}

/** A short, safe preview of a ciphertext value for display (never plaintext). */
export function envelopePreview(value: string, head = 18, tail = 6): string {
  if (value.length <= head + tail + 1) return value
  return `${value.slice(0, head)}…${value.slice(-tail)}`
}
