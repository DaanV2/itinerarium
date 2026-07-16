import type { Document } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

/** Fetches one document with the sections the caller may see — GM-only
 * sections are omitted entirely from the response for a player, not just
 * hidden client-side. A 404 may simply mean "no access". */
export async function getDocument(
	id: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Document> {
	const res = await fetchFn(`/api/documents/${id}`, {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to load document: ${res.status}`));
	}

	return (await res.json()) as Document;
}
