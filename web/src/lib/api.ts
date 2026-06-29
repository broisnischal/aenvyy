// Minimal client for the envvar control-plane API (/v1).
//
// The server is zero-knowledge: it returns only ciphertext envelopes. Decryption
// happens in the browser (planned: WASM build of the Go crypto core), so this
// client deals in envelopes, never plaintext.

export interface Health {
  status: string
  version: string
}

export interface Project {
  id: string
  name: string
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: 'application/json' } })
  if (!res.ok) {
    throw new Error(`${path}: ${res.status} ${res.statusText}`)
  }
  return res.json() as Promise<T>
}

export const api = {
  health: () => get<Health>('/healthz'),
  listProjects: () => get<Project[]>('/v1/projects'),
}
