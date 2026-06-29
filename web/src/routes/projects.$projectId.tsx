import { createFileRoute, Link } from '@tanstack/react-router'
import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  ArrowLeft,
  Check,
  ClipboardCopy,
  Download,
  FileLock2,
  Info,
  Lock,
  Plus,
  RefreshCw,
  Save,
  Users,
} from 'lucide-react'
import { api, ApiError } from '../lib/api'
import { envelopePreview, parseBundle } from '../lib/envfile'
import { Badge, Button, ErrorNote, Spinner } from '../components/ui'

export const Route = createFileRoute('/projects/$projectId')({
  component: ProjectDetail,
  // Drive the selected environment from the URL (?env=production) so it is
  // bookmarkable, shareable, and survives a refresh.
  validateSearch: (search: Record<string, unknown>): { env?: string } => ({
    env: typeof search.env === 'string' ? search.env : undefined,
  }),
})

const DEFAULT_ENVS = ['development', 'staging', 'production']

const SAMPLE_BUNDLE = `#/ envvar bundle — values are ciphertext, safe to commit
DOTENV_PUBLIC_KEY_ALG="hybrid-x25519-mlkem768"
DOTENV_RECIPIENTS="personal=pk_8f3a…2b,org-recovery=pk_4c1d…9e"
DATABASE_URL="enc:v1:Yk2p…q9:Zr7m…Lx:personal.Aa1…;org-recovery.Bb2…"
STRIPE_SECRET_KEY="enc:v1:Hn4t…w2:Qe8s…Vd:personal.Cc3…;org-recovery.Dd4…"
`

function ProjectDetail() {
  const { projectId } = Route.useParams()
  const { env: envParam } = Route.useSearch()
  const navigate = Route.useNavigate()

  const env = envParam ?? DEFAULT_ENVS[0]
  const envs = useMemo(
    () => Array.from(new Set([...DEFAULT_ENVS, env])),
    [env],
  )

  const setEnv = useCallback(
    (name: string) => {
      const clean = name.trim().toLowerCase()
      if (!clean) return
      void navigate({ search: { env: clean }, replace: true })
    },
    [navigate],
  )

  return (
    <div className="mx-auto max-w-5xl px-6 py-8">
      <Link
        to="/"
        className="inline-flex items-center gap-1.5 text-sm text-[var(--color-ink-dim)] transition hover:text-[var(--color-ink)]"
      >
        <ArrowLeft className="h-4 w-4" /> Projects
      </Link>

      <div className="mt-3 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">{projectId}</h1>
          <p className="mt-1 text-sm text-[var(--color-ink-dim)]">
            Each environment holds one encrypted secrets bundle.
          </p>
        </div>
      </div>

      <EnvTabs envs={envs} active={env} onSelect={setEnv} onAdd={setEnv} />

      <SecretsPanel key={`${projectId}:${env}`} projectId={projectId} env={env} />
    </div>
  )
}

function EnvTabs({
  envs,
  active,
  onSelect,
  onAdd,
}: {
  envs: string[]
  active: string
  onSelect: (e: string) => void
  onAdd: (e: string) => void
}) {
  const [adding, setAdding] = useState(false)
  const [name, setName] = useState('')

  return (
    <div className="mt-6 flex flex-wrap items-center gap-2 border-b border-[var(--color-line)] pb-px">
      {envs.map((e) => (
        <button
          key={e}
          onClick={() => onSelect(e)}
          className={`-mb-px rounded-t-md border-b-2 px-3 py-2 text-sm transition ${
            e === active
              ? 'border-[var(--color-accent)] text-[var(--color-ink)]'
              : 'border-transparent text-[var(--color-ink-dim)] hover:text-[var(--color-ink)]'
          }`}
        >
          {e}
        </button>
      ))}
      {adding ? (
        <form
          onSubmit={(ev) => {
            ev.preventDefault()
            onAdd(name)
            setName('')
            setAdding(false)
          }}
          className="flex items-center gap-1"
        >
          <input
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            onBlur={() => setAdding(false)}
            placeholder="env name"
            className="ring-accent w-28 rounded-md border border-[var(--color-line)] bg-[var(--color-canvas)] px-2 py-1 text-sm placeholder:text-[var(--color-ink-faint)]"
          />
        </form>
      ) : (
        <button
          onClick={() => setAdding(true)}
          className="ml-1 inline-flex items-center gap-1 rounded-md px-2 py-1 text-sm text-[var(--color-ink-faint)] transition hover:text-[var(--color-ink)]"
        >
          <Plus className="h-3.5 w-3.5" /> env
        </button>
      )}
    </div>
  )
}

