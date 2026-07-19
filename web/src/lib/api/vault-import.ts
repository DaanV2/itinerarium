import type { VaultImportFile, VaultImportFileResult } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

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
	const res = await fetchFn('/api/import/obsidian', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({ repository_id: repositoryId, files })
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `import failed: ${res.status}`));
	}

	const body = (await res.json()) as { results: VaultImportFileResult[] };
	return body.results;
}
