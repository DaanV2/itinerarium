import type { Location } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

/** Lists every campaign location. Any authenticated user may read them; the
 * caller builds the plane/place tree from the `parent_id` links. */
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

/** Creates a location (GM only). Omit `parentId` to create a top-level plane;
 * pass an existing location's id to nest under it. */
export async function createLocation(
	input: { name: string; description?: string; parentId?: string },
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Location> {
	const res = await fetchFn('/api/locations', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({
			name: input.name,
			description: input.description,
			parent_id: input.parentId
		})
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to create location: ${res.status}`));
	}

	return (await res.json()) as Location;
}

/** Deletes a location (GM only). The API returns 409 when the location still
 * has nested locations — remove or re-home the children first. */
export async function deleteLocation(
	id: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<void> {
	const res = await fetchFn(`/api/locations/${id}`, {
		method: 'DELETE',
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to delete location: ${res.status}`));
	}
}
