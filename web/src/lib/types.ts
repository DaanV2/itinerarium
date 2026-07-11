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

/** A line in a character's personal inventory. `item_definition_id` is set when
 * the line references a catalog entry; free-text items omit it. */
export interface InventoryItem {
	id: string;
	character_id: string;
	name: string;
	item_definition_id?: string;
	quantity: number;
	description?: string;
}

/** A character's holding of a single currency. */
export interface MoneyBalance {
	id: string;
	character_id: string;
	currency_id: string;
	amount: number;
}

/** A named place in the campaign world — a plane, town, building, or room.
 * Locations form a hierarchy: a location with no `parent_id` is a top-level
 * plane, and nesting one under another models physical containment. This is how
 * multi-plane campaigns are supported. */
export interface Location {
	id: string;
	name: string;
	description?: string;
	parent_id?: string;
}
