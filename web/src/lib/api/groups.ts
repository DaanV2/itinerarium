import type { Group, GroupType } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

/** Lists every group with its members. Visible to any authenticated user. */
export async function listGroups(token: string, fetchFn: typeof fetch = fetch): Promise<Group[]> {
	const res = await fetchFn('/api/groups', {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list groups: ${res.status}`));
	}

	return (await res.json()) as Group[];
}

/** Fetches one group with its members. */
export async function getGroup(
	id: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Group> {
	const res = await fetchFn(`/api/groups/${id}`, {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to load group: ${res.status}`));
	}

	return (await res.json()) as Group;
}

/** Creates a group. GM only. */
export async function createGroup(
	input: { name: string; type: GroupType; description?: string },
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Group> {
	const res = await fetchFn('/api/groups', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to create group: ${res.status}`));
	}

	return (await res.json()) as Group;
}

/** Joins one of the caller's characters (or any character, for a GM) to a
 * group. The join is recorded stamped with the character's game day. */
export async function joinGroup(
	groupId: string,
	characterId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<void> {
	const res = await fetchFn(`/api/groups/${groupId}/members`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({ character_id: characterId })
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to join group: ${res.status}`));
	}
}

/** Removes a character from a group under the same ownership rule as join. */
export async function leaveGroup(
	groupId: string,
	characterId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<void> {
	const res = await fetchFn(`/api/groups/${groupId}/members/${characterId}`, {
		method: 'DELETE',
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to leave group: ${res.status}`));
	}
}
