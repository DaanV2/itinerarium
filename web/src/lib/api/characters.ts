import type { Character } from '$lib/types';
import { apiFetch } from './client';

/** Lists the caller's own characters, or every character for a GM. */
export async function listCharacters(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Character[]> {
	return apiFetch<Character[]>('/api/characters', {
		token,
		errorContext: 'failed to list characters',
		fetchFn
	});
}

/** Fetches a single character. The API returns 404 for characters the caller
 * may not see — surface that as not-found, don't special-case it. */
export async function getCharacter(
	id: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Character> {
	return apiFetch<Character>(`/api/characters/${id}`, {
		token,
		errorContext: 'failed to load character',
		fetchFn
	});
}

/** Creates a new character for the caller. */
export async function createCharacter(
	name: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Character> {
	return apiFetch<Character>('/api/characters', {
		method: 'POST',
		token,
		body: { name },
		errorContext: 'failed to create character',
		fetchFn
	});
}
