import { describe, it, expect, vi } from 'vitest';
import { importVault } from './vault-import';

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('importVault', () => {
	it('posts the batch and returns the per-file results', async () => {
		const results = [
			{ path: 'lore/origins.md', status: 'imported', document_id: 'd1', repository_id: 'r1' },
			{ path: 'lore/dupe.md', status: 'collision', error: 'a document already exists' }
		];
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ results }));

		const files = [
			{ path: 'lore/origins.md', markdown: '# Origins' },
			{ path: 'lore/dupe.md', markdown: '# Dupe', allow_collision: false }
		];
		const got = await importVault('r1', files, 'token-123', fetchFn);

		expect(got).toEqual(results);
		expect(fetchFn).toHaveBeenCalledWith('/api/import/obsidian', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ repository_id: 'r1', files })
		});
	});

	it('surfaces a 404 on an inaccessible default repository', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'not found' }, false, 404));

		await expect(
			importVault('r1', [{ path: 'a.md', markdown: 'x' }], 'token-123', fetchFn)
		).rejects.toThrow('not found');
	});
});
