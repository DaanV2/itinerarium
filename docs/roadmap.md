# Roadmap

Every item below maps to a feature in [features.md](features.md) or to a concrete code change in the repo. A milestone is done when all its boxes are checked; a phase ends only when all its milestones are done or the remaining items have been explicitly moved to the [backlog](backlog.md).

**Where we are:** the feature milestones **M0–M6** (alpha + beta) all shipped — a real campaign can run on Itinerarium end to end. The current phase is **Hardening & Sustainability (M7–M11)**: no new user-facing features, focused on maintainability, performance, security, and upgrade durability so the codebase stays cheap to change. New feature ideas live in [backlog.md](backlog.md) and are not scheduled until this phase closes.

## Guiding constraints for this phase

These are refactors, not rewrites. Every item here must:

- **Preserve behavior.** Except the security-hardening milestone (M10), no change should alter an API response a client can observe. The security invariants in the root [CLAUDE.md](../CLAUDE.md) (game-day gating, repository access, hidden-means-404, GM-only stripping, rewind) are non-negotiable — a refactor that weakens one is a bug, not a simplification.
- **Keep the tests green and the negative tests intact.** The negative tests in `development.md` are the safety net that lets us move code confidently — never delete one to make a refactor pass; if a refactor makes one fail, the refactor is wrong.
- **Ship one milestone item per branch.** Don't bundle a DRY cleanup with a behavior change; a reviewer should be able to confirm "no behavior change" at a glance.
- **Not add a dependency without a reason a reviewer would accept** (root `CLAUDE.md`).

---

## Shipped — Alpha & Beta (M0–M6)

Feature-complete for 1.0. Summary; the full checked list is in the git history (see the `M0`–`M6` merge commits) and every capability is described in [features.md](features.md) and [architecture.md](architecture.md).

- **M0 — Foundation:** repo scaffold, Viper config, SQLite + GORM base model, RS512 JWT auth with JTI revocation, single-binary embedded-SPA Docker deploy, first-run setup wizard + `init` CLI, graceful shutdown, CI.
- **M1 — Users, Characters & Locations:** GM-managed accounts, per-character game day, personal inventory/money, currency + item catalogs, currency calculator, locations with planes.
- **M2 — Groups & Inventories:** unified group model, membership with game-day-stamped activity, shared group inventory/money, location inventories with access control, item movement.
- **M3 — Knowledge:** repositories + folder tree, markdown documents with GM-only sections, game-day gating, direct + group shares, open editing with path-collision and concurrent-edit warnings, journals and journal→document conversion, location-description documents.
- **M4 — Sessions & Game Day:** sessions with participants, per-session and per-character game-day advance/rewind, visibility recalculated on rewind.
- **M5 — Activity Log & Announcements:** append-only game-day-stamped activity feed, scope-gated visibility, announcements that bypass entity access but strip actor for players.
- **M6 — Search & Obsidian Import:** access-filtered full-text `LIKE` search, GM-only content excluded for players, Obsidian vault import with per-file collision handling.

---

## Hardening & Sustainability (M7–M11)

### M7 — Backend maintainability & DRY

The service and transport layers are clean but carry a few duplicated shapes worth collapsing before they multiply.

- [x] **One service-error → HTTP mapper.** 14 near-identical `write<Entity>ServiceError` functions in `infrastructure/transport/` each hand-map the same sentinel → status pattern. Give service sentinel errors an associated HTTP status (and optional machine code, as `path_collision` / `concurrent_edit` already do) and collapse to a single `writeServiceError(w, err)`. Per-entity validation errors map to `400` through the same mechanism.
- [x] **Extract a shared game-day visibility gate.** `DocumentService` and `LocationService` (and partly `search.go`) each re-implement "the furthest-along character of the requester that can reach this thing" and "has any character with access reached the reveal day" against different access sources (repository type vs. location grants). Extract one gate abstraction parameterised by the access-source lookup, the way `sections.go`'s generic `mergeVisibleSections` already unifies the section-merge rule across documents and locations. This is the highest-leverage item: it is exactly the security-critical logic, so having it in one audited place is a security win as well as a DRY one.
- [x] **Split `application/documents.go` (≈1040 lines).** One file owns read, create, update, share-to-group, delete, direct shares, folder-tree building, gating, and section merging. Split along those seams (e.g. `documents.go`, `documents_sharing.go`, `documents_gating.go`) with no logic change.
- [x] **Comment/structure sweep.** Fix the detached `writeDocumentServiceError` doc comment (it currently sits above `DeleteDocumentHandler` in `transport/documents.go`) and grep for other doc comments that drifted off their function during past edits.
- [x] **Guard the composition root and CLI.** `components/` (config→db→auth→repos→services→router wiring) and `cmd/` (serve/init) have no tests, so a wiring regression only surfaces at runtime — the exact code the M7–M8 refactors move things through. Add a smoke test that `BuildServer(ctx)` wires a working server against an in-memory DB, and a test for `init` bootstrapping the first GM (and refusing once an account exists). This is the structural counterpart to the DRY work: the refactors need a net under the wiring layer, not just the service layer.

### M8 — Backend performance

The gating logic is correct but re-queries more than it needs to inside a single request.

