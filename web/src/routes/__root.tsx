import {
  HeadContent,
  Link,
  Scripts,
  createRootRoute,
} from '@tanstack/react-router'
import { TanStackRouterDevtoolsPanel } from '@tanstack/react-router-devtools'
import { TanStackDevtools } from '@tanstack/react-devtools'
import { ShieldCheck } from 'lucide-react'

import appCss from '../styles.css?url'

export const Route = createRootRoute({
  head: () => ({
    meta: [
      { charSet: 'utf-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      { title: 'envvar — git-native, post-quantum secrets' },
      {
        name: 'description',
        content:
          'Git-native, post-quantum, agent-ready environment-variable manager. Zero-knowledge by default.',
      },
    ],
    links: [{ rel: 'stylesheet', href: appCss }],
  }),
  shellComponent: RootDocument,
})

function RootDocument({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <HeadContent />
      </head>
      <body>
        <div className="flex min-h-screen flex-col">
          <Header />
          <main className="flex-1">{children}</main>
          <Footer />
        </div>
        <TanStackDevtools
          config={{ position: 'bottom-right' }}
          plugins={[
            {
              name: 'Tanstack Router',
              render: <TanStackRouterDevtoolsPanel />,
            },
          ]}
        />
        <Scripts />
      </body>
    </html>
  )
}

function Header() {
  return (
    <header className="sticky top-0 z-10 border-b border-[var(--color-line)] bg-[var(--color-canvas)]/80 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-6">
        <Link to="/" className="flex items-center gap-2">
          <ShieldCheck className="h-5 w-5 text-[var(--color-accent)]" />
          <span className="text-[15px] font-semibold tracking-tight">envvar</span>
          <span className="ml-1 hidden font-mono text-[11px] text-[var(--color-ink-faint)] sm:inline">
            zero-knowledge
          </span>
        </Link>
        <nav className="flex items-center gap-5 text-sm text-[var(--color-ink-dim)]">
          <Link
            to="/"
            className="transition hover:text-[var(--color-ink)] [&.active]:text-[var(--color-ink)]"
            activeOptions={{ exact: true }}
          >
            Projects
          </Link>
          <a
            href="https://github.com/nees/envvar"
            className="transition hover:text-[var(--color-ink)]"
            target="_blank"
            rel="noreferrer"
          >
            GitHub
          </a>
        </nav>
      </div>
    </header>
  )
}

function Footer() {
  return (
    <footer className="border-t border-[var(--color-line)]">
      <div className="mx-auto flex max-w-5xl flex-col gap-1 px-6 py-5 text-xs text-[var(--color-ink-faint)] sm:flex-row sm:items-center sm:justify-between">
        <span>
          The server stores <span className="text-[var(--color-ink-dim)]">only ciphertext</span>.
          Encryption happens client-side.
        </span>
        <span className="font-mono">X25519 + ML-KEM-768 · AES-256-GCM</span>
      </div>
    </footer>
  )
}
