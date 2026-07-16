import { describe, it, expect, vi } from 'vitest';
import { getDocument, updateDocument, DocumentConflictError } from './documents';

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('getDocument', () => {
	it('sends the bearer token and returns the parsed document', async () => {
		const doc = { id: 'd1', title: 'Notes', path: '/notes', tags: [], version: 1 };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(doc));

		const result = await getDocument('d1', 'token-123', fetchFn);

		expect(result).toEqual(doc);
		expect(fetchFn).toHaveBeenCalledWith('/api/documents/d1', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('surfaces a 404 as an error', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'not found' }, false, 404));

		await expect(getDocument('d1', 'token-123', fetchFn)).rejects.toThrow('not found');
	});
});

describe('updateDocument', () => {
	const input = {
		title: 'Notes',
		path: '/notes',
		tags: ['a'],
		sharedOnGameDay: 2,
		sections: [{ id: 's1', content: 'hi', gm_only: false }],
		expectedVersion: 3
	};

	it('patches the document and returns the updated version', async () => {
		const updated = { id: 'd1', title: 'Notes', path: '/notes', tags: ['a'], version: 4 };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(updated));

		const result = await updateDocument('d1', input, 'token-123', fetchFn);

		expect(result).toEqual(updated);
		expect(fetchFn).toHaveBeenCalledWith('/api/documents/d1', {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({
				path: '/notes',
				title: 'Notes',
				tags: ['a'],
				shared_on_game_day: 2,
				sections: [{ id: 's1', content: 'hi', gm_only: false }],
				expected_version: 3,
				force: false,
				allow_collision: false
			})
		});
	});

	it('throws a DocumentConflictError with code concurrent_edit on a stale version', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(
				jsonResponse(
					{ error: 'the document changed since it was loaded', code: 'concurrent_edit' },
					false,
					409
				)
			);

		await expect(updateDocument('d1', input, 'token-123', fetchFn)).rejects.toMatchObject({
			name: 'DocumentConflictError',
			code: 'concurrent_edit'
		});
	});

	it('sends force: true to override a concurrent edit', async () => {
		const updated = { id: 'd1', title: 'Notes', path: '/notes', tags: ['a'], version: 5 };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(updated));

		await updateDocument('d1', { ...input, force: true }, 'token-123', fetchFn);

		const body = JSON.parse(fetchFn.mock.calls[0][1].body as string) as { force: boolean };
		expect(body.force).toBe(true);
	});

	it('surfaces a non-conflict 409 as a plain error', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'already shared' }, false, 409));

		const result = updateDocument('d1', input, 'token-123', fetchFn);

		await expect(result).rejects.toThrow('already shared');
		await expect(result).rejects.not.toBeInstanceOf(DocumentConflictError);
	});

	it('surfaces a 400 as a plain error', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'invalid title' }, false, 400));

		await expect(updateDocument('d1', input, 'token-123', fetchFn)).rejects.toThrow(
			'invalid title'
		);
	});
});
