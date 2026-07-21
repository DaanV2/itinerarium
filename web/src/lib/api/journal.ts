import type { Document, JournalEntry } from '$lib/types';
import { apiFetch } from './client';

/** Lists a character's journal entries. The API returns 404 for a character
 * the caller may not see — surface that as not-found, don't special-case it. */
export async function listJournalEntries(
	characterId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<JournalEntry[]> {
	return apiFetch<JournalEntry[]>(`/api/characters/${characterId}/journal`, {
		token,
		errorContext: 'failed to list journal entries',
		fetchFn
	});
}

/** Adds a journal entry to a character, stamped with its current game day. */
export async function createJournalEntry(
	characterId: string,
	content: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<JournalEntry> {
	return apiFetch<JournalEntry>(`/api/characters/${characterId}/journal`, {
		method: 'POST',
		token,
		body: { content },
		errorContext: 'failed to create journal entry',
		fetchFn
	});
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
	return apiFetch<JournalEntry>(`/api/characters/${characterId}/journal/${entryId}`, {
		method: 'PATCH',
		token,
		body: { content },
		errorContext: 'failed to update journal entry',
		fetchFn
	});
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
	return apiFetch<Document>(`/api/characters/${characterId}/journal/${entryId}/convert`, {
		method: 'POST',
		token,
		errorContext: 'failed to convert journal entry',
		fetchFn
	});
}
