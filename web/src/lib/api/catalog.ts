import type { Currency, ItemDefinition } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

/** Lists the currency catalog. Readable by any authenticated user. */
export async function listCurrencies(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Currency[]> {
	const res = await fetchFn('/api/currencies', {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list currencies: ${res.status}`));
	}

	return (await res.json()) as Currency[];
}

/** Adds a currency to the catalog. GM only. */
export async function createCurrency(
	input: { code: string; name: string; ratio: number },
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Currency> {
	const res = await fetchFn('/api/currencies', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to create currency: ${res.status}`));
	}

	return (await res.json()) as Currency;
}

/** Lists the item catalog. Readable by any authenticated user. */
export async function listItemDefinitions(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<ItemDefinition[]> {
	const res = await fetchFn('/api/items', {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list items: ${res.status}`));
	}

	return (await res.json()) as ItemDefinition[];
}

/** Adds an item definition to the catalog. GM only. */
export async function createItemDefinition(
	input: { name: string; description?: string; category?: string },
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<ItemDefinition> {
	const res = await fetchFn('/api/items', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to create item definition: ${res.status}`));
	}

	return (await res.json()) as ItemDefinition;
}
