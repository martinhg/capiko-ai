# Engram Cloud — Team Setup

capiko configures **GitHub Copilot** to use [Engram](https://github.com/Gentleman-Programming/engram)
as a shared, cross-session memory layer for the team. This document explains the
model and how each role connects.

The client-side wiring is built into capiko's **Configure engram** flow; the
**manual fallback** steps below are for connecting without the TUI.

## The model in one picture

Two stores, two transports — you always work **local**:

- **Engram memory** — decisions, context, session summaries, and (in `engram`/`hybrid`
  mode) the SDD artifacts — lives in your **local** SQLite at `~/.engram/engram.db`.
  Your agent only ever reads and writes the local store. **Engram Cloud** replicates
  it in the background: your saves are pushed up, teammates' saves are pulled down
  into your local store on the next sync cycle.
- **OpenSpec files** — `openspec/` canonical specs and in-flight change artifacts —
  are plain repo files. They reach teammates through **git** (commit / push / pull),
  reviewed like code. Engram Cloud does **not** sync these, and that is intentional:
  a canonical spec belongs in a PR with history, not in a silent memory sync.

**Team default artifact-store mode: `hybrid`** — canonical specs in git (reviewable,
versioned) **plus** memory and context flowing automatically through Engram Cloud.

**Local-first guarantee:** if the cloud is unreachable, everything keeps working
locally; sync resumes automatically when it is back. The agent never talks to the
cloud directly — cloud sync is purely background replication.

> ⚠️ Pull is **polling-based**, not a real-time push. A teammate's saves arrive on
> your next sync cycle, and only while your Engram is running with autosync enabled.

## What is secret vs shareable

| Item | Secret? | Where it lives |
|------|---------|----------------|
| Cloud server **URL** | No | `~/.engram/cloud.json` (written by `engram cloud config --server`) or `ENGRAM_CLOUD_SERVER`. Safe to put in onboarding docs. |
| Cloud **token** | **Yes** | `ENGRAM_CLOUD_TOKEN` env var, from your secrets manager. **Never commit it.** |
| Project names | No | per-repo `.engram/config.json` (committed with the repo) |

Put the **URL** wherever the team reads onboarding (this doc, a pinned message, an
`.env.example`). Put the **token** in your secrets manager and load it as an env var
per developer.

## For the devops: stand up the server (once)

The cloud server is a Go binary (`engram cloud serve`) backed by Postgres. The
workload is light (text mutation sync), so the concern is **availability,
persistence, and auth** — not scale.

**Where to host (pick one):**

1. **Small always-on VM** in the cloud the company already uses (EC2 `t4g.small`,
   GCP `e2-small`, Azure `B1s`, Hetzner/DO). Run the container behind **Caddy or
   Traefik** for automatic HTTPS, and hand the team an `https://engram.company.com`
   URL. Use managed Postgres via `ENGRAM_DATABASE_URL`, or the bundled Postgres with
   a persistent volume + scheduled `pg_dump`.
2. **VM reachable only over Tailscale/WireGuard** (recommended for a small, fully
   remote team) — no public port, no public TLS cert needed; each developer joins
   the tailnet and uses the MagicDNS name as the URL. Lowest attack surface.
3. **Existing Kubernetes / container platform** — deploy the image
   `ghcr.io/gentleman-programming/engram` with managed Postgres + ingress TLS. Only
   worth it if that platform already exists.

**Do not** run the team server from a laptop or expose port `18080` publicly without
the reverse proxy and auth.

**Run it.** capiko writes a hardened `docker-compose.cloud.yml` scaffold for you
(auth on, secrets read from the environment). Set the variables and start it:

```bash
ENGRAM_PG_PASSWORD=...  ENGRAM_JWT_SECRET=...  ENGRAM_CLOUD_ALLOWED_PROJECTS=repo-core,repo-back \
  docker compose -f docker-compose.cloud.yml up -d
```

**Hardening checklist (required for team use):**

| Variable | Set to |
|----------|--------|
| `ENGRAM_CLOUD_INSECURE_NO_AUTH` | `0` (require auth) |
| `ENGRAM_JWT_SECRET` | a strong, non-default secret |
| `ENGRAM_CLOUD_TOKEN` | a token per member (revocable) |
| `ENGRAM_CLOUD_ALLOWED_PROJECTS` | your real project names (see Multi-repo below) |
| `ENGRAM_DATABASE_URL` | your Postgres DSN (managed or bundled-with-backups) |
| TLS | terminated by Caddy/Traefik in front of the server |

**Deliver to the team:** the HTTPS **URL** and each member's **token**.

## For each team member: connect

**With capiko (recommended).** Run **Configure engram**: capiko detects Engram,
registers the Engram MCP server in your Copilot config (Copilot CLI under
`~/.copilot/` first, VS Code under `Code/User/mcp.json` or `.vscode/mcp.json`), runs
`engram cloud config`, sets the autosync env, and writes the per-repo
`.engram/config.json`.

**Manual fallback (without the TUI):**

```bash
# 1. Install engram (see its INSTALLATION docs)

# 2. Point at the team server (persists to ~/.engram/cloud.json)
engram cloud config --server https://engram.company.com

# 3. Set your env (token from the secrets manager). Autosync needs ALL THREE:
export ENGRAM_CLOUD_SERVER=https://engram.company.com
export ENGRAM_CLOUD_TOKEN=<your-token>
export ENGRAM_CLOUD_AUTOSYNC=1

# 4. Enroll each project you work on
engram cloud enroll <project>

# 5. Verify
engram cloud status
```

Then register the Engram MCP server in your Copilot surface (the JSON entry calls the
`engram` binary over stdio). capiko writes this into `~/.copilot/mcp-config.json`
(top-level key `mcpServers`) with the cloud env, keeping your token as the
`${ENGRAM_CLOUD_TOKEN}` reference so the secret never lands in a config file.

## Multi-repo workspaces

If you open a **parent folder** as one workspace (e.g. `Company/` containing
`repo-back`, `repo-core`, …) and let the agent edit across repos, give **each repo**
its own `.engram/config.json` naming its project. Engram resolves the **nearest**
`.engram/config.json` relative to the file being edited, so a memory about
`repo-core` is attributed to `repo-core` even though the workspace root is the parent
and the MCP process has a single working directory.

- Each repo's `.engram/config.json` names its project (capiko generates this file).
- `engram cloud enroll <project>` for each project.
- Add every project name to the server's `ENGRAM_CLOUD_ALLOWED_PROJECTS`.

> `mem_save(project=…)` is validated: a project name is accepted only when backed by
> an existing project, an active session, or the nearest `.engram/config.json`, so
> typos fail loudly instead of creating a phantom project bucket.

## Verify it works

```bash
engram cloud status                                  # config + daemon health
engram sync --cloud --status --project <project>     # per-project sync state
```

Save a memory on one machine; on a teammate's machine (same project enrolled, same
server), it should appear after the next sync cycle.
