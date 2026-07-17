// TypeScript types mirroring the Go API models (infrastructure/persistence/models).

export interface SetupStatus {
	needs_setup: boolean;
}

export interface InitialAccount {
	id: string;
	email: string;
	access_token: string;
}

export type Role = 'gm' | 'player';

export interface Account {
	id: string;
	email: string;
	role: Role;
}

export interface CreatedAccount extends Account {
	temporary_password: string;
}

export interface ResetPasswordResult {
	temporary_password: string;
}

export interface LoginResult {
	id: string;
	email: string;
	role: Role;
	access_token: string;
}

export interface Character {
	id: string;
	name: string;
	current_game_day: number;
	user_id: string;
	location_id?: string;
}

/** Cosmetic label — every group type behaves identically. */
export type GroupType = 'organization' | 'family' | 'other';

/** A group member as exposed to other players: identity only. */
export interface GroupMember {
	id: string;
	name: string;
}

export interface Group {
	id: string;
	name: string;
	type: GroupType;
	description?: string;
	members: GroupMember[];
}

/** A place or plane. Players only ever receive locations they can access —
 * an inaccessible location is absent from lists and 404s on direct reads. */
export interface Location {
	id: string;
	name: string;
	description?: string;
	plane?: string;
}

/** A GM-managed grant giving one character or one group access to a
 * location (view + modify, single level). */
export interface LocationAccess {
	id: string;
	location_id: string;
	character_id?: string;
	group_id?: string;
}

/** A GM-defined unit of money. `ratio` is the value of one unit in the base
 * unit (the smallest denomination, which has ratio 1). */
export interface Currency {
	id: string;
	code: string;
	name: string;
	ratio: number;
}

/** An entry in the GM's item catalog. A convenience for picking known items;
 * inventories always allow free-text items too. */
export interface ItemDefinition {
	id: string;
	name: string;
	description?: string;
	category?: string;
}

/** A line in an inventory, owned by exactly one character, group, or
 * location. `item_definition_id` is set when the line references a catalog
 * entry; free-text items omit it. */
export interface InventoryItem {
	id: string;
	character_id?: string;
	group_id?: string;
	location_id?: string;
	name: string;
	item_definition_id?: string;
	quantity: number;
	description?: string;
}

/** A character's or group's holding of a single currency. */
export interface MoneyBalance {
	id: string;
	character_id?: string;
	group_id?: string;
	currency_id: string;
	amount: number;
}

/** Addresses one inventory (or money pouch) by its owning entity. */
export interface InventoryOwnerRef {
	kind: 'character' | 'group' | 'location';
	id: string;
}

/** A per-character journal page. Readable and editable only by the owning
 * player and GMs — other players never see it. `game_day` is stamped at
 * creation from the character's `current_game_day` and never changes. */
export interface JournalEntry {
	id: string;
	character_id: string;
	game_day: number;
	content: string;
}

/** `general`/`template` are singletons visible to everyone; `group`/
 * `character` follow the membership/ownership of the entity they belong to. */
export type RepositoryType = 'general' | 'template' | 'group' | 'character';

/** A named vault of documents. Provisioned automatically — never created
 * directly by a caller. */
export interface Repository {
	id: string;
	type: RepositoryType;
	group_id?: string;
	character_id?: string;
}

/** One block of a document's content. `gm_only` sections are stripped
 * server-side before a player response is built — a player payload never
 * contains one, not even as a placeholder. */
export interface DocumentSection {
	id: string;
	content: string;
	gm_only: boolean;
}

/** A document's metadata, as returned in lists and folder trees (no
 * sections). */
export interface DocumentSummary {
	id: string;
	repository_id: string;
	path: string;
	title: string;
	tags: string[];
	shared_on_game_day: number;
}

/** A full document. `revealed` is whether any character with repository
 * access has reached `shared_on_game_day` — documents are not versioned, so
 * an already-revealed document's edits are immediately visible. */
export interface Document extends DocumentSummary {
	version: number;
	revealed: boolean;
	sections: DocumentSection[];
}

/** A GM-only direct share of a document to one character, independent of the
 * document's own repository access — revealed to that character once their
 * `current_game_day` reaches `shared_on_game_day`. */
export interface DocumentShare {
	id: string;
	document_id: string;
	character_id: string;
	shared_on_game_day: number;
}

/** What happened to the entity an activity entry describes. */
export type ActivityAction =
	'joined' | 'left' | 'added' | 'updated' | 'removed' | 'destroyed' | 'stolen';

/** One explicit recipient of an announced entry (GM view only). */
export interface ActivityTarget {
	character_id?: string;
	group_id?: string;
}

/** One event in the campaign activity log. A character's feed only ever
 * contains entries they may see: game-day gated, entity-access gated (or
 * announced to them), and with `actor` already stripped server-side on
 * announced entries for players — if `actor` is present, it may be shown. */
export interface ActivityEntry {
	id: string;
	game_day: number;
	action: ActivityAction;
	entity_type?: string;
	entity_id?: string;
	entity_name: string;
	actor?: string;
	character_id?: string;
	scope_type?: string;
	scope_id?: string;
	announced: boolean;
	announced_public?: boolean;
	targets?: ActivityTarget[];
	created_at: string;
}

/** A GM announcement: pushed to specific characters, groups, or everyone,
 * surfacing at `game_day` regardless of entity access. */
export interface AnnouncementInput {
	game_day: number;
	action: ActivityAction;
	entity_type?: string;
	entity_name: string;
	actor?: string;
	public?: boolean;
	character_ids?: string[];
	group_ids?: string[];
}

/** One level of a repository's folder tree. Folders with no documents the
 * caller may see are omitted entirely, at every level. */
export interface FolderTreeNode {
	name: string;
	path: string;
	folders: FolderTreeNode[];
	documents: DocumentSummary[];
}
