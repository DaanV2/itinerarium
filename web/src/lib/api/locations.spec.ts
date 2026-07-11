import { describe, it, expect, vi } from 'vitest';
import { createLocation, deleteLocation, listLocations } from './locations';

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('listLocations', () => {
	it('sends the bearer token and returns the parsed locations', async () => {
		const locations = [{ id: '1', name: 'The Material Plane' }];
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(locations));

		const result = await listLocations('token-123', fetchFn);

		expect(result).toEqual(locations);
		expect(fetchFn).toHaveBeenCalledWith('/api/locations', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('throws the server error message on failure', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'unauthenticated' }, false, 401));

		await expect(listLocations('token-123', fetchFn)).rejects.toThrow('unauthenticated');
	});
});

describe('createLocation', () => {
	it('posts a top-level plane and returns the created location', async () => {
		const created = { id: '2', name: 'The Feywild' };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(created, true, 201));

		const result = await createLocation({ name: 'The Feywild' }, 'token-123', fetchFn);

		expect(result).toEqual(created);
		expect(fetchFn).toHaveBeenCalledWith('/api/locations', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ name: 'The Feywild', description: undefined, parent_id: undefined })
		});
	});

	it('posts parent_id when nesting under another location', async () => {
		const created = { id: '3', name: 'Neverwinter', parent_id: '2' };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(created, true, 201));

		const result = await createLocation(
			{ name: 'Neverwinter', description: 'A city.', parentId: '2' },
			'token-123',
			fetchFn
		);

		expect(result).toEqual(created);
		expect(fetchFn).toHaveBeenCalledWith('/api/locations', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ name: 'Neverwinter', description: 'A city.', parent_id: '2' })
		});
	});

	it('throws the server error message on failure', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'forbidden' }, false, 403));

		await expect(createLocation({ name: 'Nope' }, 'token-123', fetchFn)).rejects.toThrow(
			'forbidden'
		);
	});
});

describe('deleteLocation', () => {
	it('sends a DELETE with the bearer token', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(null, true, 204));

		await deleteLocation('3', 'token-123', fetchFn);

		expect(fetchFn).toHaveBeenCalledWith('/api/locations/3', {
			method: 'DELETE',
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('throws the server error message when the location has children', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'location has child locations' }, false, 409));

		await expect(deleteLocation('2', 'token-123', fetchFn)).rejects.toThrow(
			'location has child locations'
		);
	});
});
