<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { createCharacter, listCharacters } from '$lib/api/characters';
	import { getAccessToken } from '$lib/auth-token';
	import CreateModal from '$lib/components/CreateModal.svelte';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import FormField from '$lib/components/FormField.svelte';
	import type { Character } from '$lib/types';

	let characters = $state<Character[]>([]);
	let loading = $state(true);
	let name = $state('');
	let error = $state('');

	async function loadCharacters() {
		loading = true;
		try {
			characters = await listCharacters(getAccessToken());
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load characters.';
		} finally {
			loading = false;
		}
	}

	onMount(loadCharacters);

	async function handleCreate() {
		await createCharacter(name, getAccessToken());
		name = '';
		await loadCharacters();
	}
</script>

<main class="main-page">
	<h1>Characters</h1>

	<p class="page-links">
		<a href={resolve('/groups')}>Groups</a> ·
		<a href={resolve('/locations')}>Locations</a>
	</p>

	<ErrorAlert message={error} />

	<section>
		<CreateModal triggerLabel="Create character" pendingLabel="Creating..." onSubmit={handleCreate}>
			<FormField id="name" label="Name" type="text" required bind:value={name} />
		</CreateModal>
	</section>

	<section>
		<h2>Your characters</h2>
		{#if loading}
			<p>Loading…</p>
		{:else if characters.length === 0}
			<p class="empty-state">No characters yet. Create one above to get started.</p>
		{:else}
			<ul class="entity-list">
				{#each characters as character (character.id)}
					<li>
						<a class="entity-row" href={resolve('/characters/[id]', { id: character.id })}>
							<span class="entity-name">{character.name}</span>
							<span class="entity-meta">Game day {character.current_game_day}</span>
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
</main>
