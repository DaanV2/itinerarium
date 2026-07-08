import { describe, it, expect, vi } from 'vitest';
import {
	listCurrencies,
	createCurrency,
	listItemDefinitions,
	createItemDefinition
} from './catalog';

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('listCurrencies', () => {
	it('sends the bearer token and returns the parsed currencies', async () => {
		const currencies = [{ id: '1', code: 'gp', name: 'Gold', ratio: 100 }];
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(currencies));

		const result = await listCurrencies('token-123', fetchFn);

		expect(result).toEqual(currencies);
		expect(fetchFn).toHaveBeenCalledWith('/api/currencies', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('throws the server error message on failure', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'unauthenticated' }, false, 401));

		await expect(listCurrencies('token-123', fetchFn)).rejects.toThrow('unauthenticated');
	});
});

describe('createCurrency', () => {
	it('posts the currency and returns the created row', async () => {
		const created = { id: '2', code: 'sp', name: 'Silver', ratio: 10 };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(created, true, 201));

		const result = await createCurrency(
			{ code: 'sp', name: 'Silver', ratio: 10 },
			'token-123',
			fetchFn
		);

		expect(result).toEqual(created);
		expect(fetchFn).toHaveBeenCalledWith('/api/currencies', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ code: 'sp', name: 'Silver', ratio: 10 })
		});
	});

	it('throws the server error message on a forbidden response', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'forbidden' }, false, 403));

		await expect(
			createCurrency({ code: 'sp', name: 'Silver', ratio: 10 }, 'token-123', fetchFn)
		).rejects.toThrow('forbidden');
	});
});

describe('listItemDefinitions', () => {
	it('returns the parsed item catalog', async () => {
		const items = [{ id: '1', name: 'Torch', category: 'gear' }];
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(items));

		const result = await listItemDefinitions('token-123', fetchFn);

		expect(result).toEqual(items);
		expect(fetchFn).toHaveBeenCalledWith('/api/items', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});
});

describe('createItemDefinition', () => {
	it('posts the item and returns the created row', async () => {
		const created = { id: '3', name: 'Rope', description: '50 ft' };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(created, true, 201));

		const result = await createItemDefinition(
			{ name: 'Rope', description: '50 ft' },
			'token-123',
			fetchFn
		);

		expect(result).toEqual(created);
		expect(fetchFn).toHaveBeenCalledWith('/api/items', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ name: 'Rope', description: '50 ft' })
		});
	});
});
