import type { InventoryItem, InventoryOwnerRef, MoneyBalance } from '$lib/types';
import { apiFetch, apiSend } from './client';

/** Base API path of the entity owning an inventory/money pouch. */
function ownerPath(owner: InventoryOwnerRef): string {
	switch (owner.kind) {
		case 'character':
			return `/api/characters/${owner.id}`;
		case 'group':
			return `/api/groups/${owner.id}`;
		case 'location':
			return `/api/locations/${owner.id}`;
	}
}

/** Lists an inventory. The API returns 404 for owners the caller may not
 * see — surface that as not-found, don't special-case it. */
export async function listInventory(
	owner: InventoryOwnerRef,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<InventoryItem[]> {
	return apiFetch<InventoryItem[]>(`${ownerPath(owner)}/inventory`, {
		token,
		errorContext: 'failed to list inventory',
		fetchFn
	});
}

/** Fields accepted when adding an inventory line. `name` is required; omit
 * `item_definition_id` for a free-text item. */
export interface AddInventoryItemInput {
	name: string;
	quantity: number;
	item_definition_id?: string;
	description?: string;
}

/** Adds a line to an inventory. */
export async function addInventoryItem(
	owner: InventoryOwnerRef,
	input: AddInventoryItemInput,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<InventoryItem> {
	return apiFetch<InventoryItem>(`${ownerPath(owner)}/inventory`, {
		method: 'POST',
		token,
		body: input,
		errorContext: 'failed to add item',
		fetchFn
	});
}

/** Fields that can be changed on an inventory line; omit a field to leave it. */
export interface UpdateInventoryItemInput {
	name?: string;
	quantity?: number;
	description?: string;
}

/** Edits an existing inventory line. */
export async function updateInventoryItem(
	owner: InventoryOwnerRef,
	itemId: string,
	input: UpdateInventoryItemInput,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<InventoryItem> {
	return apiFetch<InventoryItem>(`${ownerPath(owner)}/inventory/${itemId}`, {
		method: 'PATCH',
		token,
		body: input,
		errorContext: 'failed to update item',
		fetchFn
	});
}

/** Removes an inventory line. */
export async function removeInventoryItem(
	owner: InventoryOwnerRef,
	itemId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<void> {
	await apiSend(`${ownerPath(owner)}/inventory/${itemId}`, {
		method: 'DELETE',
		token,
		errorContext: 'failed to remove item',
		fetchFn
	});
}

/** Moves quantity units of an item into another inventory the caller can
 * access. Partial moves split the line; a matching line in the target absorbs
 * the units. */
export async function moveInventoryItem(
	itemId: string,
	target: InventoryOwnerRef,
	quantity: number,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<InventoryItem> {
	const body: Record<string, unknown> = { item_id: itemId, quantity };
	if (target.kind === 'character') body.to_character_id = target.id;
	if (target.kind === 'group') body.to_group_id = target.id;
	if (target.kind === 'location') body.to_location_id = target.id;

	return apiFetch<InventoryItem>('/api/inventory/move', {
		method: 'POST',
		token,
		body,
		errorContext: 'failed to move item',
		fetchFn
	});
}

/** Lists an owner's money balances across all currencies (characters and
 * groups only — locations hold items, not money). */
export async function listMoney(
	owner: InventoryOwnerRef,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<MoneyBalance[]> {
	return apiFetch<MoneyBalance[]>(`${ownerPath(owner)}/money`, {
		token,
		errorContext: 'failed to list money',
		fetchFn
	});
}

/** Sets an owner's balance in one currency to an absolute amount. */
export async function setMoney(
	owner: InventoryOwnerRef,
	currencyId: string,
	amount: number,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<MoneyBalance> {
	return apiFetch<MoneyBalance>(`${ownerPath(owner)}/money/${currencyId}`, {
		method: 'PUT',
		token,
		body: { amount },
		errorContext: 'failed to set money',
		fetchFn
	});
}
