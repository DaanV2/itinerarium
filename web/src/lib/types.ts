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
