import type { Character, Location, LocationAccess, LocationSummary } from '$lib/types';
import { apiFetch, apiSend } from './client';

/** Lists the locations the caller may see: all of them for a GM, only
 * accessible ones for a player. */
export async function listLocations(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<LocationSummary[]> {
	return apiFetch<LocationSummary[]>('/api/locations', {
		token,
		errorContext: 'failed to list locations',
		fetchFn
	});
}

/** Fetches one location. A 404 may simply mean "no access" — surface it as
 * not-found, don't special-case it. */
export async function getLocation(
	id: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Location> {
	return apiFetch<Location>(`/api/locations/${id}`, {
		token,
		errorContext: 'failed to load location',
		fetchFn
	});
}

/** Creates a location. GM only. */
export async function createLocation(
	input: { name: string; plane?: string },
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Location> {
	return apiFetch<Location>('/api/locations', {
		method: 'POST',
		token,
		body: input,
		errorContext: 'failed to create location',
		fetchFn
	});
}

/** One description-section edit in an update payload. Omit `id` for a new
 * section; only a GM may set `gm_only`. */
export interface LocationSectionInput {
	id?: string;
	content: string;
	gm_only?: boolean;
}

/** Edits a location — anyone who can see it can edit it. `sections`, when
 * given, replaces the caller's visible description sections (players can
 * never touch GM-only ones); omit it to leave the description untouched.
 * Only a GM may set `shared_on_game_day`. */
export async function updateLocation(
	id: string,
	input: {
		name?: string;
		plane?: string;
		shared_on_game_day?: number;
		sections?: LocationSectionInput[];
	},
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Location> {
	return apiFetch<Location>(`/api/locations/${id}`, {
		method: 'PATCH',
		token,
		body: input,
		errorContext: 'failed to update location',
		fetchFn
	});
}

/** Lists a location's access grants. GM only. */
export async function listLocationAccess(
	locationId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<LocationAccess[]> {
	return apiFetch<LocationAccess[]>(`/api/locations/${locationId}/access`, {
		token,
		errorContext: 'failed to list access',
		fetchFn
	});
}

/** Grants a character or group access to a location. GM only. */
export async function grantLocationAccess(
	locationId: string,
	target: { character_id?: string; group_id?: string },
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<LocationAccess> {
	return apiFetch<LocationAccess>(`/api/locations/${locationId}/access`, {
		method: 'POST',
		token,
		body: target,
		errorContext: 'failed to grant access',
		fetchFn
	});
}

/** Removes one access grant from a location. GM only. */
export async function revokeLocationAccess(
	locationId: string,
	accessId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<void> {
	await apiSend(`/api/locations/${locationId}/access/${accessId}`, {
		method: 'DELETE',
		token,
		errorContext: 'failed to revoke access',
		fetchFn
	});
}

/** Places a character at a location the character can see (GMs anywhere). */
export async function setCharacterLocation(
	characterId: string,
	locationId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Character> {
	return apiFetch<Character>(`/api/characters/${characterId}/location`, {
		method: 'PUT',
		token,
		body: { location_id: locationId },
		errorContext: 'failed to set location',
		fetchFn
	});
}

/** Clears a character's location association. */
export async function clearCharacterLocation(
	characterId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Character> {
	return apiFetch<Character>(`/api/characters/${characterId}/location`, {
		method: 'DELETE',
		token,
		errorContext: 'failed to clear location',
		fetchFn
	});
}
