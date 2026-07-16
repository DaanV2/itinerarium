<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getDocument, updateDocument, DocumentConflictError } from '$lib/api/documents';
	import { getAccessToken, isGM } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import ConcurrentEditDialog from '$lib/components/ConcurrentEditDialog.svelte';
	import type { Document, DocumentSection } from '$lib/types';

	const documentId = page.params.id ?? '';
	const gm = isGM();

	let doc = $state<Document | null>(null);
	let loading = $state(true);
	let error = $state('');
	let saving = $state(false);

	let editing = $state(false);
	let editTitle = $state('');
	let editPath = $state('');
	let editTags = $state('');
	let editSections = $state<DocumentSection[]>([]);
	let editVersion = $state(0);
	let showConflict = $state(false);

	async function loadDocument() {
		loading = true;
		try {
			doc = await getDocument(documentId, getAccessToken());
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load document.';
		} finally {
			loading = false;
		}
	}

	function startEditing() {
		if (!doc) return;
		editTitle = doc.title;
		editPath = doc.path;
		editTags = doc.tags.join(', ');
		editSections = doc.sections.map((s) => ({ ...s }));
		editVersion = doc.version;
		editing = true;
	}

	function cancelEditing() {
		editing = false;
	}

	async function save(force: boolean) {
		if (!doc) return;
		saving = true;
		try {
			doc = await updateDocument(
				documentId,
				{
					title: editTitle,
					path: editPath,
					tags: editTags
						.split(',')
						.map((t) => t.trim())
						.filter(Boolean),
					sharedOnGameDay: doc.shared_on_game_day,
					sections: editSections,
					expectedVersion: editVersion,
					force
				},
				getAccessToken()
			);
			editing = false;
			showConflict = false;
			error = '';
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
		await loadDocument();
		startEditing();
	}

	onMount(loadDocument);
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
			<input type="text" bind:value={editTitle} />
		</label>
		<label>
			Path
			<input type="text" bind:value={editPath} />
		</label>
		<label>
			Tags (comma-separated)
			<input type="text" bind:value={editTags} />
		</label>

		<div class="sections">
			{#each editSections as section, i (section.id)}
				<section class="doc-section" class:gm-only={section.gm_only}>
					<p class="section-banner">{section.gm_only ? 'GM only' : 'Visible to players'}</p>
					<textarea bind:value={editSections[i].content} rows="4"></textarea>
				</section>
			{/each}
		</div>

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
		<p class="meta">
			{doc.path} · shared on game day {doc.shared_on_game_day}
			{#if gm}
				· {doc.revealed ? 'revealed' : 'not yet revealed'}
			{/if}
		</p>
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
		padding: 0.75rem;
		white-space: pre-wrap;
	}

	label {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		margin-top: 0.75rem;
		font-size: 0.875rem;
	}

	textarea {
		width: 100%;
		border: none;
		font: inherit;
		padding: 0.75rem;
		resize: vertical;
	}

	.edit-actions {
		display: flex;
		gap: 0.5rem;
		margin-top: 1rem;
	}
</style>
