import { describe, it, expect, vi } from 'vitest';
import { createAccount, listAccounts, resetPassword } from './accounts';

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('listAccounts', () => {
	it('sends the bearer token and returns the parsed accounts', async () => {
		const accounts = [{ id: '1', email: 'gm@example.com', role: 'gm' }];
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(accounts));

		const result = await listAccounts('token-123', fetchFn);

		expect(result).toEqual(accounts);
		expect(fetchFn).toHaveBeenCalledWith('/api/admin/users', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('throws the server error message on failure', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'forbidden' }, false, 403));

		await expect(listAccounts('token-123', fetchFn)).rejects.toThrow('forbidden');
	});
});

describe('createAccount', () => {
	it('posts the email and role and returns the created account with its temporary password', async () => {
		const created = {
			id: '2',
			email: 'player@example.com',
			role: 'player',
			temporary_password: 'ABCDEFGH'
		};
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(created, true, 201));

		const result = await createAccount('player@example.com', 'player', 'token-123', fetchFn);

		expect(result).toEqual(created);
		expect(fetchFn).toHaveBeenCalledWith('/api/admin/users', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Authorization: 'Bearer token-123' },
			body: JSON.stringify({ email: 'player@example.com', role: 'player' })
		});
	});

	it('throws the server error message on failure', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'email already in use' }, false, 409));

		await expect(
			createAccount('player@example.com', 'player', 'token-123', fetchFn)
		).rejects.toThrow('email already in use');
	});
});

describe('resetPassword', () => {
	it('posts to the reset endpoint and returns the new temporary password', async () => {
		const result = { temporary_password: 'NEWPASS1' };
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse(result));

		const outcome = await resetPassword('user-1', 'token-123', fetchFn);

		expect(outcome).toEqual(result);
		expect(fetchFn).toHaveBeenCalledWith('/api/admin/users/user-1/reset-password', {
			method: 'POST',
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('throws the server error message on failure', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'not found' }, false, 404));

		await expect(resetPassword('user-1', 'token-123', fetchFn)).rejects.toThrow('not found');
	});
});
