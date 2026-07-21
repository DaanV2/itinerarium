import type { Document, DocumentSection, DocumentShare } from '$lib/types';
import { ApiError, apiFetch } from './client';

/** Thrown by {@link updateDocument} for the two editor warnings, which the
 * API reports as 409 with a machine-readable `code` (docs/architecture.md).
 * Callers branch on `code` to offer "rename or continue" / "overwrite
 * anyway" instead of treating this as a generic failure. Extends
 * {@link ApiError} so it still carries the HTTP `status`. */
export class DocumentConflictError extends ApiError {
	code: 'path_collision' | 'concurrent_edit';

	constructor(code: 'path_collision' | 'concurrent_edit', message: string) {
		super(message, 409, code);
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
	return apiFetch<Document>(`/api/documents/${id}`, {
		token,
		errorContext: 'failed to load document',
		fetchFn
	});
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
	try {
		return await apiFetch<Document>(`/api/documents/${id}`, {
			method: 'PATCH',
			token,
			body: {
				path: input.path,
				title: input.title,
				tags: input.tags,
				shared_on_game_day: input.sharedOnGameDay,
				sections: input.sections,
				expected_version: input.expectedVersion,
				force: input.force ?? false,
				allow_collision: input.allowCollision ?? false
			},
			errorContext: 'failed to update document',
			fetchFn
		});
	} catch (err) {
		if (
			err instanceof ApiError &&
			err.status === 409 &&
			(err.code === 'path_collision' || err.code === 'concurrent_edit')
		) {
			throw new DocumentConflictError(err.code, err.message);
		}
		throw err;
	}
}

/** Lists the direct character shares on a document. GM only. */
export async function listDocumentShares(
	id: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<DocumentShare[]> {
	return apiFetch<DocumentShare[]>(`/api/documents/${id}/shares`, {
		token,
		errorContext: 'failed to list document shares',
		fetchFn
	});
}
