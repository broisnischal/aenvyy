# envvar — Architecture

> How the system is built, how envs are processed and stored, and how secrets
> and keys are kept safe without breaking the system or losing data.
>
> Companion to [PLAN.md](./PLAN.md) (product/roadmap) and [RESEARCH.md](./RESEARCH.md) (landscape).

_Created: 2026-06-29_

## Locked decisions

- **Language:** Go (single static binary, stdlib `crypto/mlkem`, easy Docker).
- **Encryption unit:** **per-value** — each value independently encrypted; add/
  edit with only the public key; clean surgical git diffs.
- **Recovery model:** **multi-recipient + Shamir** — each value is encrypted to
  several public keys (e.g. personal + org-recovery); any one private key
  decrypts; private keys can be Shamir-split (e.g. 2-of-3). Lost key ≠ lost data.
- **Storage (git plane):** plain files in the git repo. **No database, no daemon.**
- **Crypto:** hybrid X25519 + ML-KEM-768 (KEM), AES-256-GCM (values), versioned
  envelope + algorithm registry for crypto-agility.
- **Two planes, one envelope:** a **git plane** (local files) and a **server
  plane** (web UI + API + SDK fetch). Both store the *same* `enc:v1:…` envelope;
  an API key maps to a recipient keypair.
- **Server trust model:** **zero-knowledge by default** (server stores only
  ciphertext; UI/SDK do crypto client-side), with a **per-environment opt-in
  "server-readable" mode** for teams that want managed server-side connectors.
- **Server delivery:** **self-host single Docker first** — React SPA embedded in
  the Go binary via `go:embed`, **SQLite (WAL)** default store. One binary, one
  image. Hosted/edge edition comes later from the same API.

---

## 1. Tech stack

| Concern | Choice | Why |
|---|---|---|
| Language | **Go** | Single static binary, cross-compile, small Docker, `crypto/mlkem` in stdlib (Go 1.24). |
| CLI | `cobra` + `viper` | Standard subcommands + config. |
| Symmetric | stdlib `crypto/aes` (GCM) | AES-256-GCM; already quantum-safe. |
| KEM | stdlib `crypto/mlkem` + `x/crypto/curve25519` | Hybrid X25519 + ML-KEM-768. |
| KDF / key-wrap | stdlib `crypto/hkdf` + AEAD | Combine KEM shared secrets; wrap per-value data keys. |
| Signatures | `Ed25519` (stdlib) + `ML-DSA-65` | Hybrid signing for grants/audit integrity. |
| Local storage | files in repo | No DB → minimal "breaks the system" risk. |
| Server (v3 only) | SQLite + WAL | Single file, crash-safe, no Postgres/Redis. |
| SDKs | Go core; format spec ported to Node/Python | Envelope format is language-agnostic. |

The entire v0–v2 product is **a CLI + files in git**. No database, no background
process.

---

## 2. Data model — files & formats

Everything stays `.env`-compatible. **Keys readable, values encrypted** → clean
diffs.

### `.env.<environment>` — committed, safe to be public
```bash
#/ public keys — safe to commit; used to ENCRYPT new values
DOTENV_PUBLIC_KEY_ALG="hybrid-x25519-mlkem768/v1"
DOTENV_RECIPIENTS="personal=pk_04ab… , org-recovery=pk_09cd…"

DATABASE_URL="enc:v1:<nonce>:<ciphertext>:<tag>:<wrap[personal]>;<wrap[org-recovery]>"
STRIPE_KEY="enc:v1:…"
PUBLIC_BASE_URL="https://api.example.com"   # non-secrets may stay plaintext
```

### `.env.keys` — gitignored, `chmod 600`, never committed
```bash
DOTENV_PRIVATE_KEY_PRODUCTION="sk_…"
DOTENV_PRIVATE_KEY_STAGING="sk_…"
```

### `envvar.toml` — committed, project config
```toml
[project]
name = "myapp"

[crypto]
kem  = "hybrid-x25519-mlkem768"   # crypto-agility selector
aead = "aes-256-gcm"

[recipients]
personal     = "pk_04ab…"
org-recovery = "pk_09cd…"

[environments]
production = ".env.production"
staging    = ".env.staging"

[sync.github]
repo = "org/repo"
[sync.vercel]
project = "prj_…"
```

**Core property:** the public keys live in the committed file, so *anyone can add
or edit a secret with only the public key*; **only a private-key holder can
read**. CI/build environments only ever decrypt.

---

## 3. Crypto processing flow (per-value, multi-recipient)

