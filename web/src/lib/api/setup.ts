import type { InitialAccount, SetupStatus } from '$lib/types';

/** Reports whether this installation still needs its first-run GM setup. */
export async function getSetupStatus(fetchFn: typeof fetch = fetch): Promise<SetupStatus> {
	const res = await fetchFn('/api/setup');
	if (!res.ok) {
		throw new Error(`failed to check setup status: ${res.status}`);
	}

	return (await res.json()) as SetupStatus;
}

/** Runs the first-run wizard, creating the installation's sole initial GM account. */
export async function createInitialAccount(
	email: string,
	password: string,
	fetchFn: typeof fetch = fetch
): Promise<InitialAccount> {
	const res = await fetchFn('/api/setup', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email, password })
	});

	if (!res.ok) {
		const body: unknown = await res.json().catch(() => null);
		const message =
			body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
				? body.error
				: `setup failed: ${res.status}`;
		throw new Error(message);
	}

	return (await res.json()) as InitialAccount;
}
