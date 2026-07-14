# Development Workflow

How to pick up and finish a feature. Written so any contributor (human or agent) can follow it without extra context.

## Read this first

1. Root `CLAUDE.md` — stack, conventions, the 8 core domain rules
2. `api/CLAUDE.md` or `web/CLAUDE.md` — layer rules and copyable code templates for the side you're touching
3. [architecture.md](architecture.md) — entities and the permission model (**source of truth**)
4. [roadmap.md](roadmap.md) — the feature list; work top-to-bottom within a milestone unless told otherwise

## Picking a feature

- Take **one roadmap checkbox** (or a tightly related cluster) per branch/session. Don't bundle milestones.
- Respect milestone order: don't build M3 knowledge features on models that M1 hasn't created yet. If a dependency is missing, build the minimal version of the dependency first and note it.
- If a feature seems to conflict with [architecture.md](architecture.md), architecture.md wins; if it's genuinely ambiguous, stop and ask rather than guessing.

## Feature workflow

1. **Locate the rules.** Find the feature in [features.md](features.md) and any matching core domain rule in the root `CLAUDE.md`. Those rules are requirements, not suggestions.
2. **API first.** Model → migration registration → repository → service → route → tests, exactly as laid out in `api/CLAUDE.md`. Get `go test ./...` green before touching the frontend.
3. **Frontend second.** Typed API wrapper in `web/src/lib/api/`, then route/components per `web/CLAUDE.md`.
4. **Verify like CI does**:
   ```bash
   just verify   # from the repo root — runs both blocks below
   ```
   ```bash
   cd api && go build ./... && go vet ./... && golangci-lint run ./... && go test ./...
   cd web && npm run lint && npm run check && npm run test && npm run build
   ```
   (CI also adds `-race` to `go test` — that needs cgo and Linux, so don't worry if it fails locally on Windows.)
5. **Close the loop.** Tick the checkbox in [roadmap.md](roadmap.md). If you changed entities or permission rules, update [architecture.md](architecture.md) in the same change.

## Definition of done

- [ ] All CI commands above pass locally
- [ ] Every new permission rule has a **negative test** (the wrong character gets 404/empty — never a leak, never a 403 that confirms existence)
- [ ] GM-only data is stripped **server-side**; the client never receives it
- [ ] New models are registered in `api/infrastructure/persistence/migrations.go`
- [ ] Roadmap checkbox ticked; architecture.md updated if the domain changed
- [ ] No new dependency added without a reason a reviewer would accept

## Security invariants — test these, always

Any feature touching documents, inventories, locations, journals, activity entries, or search must prove in tests:

| Invariant | Negative test to write |
|-----------|------------------------|
| Game-day gating | Character with `current_game_day < shared_on_game_day` gets nothing |
| Repository access | Non-member sees no group docs; non-owner sees no character docs |
| Hidden means invisible | No-access entity is absent from lists, search results, and counts — and direct GET returns 404, not 403 |
| GM-only stripping | Player response contains no `gm_only` section content and no `actor` on announced entries |
| Rewind | After game day is rewound, previously visible items disappear again |

## Environment quick reference

All recipes live in the root `justfile` (`just` lists them).

| Task | Command |
|------|---------|
| Run API | `just api` |
| Run frontend (proxies /api to :8080) | `just web` |
| All CI checks | `just verify` |
| Format everything | `just fmt` |
| Full stack | `just up` |
| API config | flags → env (`SERVER_ADDRESS`, `DATABASE_PATH`) → `--config file.yaml` → defaults; example in `config/config.example.yaml` |

The SQLite driver is pure Go (`glebarez/sqlite`) — no cgo, no gcc needed, works on any machine. Don't replace it.
