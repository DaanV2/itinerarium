<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import {
		convertJournalEntry,
		createJournalEntry,
		listJournalEntries,
		updateJournalEntry
	} from '$lib/api/journal';
	import { getAccessToken } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import SubmitButton from '$lib/components/SubmitButton.svelte';
	import type { Document, JournalEntry } from '$lib/types';

	let { characterId }: { characterId: string } = $props();

	let entries = $state<JournalEntry[]>([]);
	let loading = $state(true);
	let error = $state('');

	let newContent = $state('');
	let addingEntry = $state(false);

	let editingEntryId = $state('');
	let editContent = $state('');
	let editPending = $state(false);

	let convertingEntryId = $state('');
	let convertedDocuments = $state<Record<string, Document>>({});

	async function loadAll() {
		loading = true;
		try {
			entries = await listJournalEntries(characterId, getAccessToken());
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load journal.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAll);

	async function handleAddEntry(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		addingEntry = true;
		try {
			await createJournalEntry(characterId, newContent, getAccessToken());
			newContent = '';
			await loadAll();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to add journal entry.';
		} finally {
			addingEntry = false;
		}
	}

	function startEdit(entry: JournalEntry) {
		editingEntryId = entry.id;
		editContent = entry.content;
	}

	function cancelEdit() {
		editingEntryId = '';
	}

	async function handleEdit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		editPending = true;
		try {
			await updateJournalEntry(characterId, editingEntryId, editContent, getAccessToken());
			editingEntryId = '';
			await loadAll();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to update journal entry.';
		} finally {
			editPending = false;
		}
	}

	async function handleConvert(entry: JournalEntry) {
		error = '';
		convertingEntryId = entry.id;
		try {
			const doc = await convertJournalEntry(characterId, entry.id, getAccessToken());
			convertedDocuments = { ...convertedDocuments, [entry.id]: doc };
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to convert journal entry to a document.';
		} finally {
			convertingEntryId = '';
		}
	}
</script>

<section>
	<h2>Journal</h2>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading…</p>
	{:else}
		{#if entries.length === 0}
			<p>No journal entries yet.</p>
		{:else}
			<ul>
				{#each entries as entry (entry.id)}
					<li>
						<p>Game day {entry.game_day}</p>
						{#if editingEntryId === entry.id}
							<form onsubmit={handleEdit}>
								<textarea bind:value={editContent} required></textarea>
								<SubmitButton pending={editPending} label="Save" pendingLabel="Saving…" />
								<button type="button" onclick={cancelEdit}>Cancel</button>
							</form>
						{:else}
							<p>{entry.content}</p>
							<button type="button" onclick={() => startEdit(entry)}>Edit</button>
							{#if convertedDocuments[entry.id]}
								<a href={resolve('/documents/[id]', { id: convertedDocuments[entry.id].id })}>
									View document
								</a>
							{:else}
								<button
									type="button"
									onclick={() => handleConvert(entry)}
									disabled={convertingEntryId === entry.id}
								>
									{convertingEntryId === entry.id ? 'Converting…' : 'Convert to document'}
								</button>
							{/if}
						{/if}
					</li>
				{/each}
			</ul>
		{/if}

		<form onsubmit={handleAddEntry}>
			<h3>New entry</h3>
			<textarea bind:value={newContent} required placeholder="Dear diary…"></textarea>
			<SubmitButton pending={addingEntry} label="Add entry" pendingLabel="Adding…" />
		</form>
	{/if}
</section>
