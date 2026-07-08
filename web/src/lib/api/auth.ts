import type { LoginResult } from '$lib/types';

/** Logs in with an email + password pair, returning the account and a signed access token. */
export async function login(
	email: string,
	password: string,
	fetchFn: typeof fetch = fetch
): Promise<LoginResult> {
	const res = await fetchFn('/api/login', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email, password })
	});

	if (!res.ok) {
		const body: unknown = await res.json().catch(() => null);
		const message =
			body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
				? body.error
				: `login failed: ${res.status}`;
		throw new Error(message);
	}

	return (await res.json()) as LoginResult;
}
