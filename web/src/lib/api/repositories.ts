import type { FolderTreeNode, Repository } from '$lib/types';

async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body: unknown = await res.json().catch(() => null);
	return body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
		? body.error
		: fallback;
}

/** Lists the repositories the caller may see: every one for a GM, only the
 * general/template singletons plus the caller's own character and group
 * repositories for a player. */
export async function listRepositories(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Repository[]> {
	const res = await fetchFn('/api/repositories', {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to list repositories: ${res.status}`));
	}

	return (await res.json()) as Repository[];
}

/** Fetches one repository. A 404 may simply mean "no access" — surface it as
 * not-found, don't special-case it. */
export async function getRepository(
	id: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Repository> {
	const res = await fetchFn(`/api/repositories/${id}`, {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to load repository: ${res.status}`));
	}

	return (await res.json()) as Repository;
}

/** Fetches a repository's documents as a folder tree, sorted alphabetically
 * at every level. Folders with no accessible documents never appear. */
export async function getDocumentFolderTree(
	repositoryId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<FolderTreeNode> {
	const res = await fetchFn(`/api/repositories/${repositoryId}/documents/tree`, {
		headers: { Authorization: `Bearer ${token}` }
	});

	if (!res.ok) {
		throw new Error(await errorMessage(res, `failed to load folder tree: ${res.status}`));
	}

	return (await res.json()) as FolderTreeNode;
}
