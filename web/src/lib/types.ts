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
