import type { Account, CreatedAccount, ResetPasswordResult, Role } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

/** Lists every account. GM-only. */
export async function listAccounts(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Account[]> {
	const res = await fetchFn('/api/admin/users', {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list accounts: ${res.status}`));
	}

	return (await res.json()) as Account[];
}

/** Creates a new account with a random temporary password. GM-only. */
export async function createAccount(
	email: string,
	role: Role,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<CreatedAccount> {
	const res = await fetchFn('/api/admin/users', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({ email, role })
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to create account: ${res.status}`));
	}

	return (await res.json()) as CreatedAccount;
}

/** Resets an account's password to a new random temporary password. GM-only, no SMTP. */
export async function resetPassword(
	userId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<ResetPasswordResult> {
	const res = await fetchFn(`/api/admin/users/${userId}/reset-password`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to reset password: ${res.status}`));
	}

	return (await res.json()) as ResetPasswordResult;
}
