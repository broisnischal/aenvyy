# envvar — Project Plan

> An open-source, fast, free, self-hostable **environment-variable / secrets
> manager** that is **git-native**, **easy to self-host**, and **built for the
> age of AI agents**. Encrypted `.env` in the repo is the single source of
> truth; it fans out to every platform; agents can **use** secrets without ever
> **seeing** them; and it is **post-quantum ready** from day one.

_Created: 2026-06-29 · See [RESEARCH.md](./RESEARCH.md) for the full competitive landscape._

---

## 1. Problem & Motivation

- "I have the `.env` here but not on my other device" — copying `.env` files
  between machines is wasteful and error-prone.
- Plaintext secrets leak into git, logs, and AI-agent context windows.
- Importing/syncing env into platforms (GitHub, Vercel, Azure, K8s) is manual
  and fragmented across tools.
- Existing self-host options are either heavy (Infisical needs Postgres+Redis,
  has a proprietary `ee/` tier) or unfriendly to newcomers (SOPS).

**Core idea:** let a user commit their `.env` to git **encrypted**, safe to live
on their PC or in a public GitHub repo, with one tool that simplifies importing
that env into GitHub, Vercel, and other platforms — plus connectors (Azure Key
Vault, K8s secrets, AWS SM).

---

## 2. Competitive Landscape (summary)

Two families of prior art (full detail in `RESEARCH.md`):

1. **Centralized secrets servers** — Infisical, Phase.dev, OpenBao, Doppler.
   Server holds secrets; SDKs fetch them. Heavy / server-centric.
2. **Git-native encrypted secrets** — **dotenvx** (closest match), SOPS,
   git-crypt, sealed-secrets, dotenv-vault. Secrets encrypted into files
   committed to the repo.

