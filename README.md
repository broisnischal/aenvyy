# envvar

Git-native, post-quantum, agent-ready environment-variable manager.

- **Git-native** — commit an encrypted `.env` safely to your repo; only the
  private-key holder can read it.
- **Post-quantum** — values are wrapped with a hybrid **X25519 + ML-KEM-768**
  KEM, so committed ciphertext resists "harvest now, decrypt later" attacks.
- **Multi-recipient** — each value is encrypted to several public keys (e.g.
  personal + org-recovery), so a lost key isn't catastrophic.
- **One binary** — the CLI and a self-hostable web UI + API ship as a single Go
  binary (the SPA is embedded), runnable from one Docker image.

> Status: early scaffold. The crypto core, env-file store, and CLI
> (`init`/`set`/`get`/`run`/`keygen`) work end-to-end. The server exposes health
> + stubbed `/v1` endpoints and serves the embedded UI. See
> [PLAN.md](./PLAN.md), [ARCHITECTURE.md](./ARCHITECTURE.md), and
> [RESEARCH.md](./RESEARCH.md).

## Layout

```
cmd/envvar          CLI + server entrypoint
internal/crypto     versioned envelope: hybrid KEM + AES-256-GCM, multi-recipient
internal/envfile    .env parse/serialize, selective re-encryption
internal/store      crash-safe atomic file writes
internal/keys       private-key resolution (env var / .env.keys)
internal/config     envvar.toml
internal/server     /v1 HTTP API + embedded SPA (go:embed)
web/                TanStack Start (React) frontend, built to a static SPA
```

## Quick start (CLI)

```bash
go build -o bin/envvar ./cmd/envvar

cd my-project
bin/envvar init                              # keys + envvar.toml + .gitignore
bin/envvar set DATABASE_URL=postgres://...   # encrypts into .env (committable)
bin/envvar set TOKEN=secret --env production # encrypts into .env.production
bin/envvar run -- npm start                  # decrypts in memory, injects, runs
bin/envvar run --env production -- npm start
```

`.env` / `.env.production` hold only ciphertext and are safe to commit.
`.env.keys` holds your private key, is `chmod 600`, and is gitignored.

## Build everything (single binary with web UI)

```bash
make build         # builds the SPA, embeds it, compiles bin/envvar
make run-server    # serves UI + API on :8080
make dev           # Vite dev server (proxies /v1 to a running `envvar server`)
make test          # go test ./...
```

## Docker

```bash
docker build -t envvar .
docker run -p 8080:8080 envvar      # web UI + API at http://localhost:8080
```

## Security model

- The server is **zero-knowledge by default**: it stores and serves only
  ciphertext envelopes; encryption/decryption happen client-side.
- AES-256-GCM protects each value; the per-value data key is wrapped per
  recipient via the hybrid KEM. Any one recipient's private key decrypts.
- Algorithm ids are embedded in every ciphertext (`enc:v1:...`) for
  crypto-agility — new schemes can be added while old values still decrypt.
