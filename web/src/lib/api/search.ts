import type { SearchResult } from '$lib/types';
import { apiFetch } from './client';

/** Runs a full-text search over document titles, paths, tags, and content.
 * The server filters by access before returning anything — a document (or
 * GM-only section) the caller cannot see is never matched, not even as a hit
 * count. */
export async function searchDocuments(
	query: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<SearchResult[]> {
	return apiFetch<SearchResult[]>(`/api/search?q=${encodeURIComponent(query)}`, {
		token,
		errorContext: 'search failed',
		fetchFn
	});
}