**Closest existing tool:** [dotenvx](https://github.com/dotenvx/dotenvx) — by the
`dotenv` creator. Encrypts `.env` with a public key (ECIES), commit the encrypted
file to git, decrypt at runtime with a private key, has a Vercel integration.

**Key takeaway:** the encrypt-and-commit-to-git crypto is a *solved* problem.
The defensible value is the **multi-platform sync layer** + **agent-native
access** + **post-quantum readiness** — NOT rebuilding encryption or a full
Infisical (that's person-years).

---

## 3. Product Vision & Positioning

**Identity:** *the open-source secrets manager that is git-native, easy to
self-host, and built for AI agents — and quantum-safe in your repo.*

**Audience:** AI/agentic devs, solo devs, senior devs, indie hackers, and small
teams.

Three pillars, held equally as the north star, but **sequenced** so there is a
usable product at each step.

---

## 4. The Three Pillars (Feature Set)

### Pillar A — Agent-native secrets  *(the standout, least-contested wedge)*
Nobody owns this yet. This is the headline differentiator.
- **Built-in MCP server.** Agents reference secrets by name (`{{STRIPE_KEY}}`);
  the runtime injects real values into the subprocess. The agent **never sees
  plaintext**.
- **Use-but-never-see.** Secret values are redacted from agent
  stdout/logs/context.
- **Scoped, time-boxed grants.** e.g. "this agent may use `DATABASE_URL`,
  read-only, this project, for 30 min" — auto-expires.
- **Agent-aware audit trail.** Which agent accessed which secret, and when. No
  competitor does agent-aware auditing.

### Pillar B — Git-native core + one-command multi-platform sync  *(the dotenvx gap)*
- **Encrypt `.env`, commit safely to git** (dotenvx-style: public key encrypts,
  private key decrypts; private key never in the repo).
- **Two-way sync:**
  - `envvar sync` → push decrypted values to GitHub Actions, Vercel, Netlify,
    Cloudflare, Fly, Render…
  - `envvar pull` → import *from* those platforms and from Azure KV / K8s /
    AWS SM — onboard an existing project in seconds.
- **Build-time injection:** `envvar run -- next build`.
- **Diffable git history** — show *which keys* changed in a commit without
  revealing values.
- **Secret references / composition:** `DATABASE_URL = postgres://{{DB_USER}}:{{DB_PASS}}@...`.

### Pillar C — Newcomer-proof + easy self-host
- **Guided `envvar init`** — sets up keys + `.gitignore` correctly,
  automatically.
- **Pre-commit guard** — blocks pushing a plaintext `.env` or the private key.
- **Key recovery / escrow** — Shamir secret-sharing or KMS escrow so "lost key =
  secrets gone forever" stops being a thing.
- **Per-environment keys** (dev / staging / prod) so a leaked dev key ≠ prod
  compromise.
- **Single static binary + single Docker** (SQLite-backed, no Postgres+Redis).
- **Fully open, no proprietary `ee/` tier.**

### Table-stakes done well (devs love)
- Fast build-time injection, single binary, fully open, diffable history,
  secret composition.

---

## 5. Post-Quantum Cryptography (PQC) Design

**Why it matters most for *this* product:** the attack is **"harvest now,
decrypt later" (HNDL)** — an adversary clones the public repo today, stores the
encrypted `.env`, decrypts it once a quantum computer exists. For git-committed
secrets the ciphertext lives forever and the plaintext is a credential → HNDL is
a real, present threat. This justifies **hybrid PQC now**, not later.

### Only one layer is actually vulnerable
| Layer | Role | Quantum status | Action |
|---|---|---|---|
| **Symmetric** (AES-256-GCM / ChaCha20-Poly1305) | encrypts secret values | already safe (Grover only halves → AES-256 ≈ 128-bit) | keep AES-256 |
| **Asymmetric KEM** (X25519 / ECIES wraps the data key) | wraps the symmetric key | **broken by Shor** — the weak link | make hybrid PQC |
| **Signatures** (Ed25519 — audit log / key integrity) | authenticity | broken by Shor | hybrid signatures |

→ The whole PQC migration = **replace the KEM + signature layers; leave the
symmetric layer alone.**

### Design decisions
1. **Crypto-agility via a versioned, self-describing envelope** (this *is* the
   "adaptable" part). Every ciphertext carries its algorithm IDs:
   ```
   ENVVAR:v1:<kem_alg_id>:<aead_alg_id>:<encapsulated_key>:<nonce>:<ciphertext>:<tag>
   ```
   - An **algorithm registry** maps IDs → implementations behind one interface
     (`Kem.encapsulate/decapsulate`, `Aead.seal/open`, `Sig.sign/verify`).
   - Alg ID in the data → introduce new algorithms while still decrypting old
     files.
   - Ship `envvar rekey` to re-wrap every secret under a new algorithm/key
     (also serves rotation).
2. **Run hybrid KEM now** (classical + PQC; derive the data key from both shared
   secrets via HKDF — safe if *either* holds):
   - **KEM:** `X25519` + `ML-KEM-768` (FIPS 203, formerly Kyber) — same as TLS
     `X25519MLKEM768`.
   - **Signatures:** `Ed25519` + `ML-DSA-65` (FIPS 204, formerly Dilithium).
     SLH-DSA (FIPS 205 / SPHINCS+) only where hash-based conservatism is wanted.
3. **Libraries (2026, with maturity caveats):**
   - Go: `crypto/mlkem` (stdlib since Go 1.24) — no CGo, fits "single static
     binary".
   - Rust: `ml-kem` / `ml-dsa` (RustCrypto) or `aws-lc-rs`.
   - C / cross-language: `liboqs` (Open Quantum Safe) or `aws-lc`; Cloudflare
     CIRCL for Go.
   - Prefer audited stdlib / aws-lc / RustCrypto over hand-rolled; watch
     side-channels.

### How PQC plugs into the pillars
- **B:** the committed encrypted `.env` is HNDL-resistant from day one →
  "quantum-safe secrets in your repo" marketing point.
- **A:** agent grant tokens are signed → hybrid signatures.
- **C:** Shamir splitting operates on the symmetric data key → algorithm-agnostic.

---

## 6. Roadmap / Sequencing

All three pillars remain the goal; this is the order that yields a usable product
at each step.

- **v0 — Core that earns trust**
  - Git-native encrypt/decrypt (hybrid X25519+ML-KEM-768, AES-256-GCM values)
  - Versioned crypto envelope + algorithm registry (crypto-agility baked in)
  - `envvar run -- <cmd>` build-time injection
  - Guided `envvar init` + pre-commit guard
- **v1 — The hook (sync)**
  - `envvar sync` / `envvar pull` for top 3 platforms (GitHub Actions, Vercel,
    + one cloud)
  - Per-environment keys; `envvar rekey`
- **v2 — The differentiator (agents)**
  - Built-in MCP server; use-but-never-see redaction; scoped/time-boxed grants;
    agent audit trail
- **v3 — Server plane (web UI + hosted fetch)** — now a first-class plane, not an afterthought
  - Web frontend: create projects, add environments, manage secrets, members,
    API keys, audit (React SPA embedded in the Go binary via `go:embed`)
  - **Zero-knowledge by default** (server stores only ciphertext; UI/SDK crypto
    client-side / WASM); per-environment opt-in **server-readable** mode
  - SDK network fetch with API keys (machine identities): fetch-once + in-memory
    cache + ETag/304 + git-cache fallback → fast over the network
  - **Self-host single Docker first** (SQLite WAL default; Postgres optional);
    hosted/edge edition later from the same API
  - Key escrow / recovery (Shamir / KMS); additional connectors (Azure KV, K8s,
    AWS SM full matrix)

---

## 7. Key Decisions & Guardrails

- **Don't rebuild encryption** — the dotenvx model works; build the envelope +
  agility layer on top.
- **Don't rebuild a full Infisical** — RBAC, audit, 6 SDKs, full connector
  matrix is person-years; stay focused on sync + agents + PQC.
- **Tech stack leans Go** (single static binary, stdlib `crypto/mlkem`, easy
  cross-compile, single Docker) — to be confirmed.
- **Fully open, no proprietary tier** — positioning win over Infisical.

---

## 8. Open Questions / Next Steps

- [ ] Confirm implementation language (Go strongly favored for the binary +
      PQC stdlib story).
- [ ] Spec the concrete crypto envelope format + provider interface (v0).
- [ ] Define the CLI surface and SDK shape (`init`, `run`, `sync`, `pull`,
      `rekey`, `encrypt`).
- [ ] Design the agent/MCP grant model in detail (scopes, TTL, redaction).
- [ ] Decide initial sync targets and the connector plugin interface.
- [ ] Name / branding.

---

## 9. Sources

See [RESEARCH.md](./RESEARCH.md#sources) for the full source list (dotenvx,
SOPS, Infisical, Phase.dev, OpenBao, External Secrets Operator, etc.).
