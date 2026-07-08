<script lang="ts">
	import { onMount } from 'svelte';
	import { createCharacter, listCharacters } from '$lib/api/characters';
	import type { Character } from '$lib/types';

	let characters = $state<Character[]>([]);
	let loading = $state(true);
	let name = $state('');
	let submitting = $state(false);
	let error = $state('');

	function accessToken(): string {
		return localStorage.getItem('itinerarium_access_token') ?? '';
	}

	async function loadCharacters() {
		loading = true;
		try {
			characters = await listCharacters(accessToken());
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load characters.';
		} finally {
			loading = false;
		}
	}

	onMount(loadCharacters);

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		submitting = true;

		try {
			await createCharacter(name, accessToken());
			name = '';
			await loadCharacters();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create character.';
		} finally {
			submitting = false;
		}
	}
</script>

<main>
	<h1>Characters</h1>

	{#if error}
		<p role="alert">{error}</p>
	{/if}

	<section>
		<h2>Create character</h2>
		<form onsubmit={handleCreate}>
			<label for="name">Name</label>
			<input id="name" type="text" required bind:value={name} />

			<button type="submit" disabled={submitting}>
				{submitting ? 'Creating…' : 'Create character'}
			</button>
		</form>
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
					<li>{character.name} — game day {character.current_game_day}</li>
				{/each}
			</ul>
		{/if}
	</section>
</main>
