import { describe, it, expect, vi } from 'vitest';
import { announceActivity, listAllActivity, listCharacterActivity } from './activity';

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('listCharacterActivity', () => {
	it('sends the bearer token and returns the parsed feed', async () => {
		const feed = [
			{
				id: '1',
				game_day: 5,
				action: 'added',
				entity_name: 'Lockpicks',
				announced: false,
				created_at: '2026-01-01T00:00:00Z'
			}
		];
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(feed));

		const result = await listCharacterActivity('c1', 'token-123', fetchFn);

		expect(result).toEqual(feed);
		expect(fetchFn).toHaveBeenCalledWith('/api/characters/c1/activity', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('surfaces a 404 as an error — a hidden character reads as not found', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'not found' }, false, 404));

		await expect(listCharacterActivity('c1', 'token-123', fetchFn)).rejects.toThrow('not found');
	});
});

describe('listAllActivity', () => {
	it('fetches the GM-wide log', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse([]));

		await listAllActivity('token-123', fetchFn);

		expect(fetchFn).toHaveBeenCalledWith('/api/activity', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('surfaces a 403 as an error', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'forbidden' }, false, 403));

		await expect(listAllActivity('token-123', fetchFn)).rejects.toThrow('forbidden');
	});
});

describe('announceActivity', () => {
	it('posts the announcement and returns the created entry', async () => {
		const created = {
			id: '2',
			game_day: 4,
			action: 'stolen',
			entity_name: 'The Ruby of Vess',
			announced: true,
			created_at: '2026-01-01T00:00:00Z'
		};
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(created, true, 201));

		const input = {
			game_day: 4,
			action: 'stolen' as const,
			entity_name: 'The Ruby of Vess',
			character_ids: ['c1']
		};
		const result = await announceActivity(input, 'token-123', fetchFn);

		expect(result).toEqual(created);
		expect(fetchFn).toHaveBeenCalledWith('/api/activity/announcements', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify(input)
		});
	});

	it('throws the server error message on failure', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'invalid announcement' }, false, 400));

		await expect(
			announceActivity({ game_day: 1, action: 'stolen', entity_name: 'X' }, 'token-123', fetchFn)
		).rejects.toThrow('invalid announcement');
	});
});
