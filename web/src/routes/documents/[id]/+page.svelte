<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import {
		getDocument,
		updateDocument,
		listDocumentShares,
		DocumentConflictError
	} from '$lib/api/documents';
	import { getRepository } from '$lib/api/repositories';
	import { listCharacters } from '$lib/api/characters';
	import { listGroups } from '$lib/api/groups';
	import { getAccessToken, isGM } from '$lib/auth-token';
	import { describeAudience } from '$lib/document-reveal';
	import {
		buildDocumentUpdate,
		editFieldsFromDocument,
		emptyDocumentEditFields,
		newDocumentSection,
		sharedCharacterNames as resolveSharedNames
	} from '$lib/document-editor';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import ConcurrentEditDialog from '$lib/components/ConcurrentEditDialog.svelte';
	import type { Character, Document, DocumentShare, Group, Repository } from '$lib/types';

	const documentId = page.params.id ?? '';
	const gm = isGM();

	let doc = $state<Document | null>(null);
	let repository = $state<Repository | null>(null);
	let characters = $state<Character[]>([]);
	let groups = $state<Group[]>([]);
	let shares = $state<DocumentShare[]>([]);
	let loading = $state(true);
	let error = $state('');
	let saving = $state(false);

	let editing = $state(false);
	let form = $state(emptyDocumentEditFields());
	let showConflict = $state(false);

	let audience = $derived(repository ? describeAudience(repository, characters, groups) : '');
	let sharedCharacterNames = $derived(resolveSharedNames(shares, characters));

	async function loadAll() {
		loading = true;
		const token = getAccessToken();
		try {
			doc = await getDocument(documentId, token);
			[repository, characters, groups, shares] = await Promise.all([
				getRepository(doc.repository_id, token),
				listCharacters(token),
				listGroups(token),
				gm ? listDocumentShares(documentId, token) : Promise.resolve([])
			]);
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load document.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAll);

	function startEditing() {
		if (!doc) return;
		form = editFieldsFromDocument(doc);
		editing = true;
	}

	function cancelEditing() {
		editing = false;
	}

	function addSection() {
		form.sections = [...form.sections, newDocumentSection()];
	}

	function removeSection(index: number) {
		form.sections = form.sections.filter((_, i) => i !== index);
	}

	async function save(force: boolean) {
		if (!doc) return;
		saving = true;
		try {
			const token = getAccessToken();
			doc = await updateDocument(documentId, buildDocumentUpdate(form, force), token);
			editing = false;
			showConflict = false;
			error = '';
			if (gm) shares = await listDocumentShares(documentId, token);
		} catch (err) {
			if (err instanceof DocumentConflictError && err.code === 'concurrent_edit') {
				showConflict = true;
			} else {
				error = err instanceof Error ? err.message : 'Failed to save document.';
			}
		} finally {
			saving = false;
		}
	}

	async function reloadAfterConflict() {
		showConflict = false;
		await loadAll();
		startEditing();
	}
</script>

<main class="main-page">
	<p><a href={resolve('/repositories')}>← Repositories</a></p>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading…</p>
	{:else if !doc}
		<p>Document not found.</p>
	{:else if editing}
		<label>
			Title
			<input type="text" bind:value={form.title} />
		</label>
		<label>
			Path
			<input type="text" bind:value={form.path} />
		</label>
		<label>
			Tags (comma-separated)
			<input type="text" bind:value={form.tags} />
		</label>
		{#if gm}
			<label>
				Revealed at game day
				<input type="number" min="0" bind:value={form.sharedOnGameDay} />
			</label>
		{/if}

		<div class="sections">
			{#each form.sections as section, i (section.id || `new-${i}`)}
				<section class="doc-section" class:gm-only={section.gm_only}>
					<p class="section-banner">{section.gm_only ? 'GM only' : 'Visible to players'}</p>
					<textarea class="section-content" bind:value={form.sections[i].content} rows="4"
					></textarea>
					<div class="section-actions">
						{#if gm}
							<label class="gm-only-toggle">
								<input type="checkbox" bind:checked={form.sections[i].gm_only} />
								GM only
							</label>
						{/if}
						<button type="button" onclick={() => removeSection(i)}>Remove section</button>
					</div>
				</section>
			{/each}
		</div>

		<button type="button" onclick={addSection}>Add section</button>

		<div class="edit-actions">
			<button type="button" onclick={cancelEditing} disabled={saving}>Cancel</button>
			<button type="button" onclick={() => save(false)} disabled={saving}>
				{saving ? 'Saving…' : 'Save'}
			</button>
		</div>

		<ConcurrentEditDialog
			open={showConflict}
			onCancel={() => (showConflict = false)}
			onReload={reloadAfterConflict}
			onOverwrite={() => save(true)}
		/>
	{:else}
		<h1>{doc.title}</h1>
		<p class="meta">{doc.path}</p>

		<p class="reveal-banner">
			Revealed at game day {doc.shared_on_game_day} to {audience}.
			{#if gm && sharedCharacterNames.length > 0}
				Also directly shared with {sharedCharacterNames.join(', ')}.
			{/if}
		</p>

		{#if doc.revealed}
			<p class="revealed-warning" role="alert">
				This document is already revealed — any changes you save are visible immediately to everyone
				who can currently see it. There is no versioning; consider a new document or a GM-only
				section for future reveals instead.
			</p>
		{/if}

		{#if doc.tags.length > 0}
			<p class="tags">
				{#each doc.tags as tag (tag)}
					<span class="tag">{tag}</span>
				{/each}
			</p>
		{/if}

		<div class="sections">
			{#each doc.sections as section (section.id)}
				<section class="doc-section" class:gm-only={section.gm_only}>
					<p class="section-banner">{section.gm_only ? 'GM only' : 'Visible to players'}</p>
					<p class="section-content">{section.content}</p>
				</section>
			{/each}
		</div>

		<button type="button" onclick={startEditing}>Edit</button>
	{/if}
</main>

<style>
	.meta {
		color: rgba(128, 128, 128, 0.9);
		font-size: 0.875rem;
	}

	.tags {
		display: flex;
		gap: 0.4rem;
		flex-wrap: wrap;
	}

	.tag {
		background-color: rgba(128, 128, 128, 0.15);
		border-radius: 999px;
		padding: 0.1rem 0.6rem;
		font-size: 0.8rem;
	}

	.reveal-banner {
		background-color: rgba(59, 130, 246, 0.1);
		border: 1px solid rgba(59, 130, 246, 0.4);
		border-radius: 5px;
		padding: 0.6rem 0.9rem;
		font-size: 0.875rem;
		margin-top: 0.5rem;
	}

	.revealed-warning {
		background-color: rgba(234, 179, 8, 0.12);
		border: 1px solid rgba(234, 179, 8, 0.5);
		border-radius: 5px;
		padding: 0.6rem 0.9rem;
		font-size: 0.875rem;
		margin-top: 0.5rem;
	}

	.sections {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		margin-top: 1rem;
	}

	.doc-section {
		border: 1px solid #ccc;
		border-radius: 5px;
		overflow: hidden;
	}

	.doc-section.gm-only {
		border: 1px dashed rgba(34, 197, 94, 0.6);
	}

	.section-banner {
		margin: 0;
		padding: 0.3rem 0.75rem;
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.03em;
		background-color: rgba(128, 128, 128, 0.12);
		color: rgba(128, 128, 128, 1);
	}

	.gm-only .section-banner {
		background-color: rgba(34, 197, 94, 0.15);
		color: rgba(21, 128, 61, 1);
	}

	.section-content {
		margin: 0;
		width: 100%;
		box-sizing: border-box;
		border: none;
		padding: 0.75rem;
		font: inherit;
		white-space: pre-wrap;
		resize: vertical;
	}

	.section-actions {
		display: flex;
		align-items: center;
		gap: 1rem;
		padding: 0.5rem 0.75rem;
		border-top: 1px solid rgba(128, 128, 128, 0.2);
	}

	.gm-only-toggle {
		display: flex;
		align-items: center;
		gap: 0.3rem;
	}

	label {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		margin-top: 0.75rem;
		font-size: 0.875rem;
	}

	.edit-actions {
		display: flex;
		gap: 0.5rem;
		margin-top: 1rem;
	}
</style>