### Encrypt a value (needs only public keys)
1. Generate a random per-value `data_key`.
2. `nonce, ciphertext, tag = AES-256-GCM(data_key, value)`.
3. For **each recipient public key**:
   - Hybrid KEM encapsulate → `(x25519_ss, mlkem_ss, kem_ct)`.
   - `kek = HKDF(x25519_ss ‖ mlkem_ss)`.
   - `wrap = AEAD(kek, data_key)`  → store as `<recipient>=<kem_ct>|<wrap>`.
4. Serialize `enc:v1:nonce:ciphertext:tag:wrap[r1];wrap[r2];…`.

### Decrypt a value (needs any one private key)
1. Find the `wrap` block for a recipient whose private key you hold.
2. KEM-decapsulate with the private key → recover `kek` → unwrap `data_key`.
3. AEAD-open the ciphertext with `data_key`.

This is the age/PGP multi-recipient model: the value is encrypted **once**; the
small `data_key` is wrapped **once per recipient**.

### Two rules that prevent corruption and diff noise
- **Selective re-encryption:** on save, only re-encrypt values whose *plaintext*
  changed; unchanged values are written **byte-for-byte identical** → git diff
  shows only real changes. (Random nonces would otherwise churn every line.)
- **Round-trip validation before write:** after encrypting, decrypt-verify in
  memory. Never overwrite the file with something that can't be decrypted.

> Size: ML-KEM-768 adds ~1 KB per recipient per value. Fine for env files. A
> future "compact mode" (one wrapped data-key for the whole file) is an option
> for projects with hundreds of secrets, at the cost of needing the private key
> to add values.

---

## 4. Key storage & durability — "without breaking the system or losing data"

### (a) Where private keys live — tiered, never in git
| Environment | Storage | Notes |
|---|---|---|
| Local dev | `.env.keys` (gitignored, `0600`) | Optional OS keychain (macOS Keychain / libsecret / Windows Cred Manager). |
| CI/CD | Platform native secret | GitHub Actions secret · Vercel env · Azure Key Vault. The one bootstrap secret. |
| Docker | BuildKit secret mount (build) · `-e` / Swarm secret (run) | Never `ENV`/`ARG` (leaks into image layers). |
| Per-environment | Separate key per env | Leaked dev key ≠ prod compromise. |

### (b) Don't lose everything — escrow / recovery (multi-recipient + Shamir)
- **Multi-recipient by default:** every value is encrypted to `personal` **and**
  `org-recovery` public keys → any one private key recovers it.
- **Shamir secret-sharing:** `envvar key split` → e.g. 2-of-3 shares to
  teammates / a vault / offline. Operates on raw key bytes → algorithm-agnostic.
- **Optional KMS escrow:** add a cloud-KMS public key as a recipient so an org
  can recover via KMS even if all human keys are lost.

### (c) Never corrupt the file — atomic, validated writes
Every file mutation:
1. Read + parse current file.
2. Compute new content in memory (selective re-encryption).
3. Round-trip validate (decrypt-check).
4. Write temp file in the **same directory**, `fsync`, then **atomic `rename()`**
   over the original. A crash mid-write leaves the old file intact.
5. Git history = durable version log of encrypted values (free backups).

Server mode (v3): SQLite WAL (atomic, crash-safe), DB encrypted at rest, same
envelope format inside.

---

## 5. Component architecture

```
   GIT PLANE                              SERVER PLANE
┌──────────────────────┐        ┌──────────────────────────────┐
│ CLI (cobra)          │        │ Web UI (React SPA, go:embed)  │
│ init·run·encrypt·set │        │  + WASM crypto (client-side)  │
│ ·sync·pull·rekey·key │        │ REST/gRPC API  /v1/...         │
└──────────┬───────────┘        │ SQLite(WAL) default / Postgres│
           │                    │ Identity: User · MachineId    │
           │                    │ Audit log                     │
           │                    └───────────┬───────────────────┘
           │   SDKs (Node/Py/Go): fetch-once, cache, 304         │
           │   ── decrypt CLIENT-SIDE (zero-knowledge) ──        │
├──────────┴────────────────────────────────┴───────────────────┤
│ MCP / Agent plane: grants · use-but-never-see · audit          │
├───────────────┬────────────────┬───────────────────────────────┤
│ Key Manager   │ Storage layer  │ Sync / Connectors             │
│ file/keychain │ parse .env     │ GitHub · Vercel · Azure KV    │
│ /env/KMS;     │ atomic write   │ · K8s · AWS SM (two-way)      │
│ Shamir·escrow │ selective re-  │                               │
│ multi-recip   │ encrypt·rt val │                               │
├───────────────┴────────────────┴───────────────────────────────┤
│ Crypto core (shared by all planes): Envelope · Algorithm Reg.   │
│   KEM(hybrid x25519+ml-kem768) · AEAD(aes-256-gcm) · Sig        │
└─────────────────────────────────────────────────────────────────┘
       Both planes store the SAME enc:v1:… envelope.
```

