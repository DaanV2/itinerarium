import type { FolderTreeNode, Repository } from '$lib/types';
import { apiFetch } from './client';

/** Lists the repositories the caller may see: every one for a GM, only the
 * general/template singletons plus the caller's own character and group
 * repositories for a player. */
export async function listRepositories(
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Repository[]> {
	return apiFetch<Repository[]>('/api/repositories', {
		token,
		errorContext: 'failed to list repositories',
		fetchFn
	});
}

/** Fetches one repository. A 404 may simply mean "no access" — surface it as
 * not-found, don't special-case it. */
export async function getRepository(
	id: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<Repository> {
	return apiFetch<Repository>(`/api/repositories/${id}`, {
		token,
		errorContext: 'failed to load repository',
		fetchFn
	});
}

/** Fetches a repository's documents as a folder tree, sorted alphabetically
 * at every level. Folders with no accessible documents never appear. */
export async function getDocumentFolderTree(
	repositoryId: string,
	token: string,
	fetchFn: typeof fetch = fetch
): Promise<FolderTreeNode> {
	return apiFetch<FolderTreeNode>(`/api/repositories/${repositoryId}/documents/tree`, {
		token,
		errorContext: 'failed to load folder tree',
		fetchFn
	});
}
