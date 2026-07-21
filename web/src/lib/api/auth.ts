import type { LoginResult } from '$lib/types';
import { apiFetch } from './client';

/** Logs in with an email + password pair, returning the account and a signed access token. */
export async function login(
	email: string,
	password: string,
	fetchFn: typeof fetch = fetch
): Promise<LoginResult> {
	return apiFetch<LoginResult>('/api/login', {
		method: 'POST',
		body: { email, password },
		errorContext: 'login failed',
		fetchFn
	});
}
