<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getCharacter } from '$lib/api/characters';
	import { listLocations, setCharacterLocation, clearCharacterLocation } from '$lib/api/locations';
	import { getAccessToken } from '$lib/auth-token';
	import ActivityPanel from '$lib/components/ActivityPanel.svelte';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import InventoryPanel from '$lib/components/InventoryPanel.svelte';
	import JournalPanel from '$lib/components/JournalPanel.svelte';
	import MoneyPanel from '$lib/components/MoneyPanel.svelte';
	import Tabs from '$lib/components/Tabs.svelte';
	import type { Character, InventoryOwnerRef, LocationSummary } from '$lib/types';

	// Always present for this route; `?? ''` keeps the type a plain string.
	const characterId = page.params.id ?? '';
	const owner: InventoryOwnerRef = { kind: 'character', id: characterId };

	let character = $state<Character | null>(null);
	let locations = $state<LocationSummary[]>([]);
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
		<p class="game-day">Game day {character.current_game_day}</p>

		<section class="location-section">
			<h2>Location</h2>
			{#if currentLocation}
				<p>
					Currently at
					<a href={resolve('/locations/[id]', { id: currentLocation.id })}>
						{currentLocation.name}
					</a>
				</p>
			{:else}
				<p class="empty-state">No location set.</p>
			{/if}

			<div class="location-move">
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
			</div>
		</section>

		{#snippet inventoryPanel()}
			<InventoryPanel {owner} />
		{/snippet}

		{#snippet moneyPanel()}
			<MoneyPanel {owner} />
		{/snippet}

		{#snippet journalPanel()}
			<JournalPanel {characterId} />
		{/snippet}

		{#snippet activityPanel()}
			<ActivityPanel {characterId} />
		{/snippet}

		<Tabs
			tabs={[
				{ id: 'inventory', label: 'Inventory', panel: inventoryPanel },
				{ id: 'money', label: 'Money', panel: moneyPanel },
				{ id: 'journal', label: 'Journal', panel: journalPanel },
				{ id: 'activity', label: 'Activity', panel: activityPanel }
			]}
		/>
	{/if}
</main>

<style>
	.game-day {
		margin: 0.25rem 0 0;
		color: var(--color-muted);
		font-size: 0.9rem;
	}

	.location-section {
		margin-top: 1.5rem;
	}

	.location-move {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		margin-top: 0.5rem;
		flex-wrap: wrap;
	}

	.location-move label {
		font-size: 0.875rem;
	}

	.location-move select {
		flex: 1;
		min-width: 180px;
	}
</style>
