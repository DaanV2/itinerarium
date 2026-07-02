# Roadmap

Every item below maps to a feature in [features.md](features.md). A milestone is done when all its boxes are checked; a release phase (alpha, beta) ends only when all its milestones are done or the remaining items have been explicitly moved to the backlog.

## Alpha (M0–M4)

The alpha is done when a real campaign can run on it: accounts, characters, groups, knowledge with game-day gating, and sessions.

### M0 — Foundation

- [ ] Repo scaffold: `api/` (Go, mechanus layout) and `web/` (SvelteKit + TypeScript)
- [ ] Viper config: flags → env vars → YAML → defaults
- [ ] SQLite + GORM with shared `Model` base (UUID, timestamps, soft delete), `AutoMigrate`
- [ ] RS512 JWT auth with JTI revocation; keys auto-generated on first start
- [ ] Docker Compose for API + web
- [ ] First-run setup wizard creating the initial GM account
- [ ] Graceful shutdown lifecycle (mechanus pattern)

### M1 — Users, Characters & Locations

- [ ] GM creates accounts (player or GM rank) and hands out credentials
- [ ] GM resets passwords via temporary password from the admin panel (no SMTP)
- [ ] Login with email + password
- [ ] Multiple characters per user account
- [ ] Character `current_game_day` field
- [ ] Personal inventory (item list + quantity) per character
- [ ] Personal money (per currency) per character
- [ ] Currency catalog from JSON/YAML with conversion ratios
- [ ] Item catalog from JSON/YAML; free-text custom items always allowed
- [ ] Locations: name + description, multi-plane support
- [ ] Characters can be associated with a location

### M2 — Groups & Inventories

- [ ] Groups with `type` (`organization` / `family` / `other`) — identical mechanics
- [ ] Characters join and leave groups
- [ ] Join/leave events recorded with game-day stamp (surfaces in activity log in M5)
- [ ] Shared group inventory (item list + quantity)
- [ ] Shared group money (per currency)
- [ ] Location inventories
- [ ] Location inventory access control: single level (view + modify), granted per-character or via group; no access = existence hidden
- [ ] Item movement between character / group / location inventories

### M3 — Knowledge

- [ ] Repositories: general, template, one per group, one per character
- [ ] Folder tree per repository, alphabetical sorting
- [ ] Folders hidden when they contain no accessible documents
- [ ] Markdown documents with YAML frontmatter (`title`, `tags`, `repository`, `game_day`)
- [ ] GM-only sections, stripped server-side for players
- [ ] Section-boundary banners in the reader (GM/player sections clearly marked)
- [ ] Game-day gating: document visible when `current_game_day >= shared_on_game_day`
- [ ] Direct document shares to specific characters on a game day
- [ ] Sharing from a character repository to a group repository on a game day
- [ ] Open editing: anyone who can see a document can edit it
- [ ] Path-collision warning within a repository (rename or continue)
- [ ] Concurrent-edit warning before overwriting
- [ ] Player edit on an all-GM-only document creates a new player-visible section
- [ ] Editor shows reveal settings ("Revealed at game day X to …")
- [ ] Editor warning banner when editing an already-revealed document
- [ ] Journals: per-character entries stamped with game day, readable by owner + GM only
- [ ] Journal page → document conversion (copy into character repository)
- [ ] Location description documents (same visibility/game-day rules)
- [ ] Locations editable by anyone who can see them

### M4 — Sessions & Game Day

- [ ] Sessions with character participants
- [ ] GM advances/rewinds game day for all session participants at once
- [ ] GM advances/rewinds game day per individual character (catch-up)
- [ ] Visibility recalculates correctly after rewind (documents/entries disappear again)

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

- [ ] FTS5 full-text search over titles, file names, tags, and content
- [ ] Access filtering applied before results are returned (no leaks, no hit counts)
- [ ] GM-only sections excluded from the player search index
- [ ] GMs search across everything
- [ ] Obsidian vault import: folders map to repository paths, frontmatter parsed
- [ ] Import path-collision handling (warn, rename or continue)
