<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getCharacter } from '$lib/api/characters';
	import { getAccessToken } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import JournalPanel from '$lib/components/JournalPanel.svelte';
	import type { Character } from '$lib/types';

	// Always present for this route; `?? ''` keeps the type a plain string.
	const characterId = page.params.id ?? '';

	let character = $state<Character | null>(null);
	let loading = $state(true);
	let error = $state('');

	async function loadCharacter() {
		loading = true;
		try {
			character = await getCharacter(characterId, getAccessToken());
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load character.';
		} finally {
			loading = false;
		}
	}

	onMount(loadCharacter);
</script>

<main class="main-page">
	<p>
		<a href={resolve('/characters/[id]', { id: characterId })}>← Back to character</a>
	</p>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading...</p>
	{:else if !character}
		<p>Character not found.</p>
	{:else}
		<h1>{character.name}'s journal</h1>
		<p>Game day {character.current_game_day}</p>

		<JournalPanel {characterId} />
	{/if}
</main>
