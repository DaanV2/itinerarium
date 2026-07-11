import { describe, it, expect, vi } from 'vitest';
import {
	listInventory,
	addInventoryItem,
	updateInventoryItem,
	removeInventoryItem,
	moveInventoryItem,
	listMoney,
	setMoney
} from './inventory';
import type { InventoryOwnerRef } from '$lib/types';

const character: InventoryOwnerRef = { kind: 'character', id: 'c1' };
const group: InventoryOwnerRef = { kind: 'group', id: 'g1' };
const location: InventoryOwnerRef = { kind: 'location', id: 'l1' };

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('listInventory', () => {
	it('sends the bearer token and returns the parsed items', async () => {
		const items = [{ id: '1', character_id: 'c1', name: 'Torch', quantity: 3 }];
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(items));

		const result = await listInventory(character, 'token-123', fetchFn);

		expect(result).toEqual(items);
		expect(fetchFn).toHaveBeenCalledWith('/api/characters/c1/inventory', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('addresses group and location inventories by their own paths', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse([]));

		await listInventory(group, 'token-123', fetchFn);
		await listInventory(location, 'token-123', fetchFn);

		expect(fetchFn).toHaveBeenNthCalledWith(1, '/api/groups/g1/inventory', {
			headers: { Authorization: 'Bearer token-123' }
		});
		expect(fetchFn).toHaveBeenNthCalledWith(2, '/api/locations/l1/inventory', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('surfaces a 404 as an error rather than special-casing it', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'not found' }, false, 404));

		await expect(listInventory(character, 'token-123', fetchFn)).rejects.toThrow('not found');
	});
});

describe('addInventoryItem', () => {
	it('posts a free-text item and returns the created line', async () => {
		const created = { id: '2', character_id: 'c1', name: 'Trinket', quantity: 1 };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(created, true, 201));

		const result = await addInventoryItem(
			character,
			{ name: 'Trinket', quantity: 1 },
			'token-123',
			fetchFn
		);

		expect(result).toEqual(created);
		expect(fetchFn).toHaveBeenCalledWith('/api/characters/c1/inventory', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ name: 'Trinket', quantity: 1 })
		});
	});
});

describe('updateInventoryItem', () => {
	it('patches the quantity and returns the updated line', async () => {
		const updated = { id: '2', character_id: 'c1', name: 'Torch', quantity: 5 };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(updated));

		const result = await updateInventoryItem(character, '2', { quantity: 5 }, 'token-123', fetchFn);

		expect(result).toEqual(updated);
		expect(fetchFn).toHaveBeenCalledWith('/api/characters/c1/inventory/2', {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ quantity: 5 })
		});
	});
});

describe('removeInventoryItem', () => {
	it('sends a DELETE with the bearer token', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(null, true, 204));

		await removeInventoryItem(character, '2', 'token-123', fetchFn);

		expect(fetchFn).toHaveBeenCalledWith('/api/characters/c1/inventory/2', {
			method: 'DELETE',
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('throws the server error message on failure', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'not found' }, false, 404));

		await expect(removeInventoryItem(character, '2', 'token-123', fetchFn)).rejects.toThrow(
			'not found'
		);
	});
});

describe('moveInventoryItem', () => {
	it('posts the move with the target keyed by owner kind', async () => {
		const moved = { id: '2', group_id: 'g1', name: 'Torch', quantity: 2 };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(moved));

		const result = await moveInventoryItem('2', group, 2, 'token-123', fetchFn);

		expect(result).toEqual(moved);
		expect(fetchFn).toHaveBeenCalledWith('/api/inventory/move', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ item_id: '2', quantity: 2, to_group_id: 'g1' })
		});
	});

	it('surfaces the server error message on refusal', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'invalid quantity' }, false, 400));

		await expect(moveInventoryItem('2', location, 99, 'token-123', fetchFn)).rejects.toThrow(
			'invalid quantity'
		);
	});
});

describe('listMoney', () => {
	it('returns the parsed balances', async () => {
		const balances = [{ id: '1', character_id: 'c1', currency_id: 'gp', amount: 42 }];
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(balances));

		const result = await listMoney(character, 'token-123', fetchFn);

		expect(result).toEqual(balances);
		expect(fetchFn).toHaveBeenCalledWith('/api/characters/c1/money', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('addresses group money by the group path', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse([]));

		await listMoney(group, 'token-123', fetchFn);

		expect(fetchFn).toHaveBeenCalledWith('/api/groups/g1/money', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});
});

describe('setMoney', () => {
	it('puts the absolute amount and returns the balance', async () => {
		const balance = { id: '1', character_id: 'c1', currency_id: 'gp', amount: 75 };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(balance));

		const result = await setMoney(character, 'gp', 75, 'token-123', fetchFn);

		expect(result).toEqual(balance);
		expect(fetchFn).toHaveBeenCalledWith('/api/characters/c1/money/gp', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ amount: 75 })
		});
	});
});
