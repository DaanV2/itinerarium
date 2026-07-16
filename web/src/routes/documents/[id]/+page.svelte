<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getDocument } from '$lib/api/documents';
	import { getAccessToken, isGM } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import type { Document } from '$lib/types';

	const documentId = page.params.id ?? '';
	const gm = isGM();

	let doc = $state<Document | null>(null);
	let loading = $state(true);
	let error = $state('');

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

	onMount(loadDocument);
</script>

<main class="main-page">
	<p><a href={resolve('/repositories')}>← Repositories</a></p>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading…</p>
	{:else if !doc}
		<p>Document not found.</p>
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
</style>
