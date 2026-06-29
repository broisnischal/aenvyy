# Using envvar

A practical guide to **envvar** — a tool that lets you commit your secrets to
git **encrypted**, then use them transparently at runtime.

- New here? Read [The mental model](#the-mental-model) first.
- Just want commands? Jump to [Quickstart](#quickstart) and the
  [Command reference](#command-reference).

---

## The mental model

A normal `.env` file holds plaintext secrets, so you `.gitignore` it — and then
you're stuck copying it between machines and teammates by hand.

envvar flips that around:

```
  "postgres://…"  ──encrypt──▶  DATABASE_URL="enc:v1:Yk2p…"  ──commit──▶  git repo
                                                                            │
                                                                  git clone / pull
                                                                            │
                                                                            ▼
   your app  ◀──inject env──  envvar run  ◀──decrypt in memory──  your private key
```

Three things make this safe and useful:

| File | Contains | Committed to git? |
|---|---|---|
| `.env` / `.env.<env>` | only **ciphertext** | ✅ yes — that's the point |
| `.env.keys` | your **private key** | ❌ never (gitignored, `chmod 600`) |
| `envvar.toml` | project config + **public** recipient keys | ✅ yes |

- Anyone can encrypt **to** you using your public key (a `pk_…` "recipient").
- Only your **private key** (`sk_…`) can decrypt. Cloning the repo without it
  reveals nothing.
- Encryption is **hybrid post-quantum** (X25519 + ML-KEM-768), so committed
  ciphertext resists "harvest now, decrypt later" attacks.

---

## Install

```bash
# Build the single binary
go build -o bin/envvar ./cmd/envvar

# Put it on your PATH (also lets the git pre-commit hook find it)
export PATH="$PWD/bin:$PATH"

envvar --help
```

> Building the full app (binary **with** the embedded web UI) instead:
> `make build` → produces `bin/envvar`.

---

## Quickstart

```bash
cd my-project

# 1. One-time setup: keypair + envvar.toml + .gitignore + pre-commit guard
envvar init

# 2. Add secrets (encrypted into .env)
envvar set DATABASE_URL=postgres://localhost/mydb
envvar set STRIPE_KEY=sk_live_abc123

# 3. Inspect — key names only, never values
envvar ls

# 4. Commit the ciphertext safely
git add .env envvar.toml .gitignore
git commit -m "add secrets"          # guard blocks this if anything is still plaintext

# 5. Run your app with real secrets injected (plaintext only in memory)
envvar run -- npm start
```

On another machine: `git clone`, copy your `.env.keys` over **once** (or set the
`DOTENV_PRIVATE_KEY` env var), and `envvar run` just works.

---

## Command reference

### `envvar init`
Sets up a project: generates your keypair (writes the private half to
`.env.keys`), creates `envvar.toml` with you as the `personal` recipient, adds
key/plaintext patterns to `.gitignore`, and installs a `.git/hooks/pre-commit`
guard.

```bash
envvar init                 # project name defaults to the directory name
envvar init --name my-api
```

### `envvar set KEY=VALUE …`
Encrypts one or more values into the env file.

```bash
envvar set API_KEY=secret
envvar set HOST=localhost PORT=5432            # multiple at once
envvar set TOKEN=xyz --env production          # → .env.production
```

### `envvar get [KEY]`
Decrypts and prints a value (or all values). Resolves `{{REF}}` composition.
Handle with care — this prints plaintext.

```bash
envvar get DATABASE_URL
envvar get                  # all key=value pairs
envvar get TOKEN --env production
```

### `envvar ls` (alias `list`)
Lists keys and whether each is encrypted — **never prints values**. Safe to run
anywhere, including shared terminals.

```bash
envvar ls
#   DATABASE_URL    encrypted
#   STRIPE_KEY      encrypted
```

### `envvar rm KEY …` (alias `unset`)
Removes keys from the env file.

```bash
envvar rm OLD_TOKEN
envvar rm A B C --env production
```

### `envvar run -- <command>`
The everyday command. Decrypts in memory, resolves references, injects the
values as environment variables, and execs your command. Plaintext never hits
disk. The child process's exit code is propagated.

```bash
envvar run -- npm start
envvar run -- node server.js
envvar run --env production -- ./deploy.sh
```

### `envvar encrypt`
Encrypts any remaining plaintext values in the file in place (e.g. after you
hand-edited `.env`). Already-encrypted values are left byte-for-byte unchanged,
so git diffs stay clean.

```bash
envvar encrypt
envvar encrypt --env production
```

### `envvar rekey`
Re-wraps every secret under fresh per-value keys. Use it to rotate keys or to
grant a new recipient access to **all** existing secrets.

```bash
envvar rekey                                      # rotate
envvar rekey --add-recipient teammate=pk_AbC123…  # grant + persist to envvar.toml
```

### `envvar keygen`
Prints a fresh keypair without touching any project. Useful to generate a
teammate's or CI's keys.

```bash
envvar keygen
#   sk_…   (private — keep secret)
#   pk_…   (public recipient — safe to share)
```

### `envvar guard`
The pre-commit check (installed by `init`, runnable directly). Scans the git
staging area and **fails the commit** if it would commit `.env.keys` or any env
file with an unencrypted value.

```bash
envvar guard
```

### `envvar server`
Runs the self-hostable web UI + API (see [Team / server mode](#team--server-mode)).

```bash
envvar server                         # :8080, db at ./envvar.db
envvar server --addr :9000 --db /data/envvar.db
```

### Planned (not implemented yet)
- `envvar sync` / `envvar pull` — push/import secrets to/from GitHub Actions,
  Vercel, Azure Key Vault, etc.
- `envvar mcp` — agent MCP server: let AI agents **use** secrets without ever
  **seeing** the plaintext.

---

## Environments

Each environment is a separate file with its own secrets:

| `--env` value | File |
|---|---|
| `default` (the default) | `.env` |
| `production` | `.env.production` |
| `staging` | `.env.staging` |

```bash
envvar set DB=prod-db --env production
envvar run --env production -- npm start
```

Private keys can be per-environment too. envvar resolves a key in this order:

1. `DOTENV_PRIVATE_KEY_<ENV>` environment variable (e.g.
   `DOTENV_PRIVATE_KEY_PRODUCTION`) — ideal for CI/Docker.
2. `DOTENV_PRIVATE_KEY` environment variable.
3. The local `.env.keys` file.

So in CI you never ship `.env.keys` — you set `DOTENV_PRIVATE_KEY` as a CI
secret, commit the encrypted `.env`, and `envvar run` decrypts at build time.

---

## Secret composition

Values can reference other keys with `{{NAME}}`; references resolve at `get` /
`run` time and are never stored expanded.

```bash
envvar set DB_USER=admin
envvar set DB_PASS=s3cr3t
envvar set 'DATABASE_URL=postgres://{{DB_USER}}:{{DB_PASS}}@localhost/app'

envvar get DATABASE_URL
#   postgres://admin:s3cr3t@localhost/app
```

References may chain; reference cycles are detected and reported. A `{{…}}` that
doesn't match any key is left untouched.

---

## Team / server mode

For sharing across a team, run the self-hostable server — one binary, one
SQLite file, no Postgres/Redis.

```bash
make build && make run-server          # → http://localhost:8080
# or
docker build -t envvar . && docker run -p 8080:8080 envvar
```

The web UI lets you create projects, pick environments, and manage each
environment's encrypted bundle. The server is **zero-knowledge**: it stores only
ciphertext and never holds a decryption key — encryption/decryption happen on
the client. An unchanged bundle returns `304` so SDK fetches are cheap.

---

## Security model (in one screen)

- **AES-256-GCM** encrypts each value under a random per-value data key.
- That data key is wrapped **per recipient** with a hybrid KEM (**X25519 +
  ML-KEM-768**); any one recipient's private key can unwrap it.
- Every ciphertext is self-describing (`enc:v1:…`) so new algorithms can be
  added later while old values still decrypt (crypto-agility).
- The private key lives in `.env.keys` (gitignored, `0600`) or a CI secret —
  **never** in the repo. The pre-commit guard enforces this.

---

## FAQ

**Do I have to encrypt — can't I just gitignore `.env`?**
You can, and for a throwaway solo project that's fine. envvar earns its keep when
secrets need to travel: across your own machines, to teammates, into CI, or when
the repo is public. You stop copying `.env` around by hand.

**What if I lose my private key?**
Anything encrypted only to that key is unrecoverable — which is why you add a
second recipient early (a teammate, or an org-recovery key) with
`rekey --add-recipient`. Key escrow/recovery (Shamir) is on the roadmap.

**Is committing encrypted secrets actually safe?**
Yes — the committed file is ciphertext, and the design is post-quantum hybrid to
resist future "decrypt the old ciphertext" attacks. Just never commit
`.env.keys` (the guard blocks it).

**Does `envvar run` slow down my app?**
No. Decryption is microseconds and happens once at startup; your app then reads
normal environment variables.
