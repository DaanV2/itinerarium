import type { Character, Location, LocationAccess } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

/** Lists the locations the caller may see: all of them for a GM, only
 * accessible ones for a player. */
export async function listLocations(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Location[]> {
	const res = await fetchFn('/api/locations', {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list locations: ${res.status}`));
	}

	return (await res.json()) as Location[];
}

/** Fetches one location. A 404 may simply mean "no access" — surface it as
 * not-found, don't special-case it. */
export async function getLocation(
	id: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Location> {
	const res = await fetchFn(`/api/locations/${id}`, {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to load location: ${res.status}`));
	}

	return (await res.json()) as Location;
}

/** Creates a location. GM only. */
export async function createLocation(
	input: { name: string; description?: string; plane?: string },
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Location> {
	const res = await fetchFn('/api/locations', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to create location: ${res.status}`));
	}

	return (await res.json()) as Location;
}

/** Edits a location — anyone who can see it can edit it. */
export async function updateLocation(
	id: string,
	input: { name?: string; description?: string; plane?: string },
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Location> {
	const res = await fetchFn(`/api/locations/${id}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(input)
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to update location: ${res.status}`));
	}

	return (await res.json()) as Location;
}

/** Lists a location's access grants. GM only. */
export async function listLocationAccess(
	locationId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<LocationAccess[]> {
	const res = await fetchFn(`/api/locations/${locationId}/access`, {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list access: ${res.status}`));
	}

	return (await res.json()) as LocationAccess[];
}

/** Grants a character or group access to a location. GM only. */
export async function grantLocationAccess(
	locationId: string,
	target: { character_id?: string; group_id?: string },
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<LocationAccess> {
	const res = await fetchFn(`/api/locations/${locationId}/access`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify(target)
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to grant access: ${res.status}`));
	}

	return (await res.json()) as LocationAccess;
}

/** Removes one access grant from a location. GM only. */
export async function revokeLocationAccess(
	locationId: string,
	accessId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<void> {
	const res = await fetchFn(`/api/locations/${locationId}/access/${accessId}`, {
		method: 'DELETE',
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to revoke access: ${res.status}`));
	}
}

/** Places a character at a location the character can see (GMs anywhere). */
export async function setCharacterLocation(
	characterId: string,
	locationId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Character> {
	const res = await fetchFn(`/api/characters/${characterId}/location`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({ location_id: locationId })
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to set location: ${res.status}`));
	}

	return (await res.json()) as Character;
}

/** Clears a character's location association. */
export async function clearCharacterLocation(
	characterId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Character> {
	const res = await fetchFn(`/api/characters/${characterId}/location`, {
		method: 'DELETE',
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to clear location: ${res.status}`));
	}

	return (await res.json()) as Character;
}
