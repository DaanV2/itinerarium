import { describe, it, expect, vi } from 'vitest';
import { createInitialAccount, getSetupStatus } from './setup';

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('getSetupStatus', () => {
	it('returns the parsed status on success', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ needs_setup: true }));

		const status = await getSetupStatus(fetchFn);

		expect(status).toEqual({ needs_setup: true });
		expect(fetchFn).toHaveBeenCalledWith('/api/setup');
	});

	it('throws when the response is not ok', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({}, false, 500));

		await expect(getSetupStatus(fetchFn)).rejects.toThrow('failed to check setup status: 500');
	});
});

describe('createInitialAccount', () => {
	it('posts the credentials and returns the created account', async () => {
		const account = { id: '1', email: 'gm@example.com', access_token: 'token' };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(account, true, 201));

		const result = await createInitialAccount('gm@example.com', 'hunter22hunter', fetchFn);

		expect(result).toEqual(account);
		expect(fetchFn).toHaveBeenCalledWith('/api/setup', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email: 'gm@example.com', password: 'hunter22hunter' })
		});
	});

	it('throws the server error message when setup is refused', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'setup already completed' }, false, 409));

		await expect(createInitialAccount('gm@example.com', 'hunter22hunter', fetchFn)).rejects.toThrow(
			'setup already completed'
		);
	});
});
