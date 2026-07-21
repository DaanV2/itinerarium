// Pure helpers for the Obsidian vault import page: repository labelling, the
// vault-relative path derivation, and turning the API's per-file results into
// display rows with a running summary. The `.svelte` file keeps only the DOM
// bits (reading the picked files) and the fetch calls.

import type { Character, Group, Repository, VaultImportFileResult } from '$lib/types';

/** A picked `.md` file ready to import: its vault-relative path and contents. */
export interface PendingFile {
	path: string;
	markdown: string;
}

/** One import result plus the per-row UI state: a rename target offered on a
 * collision (starts as the original path, minus the `.md`) and a busy flag. */
export interface FileRow extends VaultImportFileResult {
	newPath: string;
	busy: boolean;
}

/** Human label for a repository in the "default repository" picker. */
export function repositoryLabel(
	repo: Repository,
	characters: Character[],
	groups: Group[]
): string {
	switch (repo.type) {
		case 'general':
			return 'General';
		case 'template':
			return 'Templates';
		case 'group': {
			const group = groups.find((g) => g.id === repo.group_id);
			return group ? `${group.name} (group)` : 'Group repository';
		}
		case 'character': {
			const character = characters.find((c) => c.id === repo.character_id);
			return character ? `${character.name} (character)` : 'Character repository';
		}
	}
}

/** Picks the default repository to preselect: the general singleton, else the
 * first available, else none. */
export function defaultRepositoryId(repositories: Repository[]): string {
	return repositories.find((r) => r.type === 'general')?.id ?? repositories[0]?.id ?? '';
}

/** The path a picked file should import to: its `webkitRelativePath` with the
 * vault's own root folder dropped (so the document tree mirrors the vault
 * layout), falling back to the bare file name when there's no folder. */
export function vaultRelativePath(fileName: string, webkitRelativePath?: string): string {
	if (webkitRelativePath && webkitRelativePath.includes('/')) {
		return webkitRelativePath.split('/').slice(1).join('/');
	}
	return fileName;
}

/** Drops a trailing `.md` (any case) — the default rename target on a collision
 * and the tidy path a document is stored under. */
export function stripMdExtension(path: string): string {
	return path.replace(/\.md$/i, '');
}

/** Turns the API's per-file results into display rows, seeding each rename
 * target with the collision-free path. */
export function toResultRows(results: VaultImportFileResult[]): FileRow[] {
	return results.map((r) => ({ ...r, newPath: stripMdExtension(r.path), busy: false }));
}

/** Running counts across the result rows for the summary line. */
export interface ImportSummary {
	imported: number;
	collisions: number;
	failed: number;
}

export function summarizeRows(rows: FileRow[]): ImportSummary {
	return {
		imported: rows.filter((r) => r.status === 'imported').length,
		collisions: rows.filter((r) => r.status === 'collision').length,
		failed: rows.filter((r) => r.status === 'error').length
	};
}
