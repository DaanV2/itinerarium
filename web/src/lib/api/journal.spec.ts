import { describe, it, expect, vi } from 'vitest';
import { createJournalEntry, listJournalEntries, updateJournalEntry } from './journal';

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('listJournalEntries', () => {
	it('sends the bearer token and returns the parsed entries', async () => {
		const entries = [{ id: '1', character_id: 'c1', game_day: 2, content: 'Dear diary' }];
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(entries));

		const result = await listJournalEntries('c1', 'token-123', fetchFn);

		expect(result).toEqual(entries);
		expect(fetchFn).toHaveBeenCalledWith('/api/characters/c1/journal', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('surfaces a 404 as an error', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'not found' }, false, 404));

		await expect(listJournalEntries('c1', 'token-123', fetchFn)).rejects.toThrow('not found');
	});
});

describe('createJournalEntry', () => {
	it('posts the content and returns the created entry', async () => {
		const created = { id: '2', character_id: 'c1', game_day: 3, content: 'New entry' };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(created, true, 201));

		const result = await createJournalEntry('c1', 'New entry', 'token-123', fetchFn);

		expect(result).toEqual(created);
		expect(fetchFn).toHaveBeenCalledWith('/api/characters/c1/journal', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ content: 'New entry' })
		});
	});

	it('throws the server error message on failure', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'invalid content' }, false, 400));

		await expect(createJournalEntry('c1', '', 'token-123', fetchFn)).rejects.toThrow(
			'invalid content'
		);
	});
});

describe('updateJournalEntry', () => {
	it('patches the content and returns the updated entry', async () => {
		const updated = { id: '2', character_id: 'c1', game_day: 3, content: 'Revised' };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(updated));

		const result = await updateJournalEntry('c1', '2', 'Revised', 'token-123', fetchFn);

		expect(result).toEqual(updated);
		expect(fetchFn).toHaveBeenCalledWith('/api/characters/c1/journal/2', {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ content: 'Revised' })
		});
	});

	it('surfaces a 404 as an error', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'not found' }, false, 404));

		await expect(updateJournalEntry('c1', '2', 'Revised', 'token-123', fetchFn)).rejects.toThrow(
			'not found'
		);
	});
});
