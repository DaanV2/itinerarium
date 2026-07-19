import type { SearchResult } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

/** Runs a full-text search over document titles, paths, tags, and content.
 * The server filters by access before returning anything — a document (or
 * GM-only section) the caller cannot see is never matched, not even as a hit
 * count. */
export async function searchDocuments(
	query: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<SearchResult[]> {
	const res = await fetchFn(`/api/search?q=${encodeURIComponent(query)}`, {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `search failed: ${res.status}`));
	}

	return (await res.json()) as SearchResult[];
}
