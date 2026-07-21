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

	<nav>
		<a href={resolve('/groups')}>Groups</a> ·
		<a href={resolve('/locations')}>Locations</a>
	</nav>

	<ErrorAlert message={error} />

	<section>
		<CreateModal triggerLabel="Create character" pendingLabel="Creating…" onSubmit={handleCreate}>
			<FormField id="name" label="Name" type="text" required bind:value={name} />
		</CreateModal>
	</section>

	<section>
		<h2>Your characters</h2>
		{#if loading}
			<p>Loading…</p>
		{:else if characters.length === 0}
			<p>No characters yet.</p>
		{:else}
			<ul>
				{#each characters as character (character.id)}
					<li>
						<a href={resolve('/characters/[id]', { id: character.id })}>{character.name}</a> — game
						day
						{character.current_game_day}
					</li>
				{/each}
			</ul>
		{/if}
	</section>
</main>
