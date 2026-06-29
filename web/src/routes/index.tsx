import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { KeyRound, ShieldCheck, GitBranch } from 'lucide-react'
import { api, type Health } from '../lib/api'

export const Route = createFileRoute('/')({ component: Home })

function Home() {
  const [health, setHealth] = useState<Health | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api
      .health()
      .then(setHealth)
      .catch((e) => setError(String(e)))
  }, [])

  return (
    <div className="mx-auto max-w-3xl px-6 py-12">
      <header className="flex items-center gap-3">
        <ShieldCheck className="h-8 w-8 text-emerald-500" />
        <h1 className="text-3xl font-bold tracking-tight">envvar</h1>
      </header>
      <p className="mt-3 text-lg text-gray-600">
        Git-native, post-quantum, agent-ready environment variables.
      </p>

      <section className="mt-8 grid gap-4 sm:grid-cols-3">
        <Feature icon={<GitBranch className="h-5 w-5" />} title="Git-native">
          Commit encrypted <code>.env</code> safely.
        </Feature>
        <Feature icon={<KeyRound className="h-5 w-5" />} title="Post-quantum">
          Hybrid X25519 + ML-KEM-768.
        </Feature>
        <Feature icon={<ShieldCheck className="h-5 w-5" />} title="Zero-knowledge">
          Server stores only ciphertext.
        </Feature>
      </section>

      <section className="mt-10 rounded-lg border border-gray-200 p-4">
        <h2 className="font-semibold">API status</h2>
        {error && <p className="mt-2 text-sm text-red-600">offline: {error}</p>}
        {!error && !health && <p className="mt-2 text-sm text-gray-500">checking…</p>}
        {health && (
          <p className="mt-2 text-sm text-gray-700">
            status <span className="font-mono text-emerald-600">{health.status}</span> · version{' '}
            <span className="font-mono">{health.version}</span>
          </p>
        )}
      </section>
    </div>
  )
}

function Feature({
  icon,
  title,
  children,
}: {
  icon: React.ReactNode
  title: string
  children: React.ReactNode
}) {
  return (
    <div className="rounded-lg border border-gray-200 p-4">
      <div className="flex items-center gap-2 text-gray-900">
        {icon}
        <span className="font-medium">{title}</span>
      </div>
      <p className="mt-1 text-sm text-gray-600">{children}</p>
    </div>
  )
}
