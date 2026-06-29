import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import {
  ArrowRight,
  Boxes,
  FolderGit2,
  GitBranch,
  KeyRound,
  Plus,
  ShieldCheck,
} from 'lucide-react'
import { api, ApiError, type Project } from '../lib/api'
import { useAsync } from '../lib/useAsync'
import { Button, EmptyState, ErrorNote, Spinner } from '../components/ui'

export const Route = createFileRoute('/')({ component: Home })

function Home() {
  const projects = useAsync<Project[]>(() => api.listProjects(), [])
  const health = useAsync(() => api.health(), [])

  return (
    <div className="mx-auto max-w-5xl px-6 py-10">
      <Hero version={health.data?.version} healthError={health.error} />

      <section className="mt-10">
        <div className="flex items-center justify-between">
          <h2 className="flex items-center gap-2 text-sm font-semibold tracking-wide text-[var(--color-ink-dim)] uppercase">
            <FolderGit2 className="h-4 w-4" /> Projects
          </h2>
        </div>

        <div className="mt-4">
          {projects.loading && <Spinner label="Loading projects…" />}
          {projects.error && <ErrorNote error={projects.error} />}
          {projects.data && (
            <ProjectGrid projects={projects.data} onCreated={projects.reload} />
          )}
        </div>
      </section>
    </div>
  )
}

function Hero({
  version,
  healthError,
}: {
  version?: string
  healthError?: Error
}) {
  return (
    <section>
      <div className="flex items-center gap-2">
        <span
          className={`inline-block h-2 w-2 rounded-full ${
            healthError ? 'bg-[var(--color-danger)]' : 'bg-[var(--color-accent)]'
          }`}
        />
        <span className="font-mono text-xs text-[var(--color-ink-dim)]">
          {healthError
            ? 'api offline'
            : version
              ? `api ok · ${version}`
              : 'connecting…'}
        </span>
      </div>
      <h1 className="mt-4 text-3xl font-semibold tracking-tight sm:text-4xl">
        Secrets you can commit.
      </h1>
      <p className="mt-3 max-w-2xl text-[var(--color-ink-dim)]">
        Git-native, post-quantum, agent-ready environment variables. Values are
        wrapped with a hybrid X25519&nbsp;+&nbsp;ML-KEM-768 KEM and stored as
        ciphertext — safe in your repo and safe on this server, which never sees
        plaintext.
      </p>
      <div className="mt-6 grid gap-3 sm:grid-cols-3">
        <Pill icon={<GitBranch className="h-4 w-4" />} title="Git-native">
          Commit encrypted <code className="font-mono">.env</code> safely.
        </Pill>
        <Pill icon={<KeyRound className="h-4 w-4" />} title="Post-quantum">
          Resists harvest-now-decrypt-later.
        </Pill>
        <Pill icon={<ShieldCheck className="h-4 w-4" />} title="Zero-knowledge">
          Server stores only ciphertext.
        </Pill>
      </div>
    </section>
  )
}

function Pill({
  icon,
  title,
  children,
}: {
  icon: React.ReactNode
  title: string
  children: React.ReactNode
}) {
  return (
    <div className="card px-4 py-3">
      <div className="flex items-center gap-2 text-[var(--color-accent)]">
        {icon}
        <span className="text-sm font-medium text-[var(--color-ink)]">{title}</span>
      </div>
      <p className="mt-1 text-xs text-[var(--color-ink-dim)]">{children}</p>
    </div>
  )
}

function ProjectGrid({
  projects,
  onCreated,
}: {
  projects: Project[]
  onCreated: () => void
}) {
  const [creating, setCreating] = useState(false)

  if (projects.length === 0 && !creating) {
    return (
      <EmptyState icon={<Boxes className="h-8 w-8" />} title="No projects yet">
        <p>
          A project groups environments (dev, staging, production), each holding
          one encrypted secrets bundle.
        </p>
        <div className="mt-4">
          <Button onClick={() => setCreating(true)}>
            <Plus className="h-4 w-4" /> New project
          </Button>
        </div>
      </EmptyState>
    )
  }

  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {projects.map((p) => (
        <ProjectCard key={p.id} project={p} />
      ))}
      {creating ? (
        <CreateCard
          onCancel={() => setCreating(false)}
          onCreated={() => {
            setCreating(false)
            onCreated()
          }}
        />
      ) : (
        <button
          onClick={() => setCreating(true)}
          className="ring-accent flex min-h-[104px] items-center justify-center gap-2 rounded-xl border border-dashed border-[var(--color-line)] text-sm text-[var(--color-ink-dim)] transition hover:border-[var(--color-accent)]/40 hover:text-[var(--color-ink)]"
        >
          <Plus className="h-4 w-4" /> New project
        </button>
      )}
    </div>
  )
}

function ProjectCard({ project }: { project: Project }) {
  return (
    <Link
      to="/projects/$projectId"
      params={{ projectId: project.id }}
      className="card group flex flex-col justify-between p-4 transition hover:border-[var(--color-accent)]/40"
    >
      <div>
        <div className="flex items-center gap-2">
          <FolderGit2 className="h-4 w-4 text-[var(--color-ink-faint)]" />
          <span className="font-medium">{project.name}</span>
        </div>
        <p className="mt-1 font-mono text-xs text-[var(--color-ink-faint)]">
          {project.id}
        </p>
      </div>
      <div className="mt-4 flex items-center justify-between text-xs text-[var(--color-ink-dim)]">
        <span>{formatDate(project.created_at)}</span>
        <ArrowRight className="h-4 w-4 -translate-x-1 opacity-0 transition group-hover:translate-x-0 group-hover:opacity-100" />
      </div>
    </Link>
  )
}

function CreateCard({
  onCreated,
  onCancel,
}: {
  onCreated: () => void
  onCancel: () => void
}) {
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    setBusy(true)
    setError(null)
    try {
      await api.createProject(name.trim())
      onCreated()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err))
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} className="card flex flex-col gap-3 p-4">
      <input
        autoFocus
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="Project name"
        className="ring-accent rounded-lg border border-[var(--color-line)] bg-[var(--color-canvas)] px-3 py-2 text-sm placeholder:text-[var(--color-ink-faint)]"
      />
      {error && <p className="text-xs text-[var(--color-danger)]">{error}</p>}
      <div className="flex gap-2">
        <Button type="submit" busy={busy} disabled={!name.trim()}>
          Create
        </Button>
        <Button type="button" variant="ghost" onClick={onCancel} disabled={busy}>
          Cancel
        </Button>
      </div>
    </form>
  )
}

function formatDate(unixSeconds: number): string {
  if (!unixSeconds) return ''
  return new Date(unixSeconds * 1000).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}
