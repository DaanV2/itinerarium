# Features

## Players

- Multiple characters per account
- Per-character journal entries stamped to a specific in-game day
- Journals are readable by the player and the GM; other players never see them
- **Journal pages can be converted into a knowledge document**: the entry is copied into the character's knowledge repository as a new document (the journal original stays untouched), and from there it can be shared to a group or character on a game day like any other document
- **Each character has a private knowledge repository**: a personal vault-style folder tree of documents only the character (and the GM) can see, separate from the shared campaign knowledge base. Sharing a personal document moves it into the normal visibility rules
- Each character has a personal inventory (item list + quantity) and money (per currency), and can move items between their own inventory and any group or location inventory they can access
- View documents and knowledge the character has access to, gated by game day and group membership

## Game Masters

- Full character and player management
- Invite new players by creating their account and providing login credentials
- Reset a player's password by generating a new temporary password from the admin panel — no email server required
- Can read all player journals
- Knowledge base with GM-only document sections hidden from players
- Advance or roll back game day for all characters in a session, or per character for players catching up

## Groups

Characters can join groups. All group types share the same mechanics:

- Shared inventory (item list + quantity)
- Shared money (per currency)
- Shared knowledge (documents become visible to members from a specific game day onward)

The `type` field (`organization`, `family`, `other`) is cosmetic — identical behaviour for all three.

Joining a group grants access to all its knowledge (subject to the game-day rule); leaving removes access to anything not otherwise granted. Join and leave events are recorded in the activity log, stamped with the game day, so the group's membership history is visible over time.

## Items & Currencies

- Currencies are defined by the GM as a JSON/YAML list with **conversion ratios** (e.g. 1 gold = 10 silver = 100 copper), shared by all inventories (character, group, location)
- A **currency calculator API** answers "how much of X is Y", adds amounts across currencies, and simplifies a total into the fewest coins — available to any authenticated user, no balance read or write involved
- The GM can provide a **default item catalog** (JSON/YAML list) so players pick from known items when adding to an inventory
- **Custom free-text items are always allowed** alongside catalog items — the catalog is a convenience, not a restriction

See [currency.md](currency.md) for how currencies and money work, and [currency-configuration.md](currency-configuration.md) for defining them.

## Documentation

### Repositories

The knowledge base is divided into **repositories**, each with its own folder tree:

| Repository | Contents | Visible to |
|-----------|----------|-----------|
| General | Campaign-wide documents from the GM | Everyone |
| Group repositories | One per organization / family / other group | Group members |
| Template repository | Document templates to start new documents from | Everyone |
| Character repositories | Each character's private knowledge | Owning player + GM |

A document lives in exactly one repository; the repository determines who can see it, combined with game-day gating. Creating or importing a document at a path that already exists in the same repository triggers a **collision warning** — the user can rename or continue.

### Documents

- Markdown documents with optional GM-only sections
- GM sections are hidden from players; a clear visual banner marks the boundary in the UI
- Documents can be shared from a character to a group on a specific game day; other members see it once their own game day reaches that point
- Documents are **not versioned by game day**: the game day gates *whether* a character sees a document, but everyone who can see it sees the latest content. GMs should keep this in mind when editing documents that contain future reveals
- Visibility follows the document's repository (see above); in addition, any document can be **directly shared to specific characters** on a game day
- **Documents are editable by everyone who can see them.** Saving over someone else's concurrent edit shows a warning first. GM-only sections remain invisible and untouchable for players: if a document consisted only of GM-only sections so far, a player's edit is added as a new player-visible section alongside them, never merged into GM content
- **Obsidian compatibility**: documents support YAML frontmatter (the `---` block) for metadata such as title, tags, visibility, and game day. This allows GMs to author and manage their knowledge base in Obsidian and import/sync `.md` files directly into Itinerarium.
- **Folder organisation, like Obsidian**: documents live in a folder tree (vault-style), shown as a collapsible file explorer in the sidebar. Folders and documents are sorted alphabetically, with manual pinning/reordering as a possible later addition. Players only see folders that contain at least one document they can access — empty-looking branches are hidden entirely.
- **Search**: full-text search across titles, file names, tags, and document content. Results are filtered by the searching character's access — a document (or GM-only section) that the character cannot see is never matched, not even as a hit count. GMs search across everything.

### Document editor

- The editor prominently shows the document's reveal settings: "Revealed at game day X to <group/character/public>"
- When editing an already-revealed document, the editor shows a warning banner: changes are immediately visible to everyone who can currently see the document — there is no versioning. It suggests creating a new document (or GM-only section) for future reveals instead
- GM-only sections are visually marked in the editor with the same banner/widget style players see at section boundaries

## Activity Log

Each character has a personal activity feed showing what changed in their world up to their current game day.

- Entries are stamped with a `game_day` and only appear once the character's `current_game_day` reaches that value
- Visibility mirrors the underlying entity: a character only sees log entries for things they already have access to (their groups, their locations, public documents, etc.)
- GMs see all activity regardless of game day

Tracked events:

| Category | Events |
|----------|--------|
| Group membership | Character joined, character left |
| Group inventory | Item added, quantity changed, item removed |
| Location inventory | Item added, quantity changed, item removed (only if character has location access) |
| Knowledge / documents | Document added, document updated, document removed |
| Group money | Balance changed |

Each entry records: `game_day`, `action` (`added` / `updated` / `removed` / `destroyed` / `stolen`), the entity type and name, and who made the change (character name or GM).

### Announcements

Activity entries can be marked as **announced**, which overrides normal access rules and pushes the entry to specific characters or groups regardless of whether they have access to the source entity. This is used for events that are inherently public knowledge to affected parties:

- A document or item is **stolen** — the owning group or location sees the theft event even if the thief (and the stolen thing) are no longer accessible
- An item or document is **destroyed** — characters present or in the owning group see the destruction event even if the entity no longer exists
- Any other action the GM wants to broadcast

The GM sets the announcement targets (specific characters, a group, or public) and the `game_day` at which it surfaces.

**What players see vs. GMs:**

| Field | Players | GMs |
|-------|---------|-----|
| What happened (`action`) | Yes | Yes |
| Entity name | Yes | Yes |
| Who did it (`actor`) | No — hidden | Yes |

The actor field is GM-only on announced entries, so players know *that* something was stolen or destroyed but not *who* did it. GMs always see the full picture.

## Sessions

- Track which characters participate in a session
- Each character has its own `current_game_day` tracked independently
- GM can advance or rewind game day for all session participants, or per character for players catching up
- Sessions are a GM-only tool: creating/editing sessions, managing participants, and advancing/rewinding game day all require the GM role — players don't interact with sessions directly

## Locations / Planes

- Named locations (towns, buildings, planes, rooms — anything physical) that characters and sessions can be associated with
- Supports multi-plane campaigns out of the box
- Each location can have its own **inventory** (item list + quantity), similar to groups — e.g. a character's house storing gear
- Location inventories have **access control**: a single access level grants both view and modify; characters without access cannot see the inventory exists
- Locations can have an associated description document, subject to the same visibility and game-day rules as other documents
- **Locations are editable by players**, same rule as knowledge: anyone who can see a location can edit its description and details
- Access can be granted per-character or via group membership; GMs always have full visibility
