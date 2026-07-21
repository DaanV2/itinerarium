import type { ActivityEntry, AnnouncementInput } from '$lib/types';
import { apiFetch } from './client';

/** Lists one character's activity feed — the events visible to that character
 * up to its current game day. The API returns 404 for a character the caller
 * may not see; on announced entries the actor is already stripped server-side
 * for players. */
export async function listCharacterActivity(
	characterId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<ActivityEntry[]> {
	return apiFetch<ActivityEntry[]>(`/api/characters/${characterId}/activity`, {
		token,
		errorContext: 'failed to list activity',
		fetchFn
	});
}

/** Lists the full campaign log, announcement targets included. GM only. */
export async function listAllActivity(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<ActivityEntry[]> {
	return apiFetch<ActivityEntry[]>('/api/activity', {
		token,
		errorContext: 'failed to list activity',
		fetchFn
	});
}

/** Broadcasts an announced activity entry to specific characters, groups, or
 * everyone. GM only. */
export async function announceActivity(
	input: AnnouncementInput,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<ActivityEntry> {
	return apiFetch<ActivityEntry>('/api/activity/announcements', {
		method: 'POST',
		token,
		body: input,
		errorContext: 'failed to announce',
		fetchFn
	});
}
