import type { ActivityEntry, AnnouncementInput } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

/** Lists one character's activity feed — the events visible to that character
 * up to its current game day. The API returns 404 for a character the caller
 * may not see; on announced entries the actor is already stripped server-side
 * for players. */
export async function listCharacterActivity(
	characterId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<ActivityEntry[]> {
	const res = await fetchFn(`/api/characters/${characterId}/activity`, {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list activity: ${res.status}`));
	}

	return (await res.json()) as ActivityEntry[];
}

/** Lists the full campaign log, announcement targets included. GM only. */
export async function listAllActivity(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<ActivityEntry[]> {
	const res = await fetchFn('/api/activity', {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list activity: ${res.status}`));
	}

	return (await res.json()) as ActivityEntry[];
}

/** Broadcasts an announced activity entry to specific characters, groups, or
 * everyone. GM only. */
export async function announceActivity(
	input: AnnouncementInput,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<ActivityEntry> {
	const res = await fetchFn('/api/activity/announcements', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to announce: ${res.status}`));
	}

	return (await res.json()) as ActivityEntry;
}
