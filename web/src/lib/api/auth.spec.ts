import { describe, it, expect, vi } from 'vitest';
import { login } from './auth';

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('login', () => {
	it('posts the credentials and returns the logged-in account', async () => {
		const result = { id: '1', email: 'player@example.com', role: 'player', access_token: 'token' };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(result));

		const account = await login('player@example.com', 'hunter22hunter', fetchFn);

		expect(account).toEqual(result);
		expect(fetchFn).toHaveBeenCalledWith('/api/login', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email: 'player@example.com', password: 'hunter22hunter' })
		});
	});

	it('throws the server error message on invalid credentials', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'invalid email or password' }, false, 401));

		await expect(login('player@example.com', 'wrong-password', fetchFn)).rejects.toThrow(
			'invalid email or password'
		);
	});

	it('throws a fallback message when the error body has no message', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({}, false, 500));

		await expect(login('player@example.com', 'hunter22hunter', fetchFn)).rejects.toThrow(
			'login failed: 500'
		);
	});
});
