# Env Var Manager — Prior Art & Landscape Research

> Goal: a fast, open-source, free, self-hostable environment-variable / secrets
> manager. Usable from an SDK, can fetch env at **build time**, deployable via a
> **single Docker** container, with a **Docker-Swarm-secret-like** distribution
> layer, and **connectors** (Azure Key Vault, K8s secrets, etc.).
>
> Refined direction (the part that's actually novel): let a user **commit their
> `.env` to git encrypted**, so the secret can safely live on their PC or in a
> GitHub repo, and the tool **simplifies importing env into GitHub, Vercel, and
> other platforms.**

_Last updated: 2026-06-29_

---

## TL;DR

There are **two different families** of tools here, and the refined goal points
squarely at the second one:

1. **Centralized secrets server** (Infisical, Phase.dev, OpenBao, Doppler) — a
   server holds secrets; clients/SDKs fetch them. Matches the "self-hostable
   server + SDK + connectors" half of the brief.
2. **Git-native encrypted secrets** (dotenvx, SOPS, git-crypt, sealed-secrets) —
   secrets are encrypted **into files committed to the repo**; decryption needs
   a separate key. Matches the **"push .env to git safely + import to Vercel"**
   half — this is the refined vision.

**The single closest existing project to the refined goal is [dotenvx](https://github.com/dotenvx/dotenvx).**
It already does: encrypt `.env` with a public key → commit encrypted file to git
→ decrypt at runtime with a private key → has a Vercel integration. By the
original author of `dotenv`. Before building, evaluate whether dotenvx + a thin
platform-sync layer already solves the problem.

---

## Family 1 — Centralized secrets servers

| Requirement | Infisical | Phase.dev | OpenBao | Doppler |
|---|---|---|---|---|
| Open source | MIT core (`ee/` dir proprietary) | open source | MPL-2.0 (true OSS, Vault fork) | closed / cloud |
| Free self-host | yes (genuinely) | yes | yes | no |
| Single Docker | needs Postgres + Redis | simpler | stateful, heavier | — |
| SDKs (Node/Py/Go/Java/.NET) | yes | yes | HTTP / Vault API | yes |
| Build-time injection | `infisical run -- <cmd>` | CLI inject | via agent | yes |
| Connectors (Azure KV, K8s) | first-class K8s operator + Azure auth/sync | yes | plugins | yes |

- **Infisical** — the incumbent open-source "Doppler/Vault alternative". MIT
  core, free self-host, SDKs in every major language, `infisical run` CLI for
  build/runtime injection, K8s operator, Azure Key Vault sync, ~13k stars.
  Caveats vs. our goals: the **`ee/` directory is proprietary** (dynamic
  secrets, SCIM, LDAP, approval workflows, HSM), and self-hosting is **not a
  single container** (needs PostgreSQL + Redis).
- **Phase.dev** — open-source, DX-focused, self-hostable; reviewed as simpler
  UX than Infisical; SDKs + CLI injection + end-to-end encryption. Closest to
  "free, opensource, easy single self-host" in this family.
- **OpenBao** — Linux Foundation fork of the last MPL-2.0 (truly open) HashiCorp
  Vault. The most "no proprietary catch" option, but heaviest operationally and
  least focused on the simple `.env`-replacement DX.

## Family 2 — Git-native encrypted secrets (matches the refined goal)

| Tool | Encrypts | Commit to git? | Key/encryption | Platform sync | Notes |
|---|---|---|---|---|---|
| **dotenvx** | `.env` files | yes (encrypted `.env`) | ECIES, `DOTENV_PUBLIC_KEY` / `DOTENV_PRIVATE_KEY` | **yes — Vercel docs, runtime `dotenvx run`** | By the `dotenv` creator. Closest match. |
| **SOPS** (getsops) | values in YAML/JSON/ENV/INI | yes (values only, keys readable, diffable) | age, PGP, AWS/GCP/Azure KMS | via KMS providers | Per-secret permissions; great for GitOps/Flux. |
| **git-crypt** | whole files | yes (transparent on commit/checkout) | GPG or symmetric key | none built-in | Encrypts entire file → diffs useless. |
| **sealed-secrets** | K8s Secrets | yes | public/private, cluster-only decrypt | K8s only | Doesn't scale across many clusters. |
| **dotenv-vault** | `.env` → `.env.vault` | commit `.env.vault` | `DOTENV_KEY` | yes | Predecessor to dotenvx. |

### How dotenvx works (the model to study)
- `dotenvx encrypt` generates a `DOTENV_PUBLIC_KEY` (encrypt) and
  `DOTENV_PRIVATE_KEY` (decrypt). Each secret is encrypted with an ephemeral
  key (ECIES) decryptable by the long-term private key.
- The **encrypted `.env` is safe to commit**. The private key is NOT committed —
  it lives in `.env.keys` (gitignored) or a cloud secret manager.
- At boot, `dotenvx run -- <cmd>` reads the private key and injects decrypted
  vars just-in-time → covers **build-time / runtime injection**.
- **Vercel**: set `DOTENV_PRIVATE_KEY_PRODUCTION` in Vercel, commit the
  encrypted `.env.production` to code. Same pattern generalizes to other PaaS.
- Attacker needs **both** the encrypted file and the private key → exposure of
  the repo alone is safe.

---

## What's actually still a gap (where a new tool could differentiate)

The refined goal mostly exists (dotenvx). To justify building, pick a real
differentiator — strongest candidates:

1. **One-command platform import/sync as a first-class feature.** dotenvx has
   docs per platform but the "push encrypted to git AND auto-sync decrypted into
   GitHub Actions secrets / Vercel / Netlify / Cloudflare / Fly" flow is still
   manual and fragmented. A polished, multi-platform **`envvar sync`** is a clear
   value-add.
2. **True single-binary / single-container self-host** (SQLite-backed, no
   Postgres+Redis) — a real DX win over Infisical if you also want a server mode.
3. **Decentralized, Swarm-secret-style mesh distribution** — nobody in either
   family does peer-to-peer/gossip propagation; everyone is centralized
   client-server or static-file-in-git. This is the most architecturally novel
   angle from the original brief.
4. **Fully open, zero proprietary tier** — positioning win over Infisical's
   `ee/` model.
5. **Connectors as a unifying layer** — read/write across Azure KV, K8s secrets,
   AWS SM, GitHub/Vercel env, *and* the git-encrypted file, so the encrypted
   `.env` in the repo is the single source of truth that fans out everywhere.
   (Compare: External Secrets Operator does this *into* K8s only.)

### Recommended shape for this project
A **git-native encrypted `.env` core (dotenvx-style) + a strong multi-platform
sync/import layer + optional connectors**, with the encrypted file in the repo as
the source of truth. Optionally add a lightweight self-hostable server later for
teams who want central rotation/audit. Avoid rebuilding a full Infisical — that's
person-years (crypto, RBAC, audit, 6 SDKs, connector matrix).

---

## Product Vision & Differentiating Features

**Identity:** the open-source secrets manager that is *git-native, easy to
self-host, and built for the age of AI agents* — all three at once. Encrypted
`.env` in the repo is the single source of truth; it fans out to every platform;
and agents can **use** secrets without ever **seeing** them.

Three pillars, held equally, but **sequenced** so there's a usable product at
each step. Audience: AI/agentic devs, solo devs, senior devs, indie hackers, and
small teams.

### Pillar A — Agent-native secrets (the standout, least-contested wedge)
Nobody owns this yet. This is the headline.
- **Built-in MCP server.** Agents reference secrets by name (`{{STRIPE_KEY}}`);
  the runtime injects real values into the subprocess. The agent **never sees
  plaintext.**
- **Use-but-never-see.** Values are redacted from agent stdout/logs/context.
- **Scoped, time-boxed grants.** "This agent may use `DATABASE_URL`, read-only,
  this project, for 30 min." Auto-expires.
- **Agent-aware audit trail.** Which agent accessed which secret, when. No
  competitor does this.

### Pillar B — Git-native core + one-command multi-platform sync (the dotenvx gap)
- **Encrypt `.env`, commit safely to git** (dotenvx-style ECIES: public key
  encrypts, private key decrypts; private key never in repo).
- **Two-way sync.** `envvar sync` pushes decrypted values to GitHub Actions,
  Vercel, Netlify, Cloudflare, Fly, Render… `envvar pull` imports *from* them
  (and from Azure KV / K8s / AWS SM) to onboard an existing project in seconds.
- **Build-time injection.** `envvar run -- next build`.
- **Diffable history.** Show *which keys* changed in a commit without revealing
  values. **Secret references / composition** (`DATABASE_URL = ...{{DB_PASS}}...`).

### Pillar C — Newcomer-proof + easy self-host
- **Guided `envvar init`** — sets up keys + `.gitignore` correctly, automatically.
- **Pre-commit guard** — blocks pushing a plaintext `.env` or the private key.
- **Key recovery / escrow** — Shamir shares or KMS escrow so "lost key = secrets
  gone forever" stops being a thing. **Per-environment keys** (dev/staging/prod).
- **Single static binary + single Docker** (SQLite-backed, no Postgres+Redis).
- **Fully open, no proprietary `ee/` tier.**

### Suggested sequencing (keeps all three pillars as the goal)
1. **v0 (core that earns trust):** git-native encrypt/decrypt + `run` build-time
   injection + guided `init` + pre-commit guard. (Pillar B core + C safety.)
2. **v1 (the hook):** `envvar sync` / `pull` for the top 3 platforms (GitHub,
   Vercel, + one cloud). Per-env keys. (Pillar B sync.)
3. **v2 (the differentiator):** MCP server + use-but-never-see + scoped grants +
   agent audit. (Pillar A — the positioning nobody else owns.)
4. **v3 (teams):** optional lightweight self-host server for shared rotation +
   central audit; key escrow/recovery.

> Reality check: the crypto + commit-to-git piece already exists (dotenvx). The
> defensible value is the **sync layer + agent-native access**, not rebuilding
> encryption. Don't rebuild a full Infisical — that's person-years.

## Sources

- [dotenvx (GitHub)](https://github.com/dotenvx/dotenvx) · [dotenvx.com](https://dotenvx.com/) · [Encryption docs](https://dotenvx.com/docs/quickstart/encryption) · [Vercel platform docs](https://dotenvx.com/docs/platforms/vercel)
- [dotenv-vault (GitHub)](https://github.com/dotenv-org/dotenv-vault)
- [SOPS (getsops/sops)](https://github.com/getsops/sops) · [SOPS guide (GitGuardian)](https://blog.gitguardian.com/a-comprehensive-guide-to-sops/) · [4 git secret tools (opensource.com)](https://opensource.com/article/19/2/secrets-management-tools-git)
- [Infisical (GitHub)](https://github.com/Infisical/infisical) · [Infisical LICENSE](https://github.com/Infisical/infisical/blob/main/LICENSE) · [Infisical EE / self-hosting](https://infisical.com/docs/self-hosting/ee)
- [Phase.dev](https://phase.dev/)
- [HashiCorp Vault alternatives / OpenBao](https://infisical.com/blog/hashicorp-vault-alternatives)
- [External Secrets Operator + Azure KV / K8s](https://oneuptime.com/blog/post/2026-01-19-kubernetes-external-secrets-vault-integration/view)
- [Top secrets management tools 2026 (GitGuardian)](https://blog.gitguardian.com/top-secrets-management-tools/)
