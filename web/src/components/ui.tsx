// Small, dependency-free UI primitives shared across routes. Styling leans on
// the tokens in styles.css plus Tailwind utilities.
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { Loader2 } from 'lucide-react'

type Variant = 'primary' | 'ghost' | 'danger'

const variants: Record<Variant, string> = {
  primary:
    'bg-[var(--color-accent)] text-[#04130d] hover:brightness-110 disabled:opacity-50',
  ghost:
    'border border-[var(--color-line)] text-[var(--color-ink)] hover:bg-[var(--color-surface-2)] disabled:opacity-40',
  danger:
    'border border-[var(--color-danger)]/40 text-[var(--color-danger)] hover:bg-[var(--color-danger)]/10 disabled:opacity-40',
}

export function Button({
  variant = 'primary',
  busy = false,
  className = '',
  children,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant
  busy?: boolean
}) {
  return (
    <button
      {...rest}
      disabled={rest.disabled || busy}
      className={`ring-accent inline-flex items-center justify-center gap-2 rounded-lg px-3.5 py-2 text-sm font-medium transition disabled:cursor-not-allowed ${variants[variant]} ${className}`}
    >
      {busy && <Loader2 className="h-4 w-4 animate-spin" />}
      {children}
    </button>
  )
}

export function Badge({
  children,
  tone = 'neutral',
}: {
  children: ReactNode
  tone?: 'neutral' | 'accent' | 'amber' | 'violet'
}) {
  const tones = {
    neutral:
      'border-[var(--color-line)] text-[var(--color-ink-dim)] bg-[var(--color-surface-2)]',
    accent:
      'border-[var(--color-accent)]/30 text-[var(--color-accent)] bg-[var(--color-accent)]/10',
    amber:
      'border-[var(--color-amber)]/30 text-[var(--color-amber)] bg-[var(--color-amber)]/10',
    violet:
      'border-[var(--color-violet)]/30 text-[var(--color-violet)] bg-[var(--color-violet)]/10',
  }
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 font-mono text-[11px] leading-none ${tones[tone]}`}
    >
      {children}
    </span>
  )
}

export function Spinner({ label }: { label?: string }) {
  return (
    <div className="flex items-center gap-2 text-sm text-[var(--color-ink-dim)]">
      <Loader2 className="h-4 w-4 animate-spin" />
      {label ?? 'Loading…'}
    </div>
  )
}

export function ErrorNote({ error }: { error: Error }) {
  return (
    <div className="rounded-lg border border-[var(--color-danger)]/30 bg-[var(--color-danger)]/10 px-4 py-3 text-sm text-[var(--color-danger)]">
      {error.message}
    </div>
  )
}

export function EmptyState({
  icon,
  title,
  children,
}: {
  icon: ReactNode
  title: string
  children?: ReactNode
}) {
  return (
    <div className="card flex flex-col items-center gap-3 px-6 py-12 text-center">
      <div className="text-[var(--color-ink-faint)]">{icon}</div>
      <h3 className="font-medium text-[var(--color-ink)]">{title}</h3>
      {children && (
        <div className="max-w-md text-sm text-[var(--color-ink-dim)]">{children}</div>
      )}
    </div>
  )
}
