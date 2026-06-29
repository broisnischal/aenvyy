// Client for the envvar control-plane API (/v1).
//
// The server is zero-knowledge: secret bundles are opaque `enc:v1:` envelopes
// (env-file text where KEYS are readable but VALUES are ciphertext). This client
// transports those bundles; it never decrypts. Client-side decryption is the
// job of the WASM crypto core (planned) — see lib/envfile.ts for the read-only
// parsing the browser can do without any key.

export interface Health {
  status: string
  version: string
}

export interface Project {
  id: string
  name: string
  created_at: number
}

export interface Bundle {
  /** Raw env-file ciphertext bundle (may be empty). */
  text: string
  /** Strong validator from the server, used for conditional GET. */
  etag: string | null
  /** True when the server answered 304 Not Modified to a conditional GET. */
  notModified: boolean
}

export interface PutResult {
  etag: string
  updated_at: number
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
    this.name = 'ApiError'
  }
}

async function json<T>(res: Response, path: string): Promise<T> {
  if (!res.ok) {
    let msg = `${res.status} ${res.statusText}`
    try {
      const body = (await res.json()) as { error?: string }
      if (body?.error) msg = body.error
    } catch {
      /* non-JSON error body */
    }
    throw new ApiError(res.status, `${path}: ${msg}`)
  }
  return res.json() as Promise<T>
}

export const api = {
  health: () =>
    fetch('/healthz', { headers: { Accept: 'application/json' } }).then((r) =>
      json<Health>(r, '/healthz'),
    ),

  listProjects: () =>
    fetch('/v1/projects', { headers: { Accept: 'application/json' } })
      .then((r) => json<{ projects: Project[] }>(r, '/v1/projects'))
      .then((b) => b.projects ?? []),

  createProject: (name: string) =>
    fetch('/v1/projects', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    }).then((r) => json<Project>(r, 'POST /v1/projects')),

  /**
   * Fetch a ciphertext bundle. Pass the previous ETag to get a cheap 304 when
   * nothing changed. Returns notModified=true / text='' in that case, and an
   * empty bundle (etag=null) when none exists yet (404).
   */
  async getSecrets(
    projectId: string,
    env: string,
    knownETag?: string | null,
  ): Promise<Bundle> {
    const path = `/v1/projects/${encodeURIComponent(projectId)}/environments/${encodeURIComponent(env)}/secrets`
    const headers: Record<string, string> = {}
    if (knownETag) headers['If-None-Match'] = knownETag
    const res = await fetch(path, { headers })
    if (res.status === 304) {
      return { text: '', etag: knownETag ?? null, notModified: true }
    }
    if (res.status === 404) {
      return { text: '', etag: null, notModified: false }
    }
    if (!res.ok) {
      throw new ApiError(res.status, `${path}: ${res.status} ${res.statusText}`)
    }
    return {
      text: await res.text(),
      etag: res.headers.get('ETag'),
      notModified: false,
    }
  },

  async putSecrets(
    projectId: string,
    env: string,
    text: string,
  ): Promise<PutResult> {
    const path = `/v1/projects/${encodeURIComponent(projectId)}/environments/${encodeURIComponent(env)}/secrets`
    const res = await fetch(path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/octet-stream' },
      body: text,
    })
    return json<PutResult>(res, `PUT ${path}`)
  },
}
