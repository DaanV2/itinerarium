import type { Document, JournalEntry } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

/** Lists a character's journal entries. The API returns 404 for a character
 * the caller may not see — surface that as not-found, don't special-case it. */
export async function listJournalEntries(
	characterId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<JournalEntry[]> {
	const res = await fetchFn(`/api/characters/${characterId}/journal`, {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list journal entries: ${res.status}`));
	}

	return (await res.json()) as JournalEntry[];
}

/** Adds a journal entry to a character, stamped with its current game day. */
export async function createJournalEntry(
	characterId: string,
	content: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<JournalEntry> {
	const res = await fetchFn(`/api/characters/${characterId}/journal`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({ content })
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to create journal entry: ${res.status}`));
	}

	return (await res.json()) as JournalEntry;
}

/** Edits a journal entry's content. The game day it was stamped with never
 * changes. */
export async function updateJournalEntry(
	characterId: string,
	entryId: string,
	content: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<JournalEntry> {
	const res = await fetchFn(`/api/characters/${characterId}/journal/${entryId}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({ content })
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to update journal entry: ${res.status}`));
	}

	return (await res.json()) as JournalEntry;
}

/** Copies a journal entry into a new document in the character's personal
 * repository. The document starts private; the journal entry itself is left
 * untouched — this is a copy, not a move. */
export async function convertJournalEntry(
	characterId: string,
	entryId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Document> {
	const res = await fetchFn(`/api/characters/${characterId}/journal/${entryId}/convert`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to convert journal entry: ${res.status}`));
	}

	return (await res.json()) as Document;
}