The **Algorithm Registry** behind one interface makes crypto-agility real: the
`enc:v1:` prefix selects the implementation, so new `v2` algorithms can be added
while still decrypting `v1` files. `envvar rekey` walks every value and re-wraps
it under a new algorithm/key/recipient set.

---

## 5b. Server plane — web UI + API + network fetch

The second plane: a hosted store with a web frontend (create projects, add envs)
and an API/SDK so apps fetch secrets over the network with an API key. Shares the
crypto core and envelope format with the git plane.

### Trust model — zero-knowledge by default
- **Default (zero-knowledge):** the server stores **only ciphertext**. The web
  UI encrypts/decrypts **in the browser** (Go crypto core compiled to **WASM**),
  and the SDK decrypts **client-side** after fetching. The API key carries/derives
  the recipient key material. A server breach leaks nothing usable.
- **Opt-in per environment (server-readable):** the server holds a project master
  key (wrapped by a KMS/root key) and decrypts on request. Enables managed
  server-side connectors and easy sharing, at the cost of plaintext exposure on
  breach. Off by default; explicit per-environment toggle.
- Either way, **API keys map to recipients** — an agent (Pillar A) is just
  another machine identity.

### Fast over the network (latency design, not crypto)
Zero-knowledge is **not** slower — the fetch is identical and ML-KEM
decapsulation is microseconds. Speed comes from the access pattern:
- **Fetch-once, cache-in-memory:** SDK pulls the whole env bundle in **one
  round-trip** on boot/build, then serves from memory.
- **Conditional GET / ETag versioning:** unchanged bundle → `304` → ~0 latency.
- **Edge-deployable read path** (hosted edition): serve reads from edge
  (Workers+KV / read replicas). Secrets are read far more than written.
- **Local/git cache fallback:** a network blip never blocks a build.
- **HTTP/2 keep-alive** (optionally gRPC).

### Identity & access
- `User` — web login (sessions, optional SSO later).
- `Machine Identity` — API keys / service tokens scoped to **project +
  environment**, read-only, with **TTL + rotation**. Same grant model the
  agent/MCP plane uses.
- Every read/write/grant written to an **audit log**.

### Stack (single Docker)
- **Frontend:** React/SPA bundle embedded via `go:embed` → still one binary.
- **API:** REST/JSON (+ optional gRPC), versioned `/v1/...`.
- **Store:** **SQLite (WAL)** default (single Docker); **Postgres optional** for
  scale. Stores ciphertext envelopes + metadata + audit log.

### Data model (server)
```
Org ─< Project ─< Environment ─< Secret(envelope)
                      │
                      ├─< MachineIdentity (API key → recipient, scope, TTL)
                      └─< Member (User, role)
AuditEvent(actor, action, secret_ref, ts)
```

---

## 6. CLI surface (initial)

| Command | Purpose |
|---|---|
| `envvar init` | Guided setup: generate keys, write `envvar.toml`, fix `.gitignore`, install pre-commit guard. |
| `envvar set KEY=val` | Encrypt a value into the env file (public key only). |
| `envvar encrypt [env]` | Encrypt/normalize a whole file. |
| `envvar run -- <cmd>` | Decrypt in memory and inject into a subprocess (build/runtime). |
| `envvar sync` / `pull` | Two-way platform sync (GitHub/Vercel/Azure/K8s/AWS). |
| `envvar rekey` | Re-wrap all values under a new alg/key/recipients. |
| `envvar key split / combine / add-recipient` | Shamir + multi-recipient management. |
| `envvar mcp` (v2) | Run the agent MCP server (scoped grants + redaction + audit). |

---

## 7. Failure modes designed against

| Risk | Mitigation |
|---|---|
| Lost private key → data unrecoverable | Multi-recipient (org-recovery) + Shamir + optional KMS escrow. |
| Plaintext `.env` committed | `envvar init` pre-commit guard blocks plaintext `.env` / private keys. |
| File corrupted mid-write | Atomic temp-file + `fsync` + `rename`; round-trip validation before write. |
| Secret baked into Docker image | BuildKit secret mounts; never `ENV`/`ARG`. |
| Agent leaks plaintext (v2) | Use-but-never-see: inject into subprocess, redact from agent output/logs. |
| Algorithm broken later | Versioned envelope + registry + `rekey`; hybrid already covers single-scheme failure. |
| Harvest-now-decrypt-later | Hybrid PQC (ML-KEM) from day one — committed ciphertext is HNDL-resistant. |
