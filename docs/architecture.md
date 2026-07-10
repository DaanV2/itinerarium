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
| `Repository`   | A named vault with its own folder tree. Types: `general` (everyone), `group` (one per group, members only), `template` (everyone), `character` (owner + GM only). |
| `Document`     | Markdown content with a folder `path` inside exactly one `Repository`. Has sections with a `gm_only` flag. Can additionally be shared directly to specific characters on a game day. |
| `Currency`     | GM-defined via JSON/YAML list, with conversion ratios to a base unit. Shared by all inventories. |
| `ItemDefinition` | Entry in the GM's default item catalog (JSON/YAML). Inventory items may reference a definition or be free-text — the catalog never restricts. |
| `InventoryItem` | A line in a character's personal inventory (M1): name + quantity, optionally referencing an `ItemDefinition`. Owner + GM only. M2 extends inventories to groups and locations. |
| `MoneyBalance` | A character's holding of a single `Currency` (M1): `amount` in that currency's own unit, one per (character, currency). Owner + GM only. |
| `Session`      | Links characters to a play event. GM advances/rewinds `game_day` per character or in bulk.                                          |
| `JournalEntry` | Belongs to a character, stamped with `game_day`. Readable by the owning player and GMs only. Can be converted (copied) into a `Document` in the character's private knowledge repository. |
| `ActivityEntry` | Append-only event log. Stamped with `game_day`. Scoped to an entity (group, location, document). Supports an `announced` flag with explicit target characters or groups that bypasses normal entity-access rules (used for theft, destruction, and GM broadcasts). |
| `Location`     | Named plane or place (town, building, room, …). Has its own inventory and access-controlled visibility. Characters and sessions can be associated with one. |

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

Activity entries have two visibility paths:
1. **Normal** — character has access to the source entity AND `current_game_day >= entry.game_day`
2. **Announced** — entry has `announced: true` and the character (or one of their groups) is in the `announced_to` list; entity-access check is skipped. The `actor` field is stripped server-side for non-GM users — players see what happened and to what, but not who did it.

## Inventory & Currency

> For a focused walkthrough see [currency.md](currency.md) (concepts, ratios, money, API) and [currency-configuration.md](currency-configuration.md) (defining the catalog).

The GM defines two campaign-wide catalogs, both readable by any authenticated user and writable only by a GM:

- **`Currency`** — `code` (unique), `name`, and an integer `ratio` giving the value of one unit in the campaign's **base unit** (the smallest denomination, which itself has `ratio` 1). For "1 gold = 10 silver = 100 copper", copper is the base (`ratio` 1), silver `ratio` 10, gold `ratio` 100. Storing an integer ratio keeps all money arithmetic in whole base units, avoiding floating-point rounding.
- **`ItemDefinition`** — `name` (unique), optional `description` and `category`. A convenience for picking known items; it never restricts inventories (core domain rule 8).

Both catalogs can be seeded from a JSON/YAML file at startup via `--catalog-path` (env `SERVER_CATALOG_PATH`). The loader **upserts** — currencies by `code`, items by `name` — so restarting with an edited file updates entries in place instead of duplicating them. See `config/catalog.example.yaml`.

In M1, inventories and money are **per character**:

- **`InventoryItem`** — a line in a character's inventory: `character_id`, `name` (required), optional `item_definition_id` (a catalog reference; omitting it makes the line a free-text item), `quantity` (≥ 1), optional `description`.
- **`MoneyBalance`** — a character's holding of one currency: `character_id`, `currency_id`, `amount` (≥ 0). At most one balance per (character, currency), enforced by a composite unique index; `SetMoney` upserts it.

**Permission rule.** A character's inventory and money follow the same visibility as the character itself: **owner + GM only**. Every inventory/money service method resolves access through the character-visibility check first, so a caller who is neither the owner nor a GM gets `404` (existence hidden, never `403`). An inventory line addressed through the wrong character's path is likewise reported as `404`.

> M1 scopes inventories to characters. M2 generalises inventories to groups and locations and adds item movement between them.

### Endpoints

| Method & path | Who | Purpose |
|---|---|---|
| `GET /api/currencies` | any authenticated | List the currency catalog |
| `POST /api/currencies` | GM | Add a currency |
| `GET /api/items` | any authenticated | List the item catalog |
| `POST /api/items` | GM | Add an item definition |
| `GET /api/characters/{id}/inventory` | owner + GM | List a character's inventory |
| `POST /api/characters/{id}/inventory` | owner + GM | Add an inventory line |
| `PATCH /api/characters/{id}/inventory/{itemId}` | owner + GM | Edit name/quantity/description |
| `DELETE /api/characters/{id}/inventory/{itemId}` | owner + GM | Remove a line |
| `GET /api/characters/{id}/money` | owner + GM | List a character's balances |
| `PUT /api/characters/{id}/money/{currencyId}` | owner + GM | Set a balance to an absolute amount |
| `PUT /api/characters/{id}/location` | owner + GM | Associate the character with a location (`{"location_id": …}`) |
| `DELETE /api/characters/{id}/location` | owner + GM | Clear the character's location association |
| `GET /api/locations` | any authenticated | List locations |
| `POST /api/locations` | GM | Add a location |
| `GET /api/locations/{id}` | any authenticated | Get one location |
| `PATCH /api/locations/{id}` | GM | Edit a location's name/description/plane |

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

Full-text search over titles, file names, tags, and content (SQLite FTS5 is the natural fit). Access rules are applied **before** results are returned:

- Documents the character cannot see (wrong group, game day not reached) are excluded entirely — no titles, no hit counts
- GM-only sections are excluded from the searchable content for non-GM users
- Folder visibility follows the same rule: a folder appears only if it contains at least one accessible document
