# Architecture

## Overview

```
itinerarium/
├── api/          # Go API server (GORM + SQLite)
├── web/          # SvelteKit frontend
├── config/       # YAML/env configuration
└── docker-compose.yml
```

| Layer      | Technology             |
| ---------- | ---------------------- |
| API        | Go, GORM, SQLite       |
| Frontend   | SvelteKit + TypeScript |
| Deployment | Docker Compose         |

## Core Concept: Game Day

`game_day` is an integer counter per character — not a real-world date. It drives all document visibility: a character only sees a document once their `current_game_day` reaches the document's `shared_on_game_day`.

## Key Entities

| Entity         | Description                                                                                                                         |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `User`         | Player or GM role. GMs create player accounts directly.                                                                             |
| `Character`    | Belongs to a User. Has its own `current_game_day`, optional `Location`, personal inventory, and money.                              |
| `Group`        | Unified model for organisations/families/other. `type` is cosmetic. Shares inventory, money, and documents among member characters. Join/leave events are recorded as `ActivityEntry` rows stamped with the game day. |
| `Repository`   | A named vault with its own folder tree. Types: `general` (everyone), `group` (one per group, members only), `template` (everyone), `character` (owner + GM only). The `general` and `template` singletons and one repository per group/character are provisioned automatically — never created directly by a caller (M3). |
| `Document`     | Markdown content with a folder `path` inside exactly one `Repository`. Has sections with a `gm_only` flag. Can additionally be shared directly to specific characters on a game day. |
| `Currency`     | GM-defined via JSON/YAML list, with conversion ratios to a base unit. Shared by all inventories. |
| `ItemDefinition` | Entry in the GM's default item catalog (JSON/YAML). Inventory items may reference a definition or be free-text — the catalog never restricts. |
| `InventoryItem` | A line in an inventory: name + quantity, optionally referencing an `ItemDefinition`. Owned by exactly one character, group, or location (embedded `InventoryOwner`); visibility follows the owner. |
| `MoneyBalance` | A character's or group's holding of a single `Currency`: `amount` in that currency's own unit, one per (owner, currency). Locations hold items, not money. |
| `LocationAccess` | A GM-managed grant giving one character or one group a location's single access level (view + modify, including its inventory). No grant = the location's existence is hidden. |
| `Session`      | Links characters to a play event via a `many2many` participant list. Carries no game day of its own — advancing/rewinding a session moves each participant's own `Character.CurrentGameDay`, either for everyone at once or for one character catching up. GM-only: creation, editing, participant management, and game-day advances. |
| `JournalEntry` | Belongs to a character, stamped with `game_day`. Readable by the owning player and GMs only. Can be converted (copied) into a `Document` in the character's private knowledge repository. |
| `ActivityEntry` | Append-only event log. Stamped with `game_day`. Scoped to an entity (group, location, document). M2 records group join/leave events; M5 adds the per-character feed and an `announced` flag with explicit target characters or groups that bypasses normal entity-access rules (used for theft, destruction, and GM broadcasts). |
| `Location`     | Named plane or place (town, building, room, …). `plane` is a free-text label grouping locations into planes of existence. Has its own inventory and access-controlled visibility via `LocationAccess`. Characters and sessions can be associated with one. |

## Permission Model

A character can read a document when **both** hold:

1. `character.current_game_day >= document.shared_on_game_day`
2. One of:
   - The document's repository is `general` or `template`, **or**
   - The document's repository belongs to a group the character is a member of, **or**
   - The document's repository is the character's own, **or**
   - The document is directly shared to the character

Documents are not versioned: game day gates visibility of the document, not its revision. Everyone who can see a document sees the latest content.

Sharing a document from a character's private repository means moving it to another repository (or direct-sharing it) with a `shared_on_game_day`, at which point normal rules apply.

Journal-to-document conversion is a **copy**: the new document lands in the character's own repository; the original journal entry is unchanged.

Any character who can see a document can edit it. Two collision warnings exist:
- **Path collision** — creating/importing a document at a path that already exists in the same repository warns the user (rename or continue)
- **Concurrent edit** — if the document changed since the editor loaded it, the save warns before overwriting

Players can never edit or see GM-only sections; a player editing a document whose visible content is empty (all sections GM-only) creates a new player-visible section alongside the GM content.

Locations follow the same edit rule as documents: any character who can see a location can edit its description and details.

GM-only document sections are stripped **server-side** before the response is sent — never rely on the client to hide them.

Location inventories apply the same access-control check: if a character lacks access to a location, the inventory (and its existence) must not appear in any API response.

### Groups and locations (M2)

