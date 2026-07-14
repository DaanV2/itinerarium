<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getCharacter } from '$lib/api/characters';
	import { listLocations, setCharacterLocation, clearCharacterLocation } from '$lib/api/locations';
	import { getAccessToken } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import InventoryPanel from '$lib/components/InventoryPanel.svelte';
	import JournalPanel from '$lib/components/JournalPanel.svelte';
	import MoneyPanel from '$lib/components/MoneyPanel.svelte';
	import type { Character, InventoryOwnerRef, Location } from '$lib/types';

	// Always present for this route; `?? ''` keeps the type a plain string.
	const characterId = page.params.id ?? '';
	const owner: InventoryOwnerRef = { kind: 'character', id: characterId };

	let character = $state<Character | null>(null);
	let locations = $state<Location[]>([]);
	let loading = $state(true);
	let error = $state('');

	let currentLocation = $derived(locations.find((l) => l.id === character?.location_id) ?? null);

	async function loadAll() {
		loading = true;
		const token = getAccessToken();
		try {
			[character, locations] = await Promise.all([
				getCharacter(characterId, token),
				listLocations(token)
			]);
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load character.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAll);

	async function handleLocationChange(locationId: string) {
		error = '';
		try {
			character = locationId
				? await setCharacterLocation(characterId, locationId, getAccessToken())
				: await clearCharacterLocation(characterId, getAccessToken());
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to update location.';
		}
	}
</script>

<main class="main-page">
	<p><a href={resolve('/characters')}>← Characters</a></p>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading…</p>
	{:else if !character}
		<p>Character not found.</p>
	{:else}
		<h1>{character.name}</h1>
		<p>Game day {character.current_game_day}</p>

		<section>
			<h2>Location</h2>
			{#if currentLocation}
				<p>
					Currently at
					<a href={resolve('/locations/[id]', { id: currentLocation.id })}>
						{currentLocation.name}
					</a>
				</p>
			{:else}
				<p>No location set.</p>
			{/if}

			<label for="character-location">Move to</label>
			<select
				id="character-location"
				value={character.location_id ?? ''}
				onchange={(e) => handleLocationChange((e.target as HTMLSelectElement).value)}
			>
				<option value="">— no location —</option>
				{#each locations as location (location.id)}
					<option value={location.id}>
						{location.name}{location.plane ? ` (${location.plane})` : ''}
					</option>
				{/each}
			</select>
		</section>

		<InventoryPanel {owner} />
		<MoneyPanel {owner} />
		<JournalPanel {characterId} />
	{/if}
</main>
