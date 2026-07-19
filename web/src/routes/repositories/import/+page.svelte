<script lang="ts">
	import { resolve } from '$app/paths';
	import { listRepositories } from '$lib/api/repositories';
	import { listCharacters } from '$lib/api/characters';
	import { listGroups } from '$lib/api/groups';
	import { importVault } from '$lib/api/vault-import';
	import { getAccessToken } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import type { Character, Group, Repository, VaultImportFileResult } from '$lib/types';
	import { onMount } from 'svelte';

	interface PendingFile {
		path: string;
		markdown: string;
	}

	interface FileRow extends VaultImportFileResult {
		/** Rename target offered on a collision; starts as the original path. */
		newPath: string;
		busy: boolean;
	}

	let repositories = $state<Repository[]>([]);
	let characters = $state<Character[]>([]);
	let groups = $state<Group[]>([]);
	let repositoryId = $state('');
	let pending = $state<PendingFile[]>([]);
	let rows = $state<FileRow[]>([]);
	let loading = $state(true);
	let importing = $state(false);
	let error = $state('');

	function repositoryLabel(repo: Repository): string {
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

	async function loadAll() {
		loading = true;
		const token = getAccessToken();
		try {
			[repositories, characters, groups] = await Promise.all([
				listRepositories(token),
				listCharacters(token),
				listGroups(token)
			]);
			repositoryId =
				repositories.find((r) => r.type === 'general')?.id ?? repositories[0]?.id ?? '';
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load repositories.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAll);

	/** Reads the picked vault folder: keeps .md files only and drops the
	 * vault's own root folder from each path, so the document tree mirrors the
	 * vault layout. */
	async function onFilesPicked(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const picked = Array.from(input.files ?? []).filter((f) =>
			f.name.toLowerCase().endsWith('.md')
		);

		const files: PendingFile[] = [];
		for (const file of picked) {
			const relative = (file as File & { webkitRelativePath?: string }).webkitRelativePath;
			const path =
				relative && relative.includes('/') ? relative.split('/').slice(1).join('/') : file.name;
			files.push({ path, markdown: await file.text() });
		}

		pending = files;
		rows = [];
		error = '';
	}

	function markdownFor(path: string): string | undefined {
		return pending.find((f) => f.path === path)?.markdown;
	}

	async function runImport() {
		if (!repositoryId || pending.length === 0) {
			return;
		}

		importing = true;
		try {
			const results = await importVault(
				repositoryId,
				pending.map((f) => ({ path: f.path, markdown: f.markdown })),
				getAccessToken()
			);
			rows = results.map((r) => ({ ...r, newPath: r.path.replace(/\.md$/i, ''), busy: false }));
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Import failed.';
		} finally {
			importing = false;
		}
	}

	/** Retries one collided file: with its (possibly renamed) path, or with
	 * allow_collision set to continue onto the occupied path. */
	async function retryFile(row: FileRow, allowCollision: boolean) {
		const markdown = markdownFor(row.path);
		if (markdown === undefined) {
			return;
		}

		row.busy = true;
		try {
			const [result] = await importVault(
				repositoryId,
				[{ path: row.newPath, markdown, allow_collision: allowCollision }],
				getAccessToken()
			);
			rows = rows.map((r) =>
				r.path === row.path ? { ...result, path: row.path, newPath: row.newPath, busy: false } : r
			);
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Import failed.';
			row.busy = false;
		}
	}

	let collisions = $derived(rows.filter((r) => r.status === 'collision').length);
	let imported = $derived(rows.filter((r) => r.status === 'imported').length);
	let failed = $derived(rows.filter((r) => r.status === 'error').length);
</script>

<main class="main-page">
	<h1>Import Obsidian vault</h1>

	<p class="hint">
		Pick your vault folder — every <code>.md</code> file is imported as a document, folders map to
		the document path, and YAML frontmatter (<code>title</code>, <code>tags</code>,
		<code>game_day</code>, <code>repository</code>) is applied. Files whose frontmatter names a
		<code>repository</code> go there; everything else goes to the repository picked below.
	</p>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading…</p>
	{:else}
		<div class="controls">
			<label>
				Default repository
				<select bind:value={repositoryId}>
					{#each repositories as repo (repo.id)}
						<option value={repo.id}>{repositoryLabel(repo)}</option>
					{/each}
				</select>
			</label>

			<label>
				Vault folder
				<input type="file" webkitdirectory multiple onchange={onFilesPicked} />
			</label>

			<button
				type="button"
				onclick={runImport}
				disabled={importing || pending.length === 0 || !repositoryId}
			>
				{importing
					? 'Importing…'
					: `Import ${pending.length} file${pending.length === 1 ? '' : 's'}`}
			</button>
		</div>

		{#if pending.length > 0 && rows.length === 0}
			<ul class="file-list">
				{#each pending as file (file.path)}
					<li><code>{file.path}</code></li>
				{/each}
			</ul>
		{/if}

		{#if rows.length > 0}
			<p class="summary">
				{imported} imported{collisions > 0 ? `, ${collisions} collisions` : ''}{failed > 0
					? `, ${failed} failed`
					: ''}
			</p>

			<ul class="result-list">
				{#each rows as row (row.path)}
					<li class="result-row {row.status}">
						<code>{row.path}</code>
						{#if row.status === 'imported'}
							<span class="status">imported</span>
							{#if row.document_id}
								<a href={resolve('/documents/[id]', { id: row.document_id })}>open</a>
							{/if}
						{:else if row.status === 'collision'}
							<span class="status">already exists — rename or continue</span>
							<span class="collision-actions">
								<input type="text" bind:value={row.newPath} aria-label="New path for {row.path}" />
								<button type="button" disabled={row.busy} onclick={() => retryFile(row, false)}>
									Rename &amp; import
								</button>
								<button type="button" disabled={row.busy} onclick={() => retryFile(row, true)}>
									Import anyway
								</button>
							</span>
						{:else}
							<span class="status">{row.error ?? 'failed'}</span>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	{/if}
</main>

<style>
	.hint {
		max-width: 48rem;
		color: #555;
	}

	.controls {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-end;
		gap: 1rem;
		margin: 1rem 0;
	}

	.controls label {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		font-size: 0.9rem;
	}

	.controls select,
	.controls input {
		font: inherit;
	}

	.controls button {
		padding: 0.5rem 1rem;
		border: 1px solid #ccc;
		border-radius: 5px;
		background: none;
		font: inherit;
		cursor: pointer;
	}

	.controls button:disabled {
		opacity: 0.5;
		cursor: default;
	}

	.file-list {
		margin: 0;
		padding-left: 1.25rem;
		color: #555;
	}

	.summary {
		font-weight: 600;
	}

	.result-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}

	.result-row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.6rem;
		border: 1px solid #ccc;
		border-left-width: 4px;
		border-radius: 5px;
		padding: 0.5rem 0.75rem;
	}

	.result-row.imported {
		border-left-color: rgba(34, 197, 94, 0.8);
	}

	.result-row.collision {
		border-left-color: rgba(245, 158, 11, 0.8);
	}

	.result-row.error {
		border-left-color: rgba(239, 68, 68, 0.8);
	}

	.status {
		color: #555;
		font-size: 0.9rem;
	}

	.collision-actions {
		display: flex;
		align-items: center;
		gap: 0.4rem;
	}

	.collision-actions input {
		font: inherit;
		padding: 0.2rem 0.4rem;
	}

	.collision-actions button {
		border: 1px solid #ccc;
		border-radius: 5px;
		background: none;
		font: inherit;
		padding: 0.2rem 0.6rem;
		cursor: pointer;
	}
</style>
