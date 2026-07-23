# Refactor plan: flatter layered layout for `api/`

Status: proposal. Restructures `api/` so the folder tree reflects dependency
direction, and de-clumps `transport/` and `application/` so each package holds
one kind of thing. No behaviour changes — moves, path renames, and import
rewrites only.

## Why

Two problems with the current layout:

1. **Direction is invisible.** `infrastructure/transport` is an *inbound*
   adapter (it drives the app) sitting in the same folder as *outbound* adapters
   (`persistence`, `authentication`) the app drives. Meanwhile `application/` is
   a top-level single package next to a multi-package `infrastructure/`
   namespace, so the tree reads as a grab-bag, not as layers.
2. **Both layers have squatters.** `transport/` mixes per-entity API handlers
   with routing machinery and cross-cutting middleware. `application/` mixes
   use-case services with pure domain rules and a format parser.

## Design decisions

- **Flat layers, named by what they are.** No `core/` / `adapters/inbound|
  outbound/` wrapper folders. Inbound-vs-outbound reads from the names —
  `transport` drives in, `database`/`auth` are driven.
- **Models stay in the database package.** They are GORM-tagged persistence
  DTOs, not dependency-free entities, so they live in `database/models`.
  Consequence: `domain/` operates on them, so `domain` imports
  `database/models`. No import cycle (`models` is a leaf); `domain` here means
  "business rules," not "the pure center." Accepted trade-off.
