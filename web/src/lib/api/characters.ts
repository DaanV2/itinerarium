import type { Character } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

/** Lists the caller's own characters, or every character for a GM. */
export async function listCharacters(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Character[]> {
	const res = await fetchFn('/api/characters', {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list characters: ${res.status}`));
	}

	return (await res.json()) as Character[];
}

/** Creates a new character for the caller. */
export async function createCharacter(
	name: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Character> {
	const res = await fetchFn('/api/characters', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({ name })
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to create character: ${res.status}`));
	}

	return (await res.json()) as Character;
}
