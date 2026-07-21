import type { VaultImportFile, VaultImportFileResult } from '$lib/types';
import { apiFetch } from './client';

/** Imports a batch of Obsidian vault files as documents. Files whose
 * frontmatter names a `repository` go there; the rest go to `repositoryId`.
 * Each file is reported individually — collisions come back as status
 * `collision` so the caller can offer rename-or-continue per file, and one
 * bad file never aborts the batch. */
export async function importVault(
	repositoryId: string,
	files: VaultImportFile[],
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<VaultImportFileResult[]> {
	const body = await apiFetch<{ results: VaultImportFileResult[] }>('/api/import/obsidian', {
		method: 'POST',
		token,
		body: { repository_id: repositoryId, files },
		errorContext: 'import failed',
		fetchFn
	});
	return body.results;
}
