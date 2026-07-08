# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**Itinerarium** — a self-hosted TTRPG campaign tool. One campaign per installation. Supports players and game masters with journaling, group management, and a game-day-gated documentation system.

GitHub: `git@github.com:DaanV2/itinerarium.git`
Reference codebase for Go patterns: [`DaanV2/mechanus`](https://github.com/DaanV2/mechanus)

Full docs live in `docs/`: [features](docs/features.md), [architecture](docs/architecture.md), [development workflow](docs/development.md), [deployment](docs/deployment.md), [roadmap](docs/roadmap.md), [backlog](docs/backlog.md). The architecture doc is the source of truth for entities and permission rules — keep it updated when the domain model changes.

**Working on a feature?** Follow [docs/development.md](docs/development.md) — it defines the workflow, definition of done, and the security tests every feature needs. `api/CLAUDE.md` and `web/CLAUDE.md` contain the layer rules and copyable code templates for their side.

**Documenting a feature?** Treat this the same as a human developer would: when you add or change a feature, update the relevant doc in `docs/` (features, architecture, deployment, roadmap, backlog as applicable) in the same change — don't leave docs to drift. New non-obvious behavior, workflows, or "how to do X with this code" knowledge belongs in `docs/` (or the relevant `CLAUDE.md`) rather than only in commit messages or code comments.

## Stack

| Layer | Technology |
|-------|-----------|
| API | Go, GORM, SQLite via `glebarez/sqlite` — pure Go, no cgo (FTS5 for search) |
| Frontend | SvelteKit + TypeScript |
| Deployment | Docker Compose |
| CLI | Cobra |
| Config | Viper (flags → env vars → YAML → defaults) |
| Auth | RS512 JWT with JTI revocation, keys auto-generated on first start |

## Repository Layout

```
api/
├── cmd/                    # Cobra CLI commands
├── application/            # Service / business logic layer
├── infrastructure/
│   ├── authentication/     # JWT, JTI, key storage
│   ├── config/             # Viper setup and manager
│   ├── lifecycle/          # graceful-shutdown phase interfaces
│   ├── persistence/
│   │   ├── models/         # GORM models
│   │   ├── repositories/   # One file per entity
│   │   └── migrations.go
│   ├── servers/            # http.Server wrapper
│   └── transport/          # Routers, middleware, CORS
├── main.go
└── go.mod

web/                        # SvelteKit + TypeScript
config/                     # YAML config files per environment
docker-compose.yml
```

## Commands

A `justfile` at the repo root is the canonical task runner — `just` lists all recipes.

```bash
just api          # run the API on :8080
just web          # run the frontend on :5173 (/api proxied to :8080)
just fmt          # format everything
just verify       # every check CI runs — run this before finishing any feature
just up           # full stack via Docker Compose (API :8080, web :3000)
```

Raw equivalents (what CI runs): from `api/` — `go build ./... && go vet ./... && golangci-lint run ./... && go test ./...` (CI adds `-race`, Linux-only); from `web/` — `npm run lint && npm run check && npm run test && npm run build`. The web package manager is **npm**.

## Go Conventions (from mechanus)

- **Models**: embed a shared `Model` base (UUID `ID` generated in `BeforeCreate`, timestamps, `gorm.DeletedAt` soft delete). `AutoMigrate()` in `migrations.go`. Many-to-many via `many2many:` tags with named junction tables.
- **Repository pattern**: one file per entity under `infrastructure/persistence/repositories/`; all DB access goes through repositories; services in `application/` own business logic.
- **Functional options**: all constructors take `...Option` variadic args (database, server, router setup).
- **Config**: Viper singleton; priority flags → env (`my-flag` → `MY_FLAG`) → YAML → defaults; per-component config contexts in a thread-safe `sync.Map`.
- **HTTP**: standard `http.ServeMux` with functional options (`WithHandle`); server wraps `*http.Server` with 10 s read-header timeout and graceful `Shutdown()`.
- **Lifecycle**: components implement shutdown hooks (`BeforeShutdown()` / `AfterShutDown()` / `ShutdownCleanup()`); Cobra coordinates shutdown via context cancellation with a 1-minute timeout.

## Core Domain Rules

These invariants are security-relevant. Enforce them **server-side**, never in the client.

1. **Game day gates visibility.** `game_day` is an int counter per character, not a date. Documents live in exactly one **repository** (`general`/`template` = everyone, `group` = members, `character` = owner + GM). A character sees a document/activity entry only when `current_game_day >= shared_on_game_day` AND the repository (or a direct share) grants access. Documents are NOT versioned — game day gates *whether* you see a doc, not which revision.
2. **GM-only content is stripped server-side**: document sections flagged `gm_only`, and the `actor` field on announced activity entries, must never reach non-GM clients.
3. **Hidden means invisible.** If a character lacks access to a location inventory, folder, or document, its *existence* must not leak through API responses, search results, or hit counts.
4. **Announcements bypass entity access but not content access**: an announced activity entry (theft, destruction, GM broadcast) reveals that something happened and to what — never the entity's content, never the actor (players).
5. **Journal → document conversion is a copy**: the new doc starts `private` in the character's personal repository; the journal entry is untouched. Journals are readable by owning player + GM only.
6. **Groups are one model**: `type` (`organization`/`family`/`other`) is cosmetic; access follows current membership, and join/leave events are logged as game-day-stamped activity entries.
7. **Anyone who can see a document or location can edit it.** Warn on path collisions within a repository and on concurrent-edit conflicts — warn, don't block. Player edits never touch GM-only sections — if all existing sections are GM-only, the player's edit becomes a new player-visible section.
8. **Item/currency catalogs are conveniences, not restrictions**: GM defines currencies (with conversion ratios) and a default item catalog via JSON/YAML; free-text custom items are always allowed.

## Auth

Email + password. GMs create accounts and reset passwords manually (no SMTP dependency). RS512 JWT, JTI revocation table, keys auto-generated on first start.
