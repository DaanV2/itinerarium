import { describe, it, expect } from 'vitest';
import {
	repositoryLabel,
	defaultRepositoryId,
	vaultRelativePath,
	stripMdExtension,
	toResultRows,
	summarizeRows
} from './vault-import-view';
import type { Character, Group, Repository, VaultImportFileResult } from './types';

const characters = [{ id: 'c1', name: 'Aria' }] as Character[];
const groups = [{ id: 'g1', name: 'The Guild' }] as Group[];

describe('repositoryLabel', () => {
	it('labels the singletons', () => {
		expect(repositoryLabel({ type: 'general' } as Repository, characters, groups)).toBe('General');
		expect(repositoryLabel({ type: 'template' } as Repository, characters, groups)).toBe(
			'Templates'
		);
	});

	it('labels group and character repositories by their owner name', () => {
		expect(
			repositoryLabel({ type: 'group', group_id: 'g1' } as Repository, characters, groups)
		).toBe('The Guild (group)');
		expect(
			repositoryLabel({ type: 'character', character_id: 'c1' } as Repository, characters, groups)
		).toBe('Aria (character)');
	});

	it('falls back when the owner is unknown', () => {
		expect(
			repositoryLabel({ type: 'group', group_id: 'ghost' } as Repository, characters, groups)
		).toBe('Group repository');
	});
});

describe('defaultRepositoryId', () => {
	it('prefers the general repository', () => {
		const repos = [
			{ id: 'r1', type: 'template' },
			{ id: 'r2', type: 'general' }
		] as Repository[];
		expect(defaultRepositoryId(repos)).toBe('r2');
	});

	it('falls back to the first, then to empty', () => {
		expect(defaultRepositoryId([{ id: 'r1', type: 'character' }] as Repository[])).toBe('r1');
		expect(defaultRepositoryId([])).toBe('');
	});
});

describe('vaultRelativePath', () => {
	it('drops the vault root folder from the path', () => {
		expect(vaultRelativePath('origins.md', 'MyVault/lore/origins.md')).toBe('lore/origins.md');
	});

	it('falls back to the file name when there is no folder', () => {
		expect(vaultRelativePath('note.md', 'note.md')).toBe('note.md');
		expect(vaultRelativePath('note.md', undefined)).toBe('note.md');
	});
});

describe('stripMdExtension', () => {
	it('drops a trailing .md of any case', () => {
		expect(stripMdExtension('lore/origins.md')).toBe('lore/origins');
		expect(stripMdExtension('lore/origins.MD')).toBe('lore/origins');
		expect(stripMdExtension('lore/origins')).toBe('lore/origins');
	});
});

describe('toResultRows / summarizeRows', () => {
	const results: VaultImportFileResult[] = [
		{ path: 'a.md', status: 'imported', document_id: 'd1' },
		{ path: 'b.md', status: 'collision', error: 'exists' },
		{ path: 'c.md', status: 'error', error: 'bad' }
	];

	it('seeds each row with a stripped rename target and not busy', () => {
		expect(toResultRows(results)).toEqual([
			{ path: 'a.md', status: 'imported', document_id: 'd1', newPath: 'a', busy: false },
			{ path: 'b.md', status: 'collision', error: 'exists', newPath: 'b', busy: false },
			{ path: 'c.md', status: 'error', error: 'bad', newPath: 'c', busy: false }
		]);
	});

	it('counts by status', () => {
		expect(summarizeRows(toResultRows(results))).toEqual({
			imported: 1,
			collisions: 1,
			failed: 1
		});
	});
});
