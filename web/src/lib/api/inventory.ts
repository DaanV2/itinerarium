import type { InventoryItem, MoneyBalance } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

/** Lists a character's inventory. The API returns 404 for characters the
 * caller may not see — surface that as not-found, don't special-case it. */
export async function listInventory(
	characterId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<InventoryItem[]> {
	const res = await fetchFn(`/api/characters/${characterId}/inventory`, {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list inventory: ${res.status}`));
	}

	return (await res.json()) as InventoryItem[];
}

/** Fields accepted when adding an inventory line. `name` is required; omit
 * `item_definition_id` for a free-text item. */
export interface AddInventoryItemInput {
	name: string;
	quantity: number;
	item_definition_id?: string;
	description?: string;
}

/** Adds a line to a character's inventory. */
export async function addInventoryItem(
	characterId: string,
	input: AddInventoryItemInput,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<InventoryItem> {
	const res = await fetchFn(`/api/characters/${characterId}/inventory`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to add item: ${res.status}`));
	}

	return (await res.json()) as InventoryItem;
}

/** Fields that can be changed on an inventory line; omit a field to leave it. */
export interface UpdateInventoryItemInput {
	name?: string;
	quantity?: number;
	description?: string;
}

/** Edits an existing inventory line. */
export async function updateInventoryItem(
	characterId: string,
	itemId: string,
	input: UpdateInventoryItemInput,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<InventoryItem> {
	const res = await fetchFn(`/api/characters/${characterId}/inventory/${itemId}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to update item: ${res.status}`));
	}

	return (await res.json()) as InventoryItem;
}

/** Removes an inventory line. */
export async function removeInventoryItem(
	characterId: string,
	itemId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<void> {
	const res = await fetchFn(`/api/characters/${characterId}/inventory/${itemId}`, {
		method: 'DELETE',
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to remove item: ${res.status}`));
	}
}

/** Lists a character's money balances across all currencies. */
export async function listMoney(
	characterId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<MoneyBalance[]> {
	const res = await fetchFn(`/api/characters/${characterId}/money`, {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list money: ${res.status}`));
	}

	return (await res.json()) as MoneyBalance[];
}

/** Sets a character's balance in one currency to an absolute amount. */
export async function setMoney(
	characterId: string,
	currencyId: string,
	amount: number,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<MoneyBalance> {
	const res = await fetchFn(`/api/characters/${characterId}/money/${currencyId}`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({ amount })
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to set money: ${res.status}`));
	}

	return (await res.json()) as MoneyBalance;
}