- **Groups are campaign structure, their content is not.** Any authenticated user may list groups and see member identity (id + name only — never a member's game day or owning account); only a GM creates or edits a group. The group's *content* — shared inventory, shared money, and (from M3) its repository — is member-only: a non-member gets `404`, never a `403`.
- **Membership changes** are allowed to the character's owner and to GMs. Every join/leave writes an `ActivityEntry` (`joined`/`left`) stamped with the character's `current_game_day`, in the same transaction as the membership change, so history and membership can never drift apart.
- **Location visibility is grant-gated.** GMs see every location; a player sees one only through a `LocationAccess` grant held by one of their characters, directly or via a group that character belongs to. One grant is the single access level: view + modify, location fields and inventory alike. Grants are GM-managed; players never see the grant list.
- **Character ↔ location association**: the owner or a GM sets it. A player may only place a character at a location *that character* can see — an inaccessible location reads as `404` so its existence never leaks. GMs place anyone anywhere.

### Repositories (M3)

- **Provisioning is automatic, not user-driven.** The `general` and `template` repositories are singletons created once at startup (`RepositoryService.EnsureSystemRepositories`, idempotent). A group's repository is created in `GroupService.Create`; a character's repository is created in `CharacterService.Create`. There is no create endpoint — repositories only ever come from these paths, so "one per group, one per character" can never drift.
- **Visibility mirrors the entity it belongs to.** `general`/`template` are visible to everyone; a `group` repository follows that group's membership; a `character` repository follows character ownership. GMs see every repository. A caller without access gets `404`, never `403` — same existence-hiding rule as locations.

### Documents (M3)

- **Content model.** A `Document` holds metadata (path, title, tags, `shared_on_game_day`) plus ordered `DocumentSection` rows, each with a `gm_only` flag. GM-only sections are stripped in the service layer before any non-GM response is built — a player payload never contains them, not even as empty placeholders.
- **A player's game day for a repository** is the highest `current_game_day` among *their* characters that the repository's own rule grants access (owner for character repos, members for group repos, all of their characters for general/template). No qualifying character means no documents — the repository looks empty and direct reads are `404`.
- **Anyone who can see a repository can create documents in it**, and anyone who can see a document can edit it. Only GMs can mark sections GM-only or change `shared_on_game_day` after creation.
- **Player edits merge, never clobber**: a player's save replaces only the visible sections (by section ID); GM-only rows keep their position untouched. New sections land player-visible at the end — so an edit on an all-GM-only document becomes a new player-visible section (core domain rule 7). A player submitting a GM-only section ID gets the same "unknown section" error as a garbage ID, so GM-ness never leaks.
- **Warnings, not blocks** (both `409` with a machine-readable `code`): creating/moving onto an occupied path returns `path_collision` unless `allow_collision` is set; saving with a stale `expected_version` returns `concurrent_edit` unless `force` is set. `version` is an integer that increments on every save — editors echo it back.
- **Reveal state for the editor**: document responses carry `revealed` — whether any character with repository access has reached `shared_on_game_day` — so the editor can warn that edits to an already-revealed document are immediately visible (documents are not versioned).
- **Frontmatter**: `POST … /documents` accepts raw markdown whose leading `---` YAML block sets `title`, `tags`, and `game_day` (explicit request fields win). This is the same Obsidian-compatible format the M6 vault import will use.

### Item movement (M2)

`POST /api/inventory/move` transfers `quantity` units of an inventory line into another inventory. The caller needs access to **both** ends: no source access means the item itself reads as `404`; no target access means the target does. Moving the full quantity re-owns the line; a partial quantity splits it; if the target already holds a line with the same name and catalog reference, the moved units merge into it. The whole move runs in one database transaction.

Activity entries have two visibility paths:
1. **Normal** — character has access to the source entity AND `current_game_day >= entry.game_day`
2. **Announced** — entry has `announced: true` and the character (or one of their groups) is in the `announced_to` list; entity-access check is skipped. The `actor` field is stripped server-side for non-GM users — players see what happened and to what, but not who did it.

## Inventory & Currency

> For a focused walkthrough see [currency.md](currency.md) (concepts, ratios, money, API) and [currency-configuration.md](currency-configuration.md) (defining the catalog).

The GM defines two campaign-wide catalogs, both readable by any authenticated user and writable only by a GM:

- **`Currency`** — `code` (unique), `name`, and an integer `ratio` giving the value of one unit in the campaign's **base unit** (the smallest denomination, which itself has `ratio` 1). For "1 gold = 10 silver = 100 copper", copper is the base (`ratio` 1), silver `ratio` 10, gold `ratio` 100. Storing an integer ratio keeps all money arithmetic in whole base units, avoiding floating-point rounding.
- **`ItemDefinition`** — `name` (unique), optional `description` and `category`. A convenience for picking known items; it never restricts inventories (core domain rule 8).

Both catalogs can be seeded from a JSON/YAML file at startup via `--catalog-path` (env `SERVER_CATALOG_PATH`). The loader **upserts** — currencies by `code`, items by `name` — so restarting with an edited file updates entries in place instead of duplicating them. See `config/catalog.example.yaml`.

Since M2, inventories are **owner-based** — a line belongs to exactly one character, group, or location:

- **`InventoryItem`** — a line in an inventory: one owner id (`character_id` / `group_id` / `location_id`), `name` (required), optional `item_definition_id` (a catalog reference; omitting it makes the line a free-text item), `quantity` (≥ 1), optional `description`.
- **`MoneyBalance`** — a character's or group's holding of one currency: `character_id` *or* `group_id`, `currency_id`, `amount` (≥ 0). At most one balance per (owner, currency), enforced by composite unique indexes; `SetMoney` upserts it. Locations hold items, not money.

**Permission rule.** Access is a single level — whoever can view an inventory can modify it — and resolves through the owning entity's own visibility rule: character inventories are **owner + GM only**, group inventories/money are **members + GM** (via any of the caller's characters), location inventories follow the location's **access grants**. A caller without access gets `404` (existence hidden, never `403`), and a line addressed through the wrong owner's path is likewise `404`.

