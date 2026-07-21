import type { Account, CreatedAccount, ResetPasswordResult, Role } from '$lib/types';
import { apiFetch } from './client';

/** Lists every account. GM-only. */
export async function listAccounts(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Account[]> {
	return apiFetch<Account[]>('/api/admin/users', {
		token,
		errorContext: 'failed to list accounts',
		fetchFn
	});
}

/** Creates a new account with a random temporary password. GM-only. */
export async function createAccount(
	email: string,
	role: Role,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<CreatedAccount> {
	return apiFetch<CreatedAccount>('/api/admin/users', {
		method: 'POST',
		token,
		body: { email, role },
		errorContext: 'failed to create account',
		fetchFn
	});
}

/** Resets an account's password to a new random temporary password. GM-only, no SMTP. */
export async function resetPassword(
	userId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<ResetPasswordResult> {
	return apiFetch<ResetPasswordResult>(`/api/admin/users/${userId}/reset-password`, {
		method: 'POST',
		token,
		errorContext: 'failed to reset password',
		fetchFn
	});
}
