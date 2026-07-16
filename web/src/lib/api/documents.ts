import type { Document, DocumentSection } from '$lib/types';

async function errorBody(res: Response): Promise<{ error?: string; code?: string } | null> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' ? (body as { error?: string; code?: string }) : null;
}

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body = await errorBody(res);
	return body && typeof body.error === 'string' ? body.error : fallback;
}

/** Thrown by {@link updateDocument} for the two editor warnings, which the
 * API reports as 409 with a machine-readable `code` (docs/architecture.md).
 * Callers branch on `code` to offer "rename or continue" / "overwrite
 * anyway" instead of treating this as a generic failure. */
export class DocumentConflictError extends Error {
	code: 'path_collision' | 'concurrent_edit';

	constructor(code: 'path_collision' | 'concurrent_edit', message: string) {
		super(message);
		this.name = 'DocumentConflictError';
		this.code = code;
	}
}

export interface UpdateDocumentInput {
	path: string;
	title: string;
	tags: string[];
	sharedOnGameDay: number;
	sections: DocumentSection[];
	expectedVersion: number;
	force?: boolean;
	allowCollision?: boolean;
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

/** Saves a document's metadata and sections. Echoes back the version the
 * editor loaded as `expected_version` — if the document changed since then,
 * the API returns 409 `concurrent_edit` (thrown as {@link DocumentConflictError})
 * instead of silently overwriting the other edit. Pass `force: true` to save
 * anyway once the user has confirmed. */
export async function updateDocument(
	id: string,
	input: UpdateDocumentInput,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Document> {
	const res = await fetchFn(`/api/documents/${id}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({
			path: input.path,
			title: input.title,
			tags: input.tags,
			shared_on_game_day: input.sharedOnGameDay,
			sections: input.sections,
			expected_version: input.expectedVersion,
			force: input.force ?? false,
			allow_collision: input.allowCollision ?? false
		})
	});

	if (!res.ok) {
		if (res.status === 409) {
			const body = await errorBody(res);
			if (body?.code === 'path_collision' || body?.code === 'concurrent_edit') {
				throw new DocumentConflictError(body.code, body.error ?? 'conflict saving document');
			}
		}
		throw new Error(await errorMessage(res, `failed to update document: ${res.status}`));
	}

	return (await res.json()) as Document;
}
