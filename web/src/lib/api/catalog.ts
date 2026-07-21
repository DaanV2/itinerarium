import type { Currency, ItemDefinition } from '$lib/types';
import { apiFetch } from './client';

/** Lists the currency catalog. Readable by any authenticated user. */
export async function listCurrencies(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Currency[]> {
	return apiFetch<Currency[]>('/api/currencies', {
		token,
		errorContext: 'failed to list currencies',
		fetchFn
	});
}

/** Adds a currency to the catalog. GM only. */
export async function createCurrency(
	input: { code: string; name: string; ratio: number },
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Currency> {
	return apiFetch<Currency>('/api/currencies', {
		method: 'POST',
		token,
		body: input,
		errorContext: 'failed to create currency',
		fetchFn
	});
}

/** Lists the item catalog. Readable by any authenticated user. */
export async function listItemDefinitions(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<ItemDefinition[]> {
	return apiFetch<ItemDefinition[]>('/api/items', {
		token,
		errorContext: 'failed to list items',
		fetchFn
	});
}

/** Adds an item definition to the catalog. GM only. */
export async function createItemDefinition(
	input: { name: string; description?: string; category?: string },
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<ItemDefinition> {
	return apiFetch<ItemDefinition>('/api/items', {
		method: 'POST',
		token,
		body: input,
		errorContext: 'failed to create item definition',
		fetchFn
	});
}
