import type { InitialAccount, SetupStatus } from '$lib/types';
import { apiFetch } from './client';

/** Reports whether this installation still needs its first-run GM setup. */
export async function getSetupStatus(fetchFn: typeof fetch = fetch): Promise<SetupStatus> {
	return apiFetch<SetupStatus>('/api/setup', {
		errorContext: 'failed to check setup status',
		fetchFn
	});
}

/** Runs the first-run wizard, creating the installation's sole initial GM account. */
export async function createInitialAccount(
	email: string,
	password: string,
	fetchFn: typeof fetch = fetch
): Promise<InitialAccount> {
	return apiFetch<InitialAccount>('/api/setup', {
		method: 'POST',
		body: { email, password },
		errorContext: 'setup failed',
		fetchFn
	});
}