### Endpoints

| Method & path | Who | Purpose |
|---|---|---|
| `GET /api/currencies` | any authenticated | List the currency catalog |
| `POST /api/currencies` | GM | Add a currency |
| `POST /api/currencies/convert` | any authenticated | Add up currency amounts and express the total in a target currency (stateless — no balance read/write) |
| `POST /api/currencies/simplify` | any authenticated | Add up currency amounts and break the total into the fewest coins across the catalog (stateless) |
| `GET /api/items` | any authenticated | List the item catalog |
| `POST /api/items` | GM | Add an item definition |
| `GET\|POST /api/<owner>/{id}/inventory` | per owner rule | List / add inventory lines (`<owner>` = `characters`, `groups`, or `locations`) |
| `PATCH\|DELETE /api/<owner>/{id}/inventory/{itemId}` | per owner rule | Edit / remove a line |
| `POST /api/inventory/move` | access to both ends | Move item quantity between inventories |
| `GET /api/characters/{id}/money` | owner + GM | List a character's balances |
| `PUT /api/characters/{id}/money/{currencyId}` | owner + GM | Set a balance to an absolute amount |
| `GET\|PUT /api/groups/{id}/money…` | members + GM | Same, for a group's shared money |
| `GET /api/groups` | any authenticated | List groups with member identity |
| `POST /api/groups`, `PATCH /api/groups/{id}` | GM | Create / edit a group |
| `POST /api/groups/{id}/members` | character owner + GM | Join a character to a group |
| `DELETE /api/groups/{id}/members/{characterId}` | character owner + GM | Leave a group |
| `GET /api/locations` | GM all, players accessible only | List visible locations |
| `POST /api/locations` | GM | Create a location |
| `GET\|PATCH /api/locations/{id}` | anyone with access | Read / edit a location (404 without access) |
| `GET\|POST /api/locations/{id}/access`, `DELETE …/access/{accessId}` | GM | Manage a location's grants |
| `PUT\|DELETE /api/characters/{id}/location` | owner + GM | Set / clear a character's location (players only to locations the character can see) |
| `GET /api/repositories` | any authenticated | List visible repositories: general, template, plus own character/group repositories (GM sees all) |
| `GET /api/repositories/{id}` | per repository rule | Read one repository (404 without access) |
| `GET /api/repositories/{id}/documents` | per repository rule | List the repository's documents the caller may see (game-day gated for players; no sections) |
| `POST /api/repositories/{id}/documents` | anyone who sees the repository | Create a document (structured `sections`, or raw `markdown` with optional YAML frontmatter) |
| `GET /api/documents/{id}` | per document rule | Read one document; GM-only sections are stripped server-side for players (404 without access) |
| `PATCH /api/documents/{id}` | anyone who sees the document | Replace metadata + the caller's visible sections (players can never touch GM-only sections or the reveal day) |
| `GET\|POST /api/characters/{id}/journal` | owner + GM | List / add a character's journal entries. New entries are stamped with the character's current `current_game_day` |
| `GET\|PATCH /api/characters/{id}/journal/{entryId}` | owner + GM | Read / edit a journal entry's content (404 without access; game day never changes after creation) |
| `GET\|POST /api/sessions` | GM | List / create sessions |
| `GET\|PATCH /api/sessions/{id}` | GM | Read / edit a session |
| `POST /api/sessions/{id}/participants` | GM | Add a character to a session |
| `DELETE /api/sessions/{id}/participants/{characterId}` | GM | Remove a character from a session |
| `POST /api/sessions/{id}/game-day` | GM | Advance/rewind `game_day` for every participant (or one, via `character_id`) by a signed `delta` |

## Document Format

Documents are stored as Markdown with optional YAML frontmatter:

```markdown
---
title: The Thieves Guild
tags: [faction, city]
visibility: group
game_day: 12
---

Full markdown content here...
```

Supported frontmatter keys:

| Key | Description |
|-----|-------------|
| `title` | Display name (falls back to filename) |
| `tags` | Free-form labels for filtering |
| `repository` | Target repository on import (`general`, `template`, a group name, or a character name) |
| `game_day` | The `shared_on_game_day` value |

This format is intentionally compatible with Obsidian so GMs can author documents in their Obsidian vault and import/sync `.md` files directly. The folder structure of the vault maps to the document `path`, so the tree in Itinerarium mirrors the vault layout.

## Search

Full-text search over titles, file names, tags, and content (search backend TBD). Access rules are applied **before** results are returned:

- Documents the character cannot see (wrong group, game day not reached) are excluded entirely — no titles, no hit counts
- GM-only sections are excluded from the searchable content for non-GM users
- Folder visibility follows the same rule: a folder appears only if it contains at least one accessible document