- [x] **Load the requester's characters once per request.** A single document read calls `characters.ListByUser` up to four times (`getAccessible` → `getViaDirectShare`, `view`, `effectiveGameDay`, `documentEntry`). Resolve the requester's characters (and their group memberships) once and thread that context through the gate from M7. Done via a per-request cache (`application/request_cache.go`) installed by `RequireAuth` and read by the gating helpers (`requesterCharacters`, `cachedGroupIDsForCharacters`, `cachedGroup`); absent the cache the helpers fall through to the database, so behavior is identical either way.
- [x] **Remove the N+1 in `ListSharedWithMe`.** It issues a `GetByID` document load and a `GetUnchecked` repository load per share in a loop — batch-load the documents and repositories by id set instead. Now two batch loads (`Documents.ListByIDs`, `RepositoryService.GetManyUnchecked`) regardless of share count.
- [x] **Audit every list endpoint for N+1** (documents, activity feed, search already precomputes `dayByRepo` — use it as the model). Add a lightweight per-request access cache if the gate needs one. All `ListByUser` / `GroupIDsForCharacters` / `groups.GetByID` gating lookups now route through the per-request cache, which also collapses the per-group-repo group loads inside `search.go`'s `searchScope`.
- [x] Add a couple of representative benchmarks or query-count assertions so a future regression is caught, not re-discovered. See `application/performance_test.go`: one asserts the requester's characters load exactly once per document update, the other that `ListSharedWithMe`'s query count is constant in the number of shared documents.

### M9 — Frontend maintainability

The API layer and route pages carry the mirror image of the backend's duplication.

- [ ] **A single API client.** `errorMessage`/`errorBody` are copy-pasted into 12 files under `lib/api/`, and the bearer-header + `fetchFn` plumbing repeats ~55 times. Extract `lib/api/client.ts` — one `apiFetch` that injects the token, parses the `{error, code}` body, and throws a typed `ApiError` (with `status` + `code`) that `DocumentConflictError` and friends extend. Each endpoint wrapper becomes a few lines.
- [ ] **Thin out the large route pages.** `routes/locations/[id]`, `routes/documents/[id]`, and `routes/repositories/import` (≈340–360 lines each) mix fetch orchestration, editor state, and markup. Extend the pattern the repo already uses well (`activity-view.ts`, `inventory-view.ts`, `document-reveal.ts`): pull the logic into tested `.ts` modules so the `.svelte` file is mostly template.
- [ ] **Delete scaffolding.** Remove `lib/vitest-examples/` (the `greet` placeholder) now that real specs exist.

### M10 — Security hardening

Itinerarium is self-hosted and may be exposed to the internet. The invariants are enforced; the surface around them is not yet hardened. This is the one milestone that intentionally changes observable behavior.

- [ ] **Login/reset rate limiting.** Add per-account and per-IP throttling + backoff on `/api/login` and the GM password-reset path; no unauthenticated endpoint should allow unbounded attempts. _(Deferred — the reverse proxy in front of a self-hosted deployment is the intended place for this; revisit if an in-process limiter is wanted.)_
- [x] **Request body size limits.** `transport.MaxBytes` middleware wraps every request body in `http.MaxBytesReader` (default 10 MiB, `security.body-limit`) so a single request can't exhaust memory.
- [x] **Security-header middleware.** `transport.SecurityHeaders` sets `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, a minimal CSP for the embedded SPA (`security.csp`), and HSTS when served behind TLS (`security.hsts`, or any TLS request) — alongside the existing `Logging` middleware.
- [x] **Fresh invariant review.** The full negative-test suite (the checklist in `development.md`) still passes against the M7 gate; added tests cover the new middleware (`transport/security_test.go`). The `security.*` surface is opt-out-able but on by default.
- [x] **Repo-hygiene for a security-shaped product.** Added [`SECURITY.md`](../SECURITY.md) with a GitHub private-advisory disclosure path and operator hardening notes, and an [`.editorconfig`](../.editorconfig) (LF, tabs for Go/frontend, 2-space YAML) to blunt the CRLF/`autocrlf` friction. `CONTRIBUTING.md` / `CODEOWNERS` left out — `docs/development.md` + `docs/onboarding.md` cover the contributor story.

### M11 — Schema & contract durability

Two long-term maintenance risks for a product users upgrade in place.

- [ ] **Versioned migrations.** Schema evolution currently rides on `AutoMigrate` plus an ad-hoc `backfillActivityScopes` in `migrations.go`. `AutoMigrate` never drops or renames columns and can't express data backfills cleanly, so in-place upgrades drift. Adopt an explicit ordered migration list (e.g. `gormigrate`), keep `AutoMigrate` only as the dev/fresh-install fast path, and fold the existing backfill into a numbered migration. Decide and document the strategy in `architecture.md` / `deployment.md`.
- [ ] **An API contract to stop drift.** The ~50-route API is described only in prose in `architecture.md`, and the TypeScript types in `web/src/lib/types.ts` are hand-mirrored from the Go handlers. Publish an OpenAPI description (generated or maintained) and a check that the client types match it, so a handler change that outruns the client is caught in CI.

---

## Exit criteria

This phase is done — and the codebase is ready for post-1.0 feature work from [backlog.md](backlog.md) — when every box above is checked or explicitly deferred to the backlog, `just verify` is green, and the security invariants still hold with their negative tests intact.
