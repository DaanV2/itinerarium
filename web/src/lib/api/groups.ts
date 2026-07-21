import type { Group, GroupType } from '$lib/types';
import { apiFetch, apiSend } from './client';

/** Lists every group with its members. Visible to any authenticated user. */
export async function listGroups(token: string, fetchFn: typeof fetch = fetch): Promise<Group[]> {
	return apiFetch<Group[]>('/api/groups', {
		token,
		errorContext: 'failed to list groups',
		fetchFn
	});
}

/** Fetches one group with its members. */
export async function getGroup(
	id: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Group> {
	return apiFetch<Group>(`/api/groups/${id}`, {
		token,
		errorContext: 'failed to load group',
		fetchFn
	});
}

/** Creates a group. GM only. */
export async function createGroup(
	input: { name: string; type: GroupType; description?: string },
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Group> {
	return apiFetch<Group>('/api/groups', {
		method: 'POST',
		token,
		body: input,
		errorContext: 'failed to create group',
		fetchFn
	});
}

/** Joins one of the caller's characters (or any character, for a GM) to a
 * group. The join is recorded stamped with the character's game day. */
export async function joinGroup(
	groupId: string,
	characterId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<void> {
	await apiSend(`/api/groups/${groupId}/members`, {
		method: 'POST',
		token,
		body: { character_id: characterId },
		errorContext: 'failed to join group',
		fetchFn
	});
}

/** Removes a character from a group under the same ownership rule as join. */
export async function leaveGroup(
	groupId: string,
	characterId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<void> {
	await apiSend(`/api/groups/${groupId}/members/${characterId}`, {
		method: 'DELETE',
		token,
		errorContext: 'failed to leave group',
		fetchFn
	});
}
