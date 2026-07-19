import { describe, it, expect, vi } from 'vitest';
import { searchDocuments } from './search';

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('searchDocuments', () => {
	it('encodes the query and returns the parsed results', async () => {
		const results = [
			{ id: 'd1', title: 'Dragon Lore', path: 'lore/dragons', matched_in: ['title'] }
		];
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(results));

		const found = await searchDocuments('50% dragon', 'token-123', fetchFn);

		expect(found).toEqual(results);
		expect(fetchFn).toHaveBeenCalledWith('/api/search?q=50%25%20dragon', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('surfaces an API error message', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'search query is required' }, false, 400));

		await expect(searchDocuments('', 'token-123', fetchFn)).rejects.toThrow(
			'search query is required'
		);
	});
});
