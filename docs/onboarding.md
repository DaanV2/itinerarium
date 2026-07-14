# Onboarding

Your first day on Itinerarium. This gets you from a fresh clone to a running stack and points you at the right doc for whatever you do next. If you're here to build a feature, skim this once, then live in [development.md](development.md).

## What this project is

Itinerarium is a **self-hosted TTRPG campaign tool** — one campaign per installation, for players and a game master. The interesting part is the domain: a game-day-gated documentation system where visibility of documents, inventories, locations, and activity is controlled server-side by permission rules. Those rules are security-relevant and non-negotiable; read the **8 core domain rules** in the root [CLAUDE.md](../CLAUDE.md) before writing anything that touches visibility.

Two halves:

- `api/` — Go API server (GORM + pure-Go SQLite). Layered: transport → application → repositories → models.
- `web/` — SvelteKit + TypeScript frontend (Svelte 5, runes mode).

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.26+ | matches `api/go.mod` |
| Node + npm | 20+ (npm) | package manager is **npm** — not pnpm/yarn/bun |
| [just](https://github.com/casey/just) | any recent | task runner; `just` lists all recipes |
| golangci-lint | v2 | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` |
| Docker (optional) | — | only needed for the full-stack `just up` path |

No cgo/gcc required. The SQLite driver (`glebarez/sqlite`) is pure Go, so everything builds on Windows without a C toolchain. The one thing you lose locally is `go test -race` (CI runs it on Linux) — plain `go test ./...` is fine.

## First run

```bash
git clone git@github.com:DaanV2/itinerarium.git
cd itinerarium

# one-time frontend deps
just web-install

# prove the toolchain works — this is every check CI runs
just verify
```

If `just verify` is green, you're set up correctly. From here, two ways to run it:

### Run the two dev servers (day-to-day)

```bash
just api      # Go API on :8080
just web      # SvelteKit dev server on :5173, proxies /api → :8080
```

Open http://localhost:5173. On first start a **setup wizard** creates the initial GM account. Prefer the CLI? Bootstrap headless instead:

```bash
cd api && go run . init --email gm@example.com --password <password>
```

(Both fail once any account exists — that's intentional.)

### Run the production build (closest to production)

Production is **one binary**: the frontend is compiled to a static SPA and embedded into the Go server, which serves the UI and the API together on `:8080`.

```bash
just build    # builds web, embeds it, produces api/itinerarium(.exe)
just up       # same thing as a single Docker container on :8080 (needs Docker)
```

## Where things live

```
api/
├── cmd/                 # Cobra CLI commands (serve, init)
├── application/         # services — business logic + ALL permission rules
├── infrastructure/
│   ├── authentication/  # RS512 JWT, JTI revocation, key storage
│   ├── config/          # Viper (flags → env → YAML → defaults)
│   ├── persistence/
│   │   ├── models/      # GORM structs
│   │   ├── repositories/# all DB access, one file per entity
│   │   └── migrations.go# register every new model here
│   ├── transport/       # routers, handlers, middleware
│   └── webapp/          # embedded web build (embedweb build tag)
└── main.go

web/src/
├── lib/api/             # typed API client — the ONLY place fetch() is called
├── lib/types.ts         # TS types mirroring API models
└── routes/              # +page.svelte / +page.ts per URL

config/                  # YAML config per environment (see config.example.yaml)
docs/                    # you are here
```

## The mental model that matters

Three things trip up newcomers. Internalize these:

1. **Permission checks live in services, never in repositories or handlers.** A repository returns what it's asked for; the service decides what the caller may see. See the layer table in [api/CLAUDE.md](../api/CLAUDE.md).
2. **Hidden means invisible — 404, not 403.** If a character can't see something, its *existence* must not leak: not in a list, not in search, not in a hit count, and a direct GET returns 404. A 403 confirms the thing exists, which is a leak.
3. **The client renders only what the API returns.** GM-only content is stripped server-side. Never filter it client-side, never hide "forbidden" items with CSS. If a secret reaches the browser payload, that's an API bug to report — not something to patch over in the frontend.

Everything security-relevant flows from those. The full set of invariants (game-day gating, repository access, GM-only stripping, rewind) is in the root [CLAUDE.md](../CLAUDE.md), with the negative tests each one requires in [development.md](development.md).

## Everyday commands

`just` lists everything. The ones you'll actually use:

| Task | Command |
|------|---------|
| Run API | `just api` |
| Run frontend | `just web` |
| Format everything | `just fmt` |
| **Every check CI runs** (before finishing anything) | `just verify` |
| Single production binary (API + embedded web UI) | `just build` |
| Full stack in Docker | `just up` |
| API-only checks | `just api-verify` |
| Web-only checks | `just web-verify` |

## Where to go next

- **Building a feature?** → [development.md](development.md) — the workflow, definition of done, and the security tests every feature needs. Then the copyable per-layer templates in [api/CLAUDE.md](../api/CLAUDE.md) / [web/CLAUDE.md](../web/CLAUDE.md).
- **Understanding the domain?** → [architecture.md](architecture.md) — entities and the permission model. This is the **source of truth**; if a feature seems to conflict with it, it wins.
- **What features exist / are planned?** → [features.md](features.md), [roadmap.md](roadmap.md), [backlog.md](backlog.md).
- **Deploying it?** → [deployment.md](deployment.md).

Golden rule: when you change entities or permission rules, update [architecture.md](architecture.md) in the same change. Docs don't drift here.