interface BundleState {
  text: string
  etag: string | null
  updatedAt: number | null
  loading: boolean
  error: Error | null
  exists: boolean
  cached: boolean
}

function SecretsPanel({ projectId, env }: { projectId: string; env: string }) {
  const [state, setState] = useState<BundleState>({
    text: '',
    etag: null,
    updatedAt: null,
    loading: true,
    error: null,
    exists: false,
    cached: false,
  })
  const [draft, setDraft] = useState('')
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

  const load = useCallback(
    async (conditionalETag?: string | null) => {
      setState((s) => ({ ...s, loading: true, error: null, cached: false }))
      try {
        const b = await api.getSecrets(projectId, env, conditionalETag)
        if (b.notModified) {
          setState((s) => ({ ...s, loading: false, cached: true }))
          return
        }
        setState({
          text: b.text,
          etag: b.etag,
          updatedAt: null,
          loading: false,
          error: null,
          exists: b.etag !== null,
          cached: false,
        })
        setDraft(b.text)
      } catch (err) {
        setState((s) => ({
          ...s,
          loading: false,
          error: err instanceof Error ? err : new Error(String(err)),
        }))
      }
    },
    [projectId, env],
  )

  useEffect(() => {
    void load()
  }, [load])

  async function save() {
    setSaving(true)
    setSaveError(null)
    try {
      const res = await api.putSecrets(projectId, env, draft)
      setState((s) => ({
        ...s,
        text: draft,
        etag: res.etag,
        updatedAt: res.updated_at,
        exists: true,
        cached: false,
      }))
    } catch (err) {
      setSaveError(err instanceof ApiError ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  const parsed = useMemo(() => parseBundle(state.text), [state.text])
  const dirty = draft !== state.text

  if (state.loading) {
    return (
      <div className="mt-6">
        <Spinner label={`Loading ${env} bundle…`} />
      </div>
    )
  }
  if (state.error) {
    return (
      <div className="mt-6">
        <ErrorNote error={state.error} />
      </div>
    )
  }

  return (
    <div className="mt-6 grid gap-6 lg:grid-cols-[1fr_360px]">
      <Editor
        env={env}
        draft={draft}
        dirty={dirty}
        saving={saving}
        saveError={saveError}
        exists={state.exists}
        onChange={setDraft}
        onSave={save}
        onInsertSample={() => setDraft(SAMPLE_BUNDLE)}
        onReload={() => load(state.etag)}
        cached={state.cached}
      />
      <Sidebar parsed={parsed} etag={state.etag} exists={state.exists} />
    </div>
  )
}

function Editor({
  env,
  draft,
  dirty,
  saving,
  saveError,
  exists,
  cached,
  onChange,
  onSave,
  onInsertSample,
  onReload,
}: {
  env: string
  draft: string
  dirty: boolean
  saving: boolean
  saveError: string | null
  exists: boolean
  cached: boolean
  onChange: (v: string) => void
  onSave: () => void
  onInsertSample: () => void
  onReload: () => void
}) {
  const [copied, setCopied] = useState(false)

  async function copy() {
    await navigator.clipboard.writeText(draft)
    setCopied(true)
    setTimeout(() => setCopied(false), 1200)
  }

  function download() {
    const blob = new Blob([draft], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = env === 'development' ? '.env' : `.env.${env}`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="card flex flex-col">
      <div className="flex items-center justify-between border-b border-[var(--color-line)] px-4 py-2.5">
        <div className="flex items-center gap-2 text-sm">
          <FileLock2 className="h-4 w-4 text-[var(--color-ink-faint)]" />
          <span className="font-mono text-[var(--color-ink-dim)]">
            {env === 'development' ? '.env' : `.env.${env}`}
          </span>
          {dirty && <Badge tone="amber">unsaved</Badge>}
          {cached && <Badge tone="accent">304 cached</Badge>}
        </div>
        <div className="flex items-center gap-1">
          <IconBtn title="Reload" onClick={onReload}>
            <RefreshCw className="h-4 w-4" />
          </IconBtn>
          <IconBtn title="Copy" onClick={copy}>
            {copied ? (
              <Check className="h-4 w-4 text-[var(--color-accent)]" />
            ) : (
              <ClipboardCopy className="h-4 w-4" />
            )}
          </IconBtn>
          <IconBtn title="Download" onClick={download}>
            <Download className="h-4 w-4" />
          </IconBtn>
        </div>
      </div>

      <textarea
        value={draft}
        onChange={(e) => onChange(e.target.value)}
        spellCheck={false}
        placeholder={
          exists
            ? ''
            : `No bundle for "${env}" yet.\n\nProduce one with the CLI:\n  envvar set DATABASE_URL=… --env ${env}\nthen paste the encrypted .env here — or insert a sample below.`
        }
        className="ring-accent min-h-[340px] w-full resize-y bg-transparent p-4 font-mono text-[13px] leading-relaxed text-[var(--color-ink)] placeholder:text-[var(--color-ink-faint)] focus:outline-none"
      />

      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-[var(--color-line)] px-4 py-3">
        <div className="flex items-center gap-2">
          <Button onClick={onSave} busy={saving} disabled={!dirty}>
            <Save className="h-4 w-4" /> Save bundle
          </Button>
          {!draft && (
            <Button variant="ghost" onClick={onInsertSample}>
              Insert sample
            </Button>
          )}
        </div>
        {saveError && (
          <span className="text-xs text-[var(--color-danger)]">{saveError}</span>
        )}
      </div>
    </div>
  )
}

function Sidebar({
  parsed,
  etag,
  exists,
}: {
  parsed: ReturnType<typeof parseBundle>
  etag: string | null
  exists: boolean
}) {
  return (
    <aside className="flex flex-col gap-4">
      <ZeroKnowledgeNote />

      {exists && (
        <div className="card p-4">
          <h3 className="flex items-center gap-2 text-sm font-medium">
            <Lock className="h-4 w-4 text-[var(--color-accent)]" />
            Keys
            <span className="ml-auto font-mono text-xs text-[var(--color-ink-faint)]">
              {parsed.encryptedCount}/{parsed.entries.length} encrypted
            </span>
          </h3>
          <ul className="mt-3 flex flex-col gap-2">
            {parsed.entries.length === 0 && (
              <li className="text-xs text-[var(--color-ink-dim)]">
                No key/value pairs found in this bundle.
              </li>
            )}
            {parsed.entries.map((e) => (
              <li key={e.key} className="flex flex-col gap-1">
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate font-mono text-[13px]">{e.key}</span>
                  {e.encrypted ? (
                    <Badge tone="accent">
                      <Lock className="h-3 w-3" /> enc
                    </Badge>
                  ) : (
                    <Badge tone="amber">plaintext</Badge>
                  )}
                </div>
                <span className="truncate font-mono text-[11px] text-[var(--color-ink-faint)]">
                  {e.encrypted ? envelopePreview(e.value) : e.value}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {parsed.recipients.length > 0 && (
        <div className="card p-4">
          <h3 className="flex items-center gap-2 text-sm font-medium">
            <Users className="h-4 w-4 text-[var(--color-violet)]" /> Recipients
          </h3>
          <ul className="mt-3 flex flex-col gap-2">
            {parsed.recipients.map((r) => (
              <li key={r.label} className="flex items-center justify-between gap-2">
                <span className="font-mono text-[13px]">{r.label}</span>
                <span className="truncate font-mono text-[11px] text-[var(--color-ink-faint)]">
                  {r.publicKey}
                </span>
              </li>
            ))}
          </ul>
          {parsed.alg && (
            <p className="mt-3 border-t border-[var(--color-line)] pt-3 font-mono text-[11px] text-[var(--color-ink-faint)]">
              alg {parsed.alg}
            </p>
          )}
        </div>
      )}

      {etag && (
        <div className="card p-4">
          <h3 className="text-sm font-medium">Version</h3>
          <p className="mt-2 break-all font-mono text-[11px] text-[var(--color-ink-faint)]">
            ETag {etag}
          </p>
          <p className="mt-1 text-xs text-[var(--color-ink-dim)]">
            SDKs fetch with this validator — an unchanged bundle returns 304.
          </p>
        </div>
      )}
    </aside>
  )
}

function ZeroKnowledgeNote() {
  return (
    <div className="card border-[var(--color-accent)]/20 bg-[var(--color-accent)]/[0.04] p-4">
      <h3 className="flex items-center gap-2 text-sm font-medium text-[var(--color-accent)]">
        <Info className="h-4 w-4" /> Zero-knowledge
      </h3>
      <p className="mt-2 text-xs leading-relaxed text-[var(--color-ink-dim)]">
        This page transports ciphertext only. Key names stay readable so you can
        see <em>what</em> changed; values are encrypted client-side by the CLI
        (in-browser WASM encryption is on the roadmap). The server never holds a
        decryption key.
      </p>
    </div>
  )
}

function IconBtn({
  title,
  onClick,
  children,
}: {
  title: string
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      title={title}
      onClick={onClick}
      className="ring-accent rounded-md p-1.5 text-[var(--color-ink-dim)] transition hover:bg-[var(--color-surface-2)] hover:text-[var(--color-ink)]"
    >
      {children}
    </button>
  )
}
