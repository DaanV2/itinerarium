import { describe, it, expect, vi } from 'vitest';
import { createCharacter, listCharacters } from './characters';

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('listCharacters', () => {
	it('sends the bearer token and returns the parsed characters', async () => {
		const characters = [{ id: '1', name: 'Aria', current_game_day: 0, user_id: 'u1' }];
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(characters));

		const result = await listCharacters('token-123', fetchFn);

		expect(result).toEqual(characters);
		expect(fetchFn).toHaveBeenCalledWith('/api/characters', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('throws the server error message on failure', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'unauthenticated' }, false, 401));

		await expect(listCharacters('token-123', fetchFn)).rejects.toThrow('unauthenticated');
	});
});

describe('createCharacter', () => {
	it('posts the name and returns the created character', async () => {
		const created = { id: '2', name: 'Beren', current_game_day: 0, user_id: 'u1' };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(created, true, 201));

		const result = await createCharacter('Beren', 'token-123', fetchFn);

		expect(result).toEqual(created);
		expect(fetchFn).toHaveBeenCalledWith('/api/characters', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ name: 'Beren' })
		});
	});

	it('throws the server error message on failure', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'invalid name' }, false, 400));

		await expect(createCharacter('', 'token-123', fetchFn)).rejects.toThrow('invalid name');
	});
});