- **Each layer owns its own data shapes.** `database/models` = DB shapes;
  `transport/handlers` = request/response DTOs (already the pattern — see
  `catalog.go`'s `currencyResponse` + `toCurrencyResponse`). Neither reuses the
  other's structs across the boundary.

## Target tree

Minimal-churn version: `transport` and the new `handlers`/`domain` come to the
top level; everything else that was already in `infrastructure/` stays there.

```
api/
├── cmd/                     # unchanged — CLI entry (Cobra)
├── domain/                  # business rules (imports infrastructure/persistence/models)
│   ├── access.go            # ← application/gate.go + application/sections.go
│   └── documentfmt/         # ← application/frontmatter.go
├── application/             # USE-CASE SERVICES ONLY (gate/sections/frontmatter removed)
│   ├── <entity>.go services # activity, auth, catalog, characters, documents*, groups,
│   │                        #   inventory*, journal_entries, locations, repositories,
│   │                        #   search, sessions, setup, users, vault_import
│   ├── errors.go            # ErrorKind / ServiceError — stays
│   ├── requester.go         # Requester port — stays
│   ├── request_cache.go     # request-scoped caching — stays
│   └── doc.go
├── handlers/                # per-entity HTTP endpoint adapters + request/response DTOs
│   └── activity, auth (LoginHandler), catalog, characters, documents, groups,
│      inventory (+ OwnerExtractor/owner funcs), journal_entries, locations,
│      repositories, search, sessions, setup, users, vault_import
├── transport/               # HTTP MECHANISM only (app-agnostic)
│   ├── router.go            # Router + Middleware type
│   ├── middleware.go        # Logging
│   ├── security.go          # SecurityHeaders, MaxBytes
│   ├── throttle.go          # login/reset backoff limiter
│   ├── errors.go            # WriteServiceError (ErrorKind → HTTP status)
│   ├── requester.go         # RequireAuth, RequesterFrom, bearer-token + ctx plumbing
│   ├── spa.go               # embedded-SPA fallback handler
│   ├── health.go
│   └── server/              # ← infrastructure/servers (http host)
├── infrastructure/          # unchanged bucket — everything the app depends on
│   ├── persistence/         #   (models stay here as DB DTOs, repositories)
│   ├── authentication/  config/  logging/  lifecycle/  webapp/
├── components/              # unchanged location — composition root (wiring)
├── pkg/extensions/xhttp/    # unchanged — generic http helpers
├── test/component/          # unchanged location — integration harness
└── main.go
```

Only three things leave `infrastructure/`: `transport` (to top level, split into
mechanism vs `handlers`) and `servers` (folded under `transport/server`).
`persistence`, `authentication`, `config`, `logging`, `lifecycle`, `webapp`
stay put — no `persistence`→`database` rename, no mass outbound rewrite.

### The transport → transport + handlers split

`transport` currently holds both HTTP machinery and every endpoint handler in one
package. Split by concern:

- **`transport` (mechanism, app-agnostic):** `router.go`, `middleware.go`,
  `security.go`, `throttle.go`, `errors.go`, `spa.go`, `health.go`, and the auth
  middleware + requester-context plumbing (`RequireAuth`, `getBearerToken`,
  `requesterFrom`, the context key). Plus `server/` (the http host).
- **`handlers` (app-specific endpoint adapters):** every `*Handler` func with its
  request/response DTOs, `LoginHandler` (split out of `auth.go`), and
  `OwnerExtractor` + `CharacterOwner`/`GroupOwner`/`LocationOwner` (split out of
  `inventory.go`).

`handlers` imports `transport` for the shared seams, which must be **exported**:
`requesterFrom`→`RequesterFrom`, `writeServiceError`→`WriteServiceError`, and
`clientIP`/`writeThrottled` (used by `LoginHandler`). `transport` imports nothing
from `handlers` — one-way dependency, no cycle. `components/router.go` then wires
`transport.NewRouter(...)` with `handlers.*` handler funcs.

## What moves OUT of `transport/`

The per-entity handlers are the real "api/handlers"; everything else is
mechanism. Split them:

| File(s) | Goes to | Why |
|---|---|---|
| `activity, auth, catalog, characters, documents, groups, inventory, journal_entries, locations, repositories, search, sessions, setup, users, vault_import` | `transport/handlers` | Per-entity request→service→response, plus their request/response DTOs. |
| `router.go`, `middleware.go`, `security.go`, `throttle.go`, `errors.go`, `spa.go`, `health.go` | stay in `transport` | Routing engine, `Middleware` type, cross-cutting middleware, status mapping, static delivery — the mechanism. |

Keeping router/middleware/errors in the parent `transport` package avoids an
import cycle: `handlers` imports `transport` for the `Middleware`/`Router`
types; the parent never imports `handlers` (the composition root wires them).

## What moves OUT of `application/`

Services stay. The non-services move to `domain/`:

| File | Goes to | Why |
|---|---|---|
| `gate.go` | `domain` (`access.go`) | Game-day visibility math — security-critical business rule, no I/O. |
| `sections.go` | `domain` (`access.go`) | Rule-7 GM-only section merge — business rule. |
| `frontmatter.go` | `domain/documentfmt` | YAML frontmatter parser — a format utility. |

Stays in `application`: all `*Service` files, `errors.go`
(`ErrorKind`/`ServiceError` vocabulary the services raise), `requester.go` (the
`Requester` port), `request_cache.go`, `doc.go`.

## Import-path rewrites (mechanical; `gofmt`/`goimports` after)

- `infrastructure/transport` → `transport` and `handlers`: every service/test
  that referenced a handler now points at `handlers`; mechanism references point
  at `transport`. `components/router.go` is the big one (references both).
- `infrastructure/servers` → `transport/server`: `cmd/serve.go`,
  `components/build.go`, `test/component/harness.go`.
- `application/{gate,sections,frontmatter}` symbols → `domain` /
  `domain/documentfmt`: the document, location, and inventory services.

Everything else in `infrastructure/` keeps its import path. `components/` and
`cmd/` churn most — they wire everything.

## Suggested sequence (each step compiles + tests green before the next)

1. `infrastructure/transport` → top-level `transport` (single package still);
   rewrite the import path. Self-contained.
2. Split `handlers` out of `transport`: export the shared seams, move the
   endpoint files, update `components/router.go` and the moved tests.
3. `infrastructure/servers` → `transport/server`.
4. Create `domain/`: `gate.go` + `sections.go` → `domain/access.go`,
   `frontmatter.go` → `domain/documentfmt`; export + qualify call sites.
5. Update `CLAUDE.md` (both), `api/CLAUDE.md` layer rules, and
   `docs/architecture.md`.

This order keeps the tree valid and tests passing at every step.

## Open notes

- **`request_cache.go`** — request-scoped caching over repositories. Kept in
  `application` (it reads request context, an application concern) rather than
  pushed into `database`. Revisit if it grows into a general repo-caching layer.
- **`auth` vs `platform`** — `auth` is kept top-level rather than folded into
  `platform` because application services depend on it as a capability, so its
  visibility is worth the extra top-level entry. Minor call; fold it in if the
  top level feels crowded.
- **`domain` naming** — it holds business *rules*, and depends on
  `database/models`. If a pure entity layer is ever wanted, that is a separate,
  larger change (introduce mapping between domain entities and GORM models).

## Not in scope (separate follow-up)

Handlers import `infrastructure/persistence/repositories` directly (~13 files),
bypassing the service layer. This refactor makes that leak *visible* but does
not fix it — routing those reads through `application` services is a
behaviour-level change and deserves its own pass.
```
