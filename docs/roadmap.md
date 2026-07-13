# Roadmap

Every item below maps to a feature in [features.md](features.md). A milestone is done when all its boxes are checked; a release phase (alpha, beta) ends only when all its milestones are done or the remaining items have been explicitly moved to the backlog.

## Alpha (M0–M4)

The alpha is done when a real campaign can run on it: accounts, characters, groups, knowledge with game-day gating, and sessions.

### M0 — Foundation

- [x] Repo scaffold: `api/` (Go, mechanus layout) and `web/` (SvelteKit + TypeScript)
- [x] Viper config: flags → env vars → YAML → defaults
- [x] SQLite + GORM with shared `Model` base (UUID, timestamps, soft delete), `AutoMigrate`
- [x] RS512 JWT auth with JTI revocation; keys auto-generated on first start
- [x] Docker Compose for API + web *(verified end-to-end without Docker itself, still unavailable on the dev machine: built the API binary and the SvelteKit adapter-node bundle and ran them side by side, exercising the same `node build` entrypoint the web Dockerfile uses)*
- [x] First-run setup wizard creating the initial GM account (web wizard + `itinerarium init` CLI for headless deployments)
- [x] Graceful shutdown lifecycle (mechanus pattern)
- [x] Create Github Workflows, with testing included

### M1 — Users, Characters & Locations

- [x] GM creates accounts (player or GM rank) and hands out credentials
- [x] GM resets passwords via temporary password from the admin panel (no SMTP)
- [x] Login with email + password
- [x] Multiple characters per user account
- [x] Character `current_game_day` field
- [x] Personal inventory (item list + quantity) per character
- [x] Personal money (per currency) per character
- [x] Currency catalog from JSON/YAML with conversion ratios
- [x] Item catalog from JSON/YAML; free-text custom items always allowed
- [x] Locations: name + description, multi-plane support
- [x] Characters can be associated with a location

### M2 — Groups & Inventories

- [x] Groups with `type` (`organization` / `family` / `other`) — identical mechanics
- [x] Characters join and leave groups
- [x] Join/leave events recorded with game-day stamp (surfaces in activity log in M5)
- [x] Shared group inventory (item list + quantity)
- [x] Shared group money (per currency)
- [x] Location inventories
- [x] Location inventory access control: single level (view + modify), granted per-character or via group; no access = existence hidden
- [x] Item movement between character / group / location inventories

### M3 — Knowledge

- [x] Repositories: general, template, one per group, one per character
- [ ] Folder tree per repository, alphabetical sorting
- [ ] Folders hidden when they contain no accessible documents
- [x] Markdown documents with YAML frontmatter (`title`, `tags`, `repository`, `game_day`)
- [x] GM-only sections, stripped server-side for players
- [ ] Section-boundary banners in the reader (GM/player sections clearly marked)
- [x] Game-day gating: document visible when `current_game_day >= shared_on_game_day`
- [ ] Direct document shares to specific characters on a game day
- [ ] Sharing from a character repository to a group repository on a game day
- [x] Open editing: anyone who can see a document can edit it
- [ ] Path-collision warning within a repository (rename or continue) *(API warns with 409 `path_collision`; editor dialog pending)*
- [ ] Concurrent-edit warning before overwriting *(API warns with 409 `concurrent_edit` on stale `version`; editor dialog pending)*
- [x] Player edit on an all-GM-only document creates a new player-visible section
- [ ] Editor shows reveal settings ("Revealed at game day X to …")
- [ ] Editor warning banner when editing an already-revealed document
- [x] Journals: per-character entries stamped with game day, readable by owner + GM only
- [ ] Journal page → document conversion (copy into character repository)
- [ ] Location description documents (same visibility/game-day rules)
- [ ] Locations editable by anyone who can see them

### M4 — Sessions & Game Day

- [x] Sessions with character participants
- [x] GM advances/rewinds game day for all session participants at once
- [x] GM advances/rewinds game day per individual character (catch-up)
- [ ] Visibility recalculates correctly after rewind (documents/entries disappear again) *(depends on M3 knowledge gating, not yet built)*

## Beta (M5–M6)

The beta phase ends — and the product is 1.0 — only when every box below is checked or explicitly moved to [backlog.md](backlog.md).

### M5 — Activity Log & Announcements

- [ ] Append-only `ActivityEntry` model stamped with game day
- [ ] Per-character activity feed, gated by `current_game_day` and entity access
- [ ] GMs see all activity regardless of game day
- [ ] Tracked: group membership (joined / left)
- [ ] Tracked: group inventory (item added / quantity changed / removed)
- [ ] Tracked: location inventory (same, only visible with location access)
- [ ] Tracked: documents (added / updated / removed)
- [ ] Tracked: group money (balance changed)
- [ ] Actions include `destroyed` and `stolen`
- [ ] Announcements: GM targets specific characters, a group, or public, surfacing at a chosen game day
- [ ] Announced entries bypass entity access but never reveal entity content
- [ ] `actor` field stripped server-side for players on announced entries

### M6 — Search & Obsidian Import

- [ ] Full-text search over titles, file names, tags, and content
- [ ] Access filtering applied before results are returned (no leaks, no hit counts)
- [ ] GM-only sections excluded from the player search index
- [ ] GMs search across everything
- [ ] Obsidian vault import: folders map to repository paths, frontmatter parsed
- [ ] Import path-collision handling (warn, rename or continue)
