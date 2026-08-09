<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { createLocation, listLocations } from '$lib/api/locations';
	import { getAccessToken } from '$lib/auth-token';
	import CreateModal from '$lib/components/CreateModal.svelte';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import FormField from '$lib/components/FormField.svelte';
	import GmOnly from '$lib/components/GmOnly.svelte';
	import type { LocationSummary } from '$lib/types';

	let locations = $state<LocationSummary[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Create form (GM only — the API rejects players with 403). The
	// description is added afterward from the location's own page.
	let name = $state('');
	let plane = $state('');

	async function loadLocations() {
		loading = true;
		try {
			locations = await listLocations(getAccessToken());
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load locations.';
		} finally {
			loading = false;
		}
	}

	onMount(loadLocations);

	async function handleCreate() {
		await createLocation({ name, plane: plane || undefined }, getAccessToken());
		name = '';
		plane = '';
		await loadLocations();
	}
</script>

<main class="main-page">
	<p><a href={resolve('/characters')}>← Characters</a></p>

	<h1>Locations</h1>

	<ErrorAlert message={error} />

	<GmOnly>
		<section>
			<CreateModal
				triggerLabel="Create location"
				pendingLabel="Creating..."
				onSubmit={handleCreate}
			>
				<FormField id="location-name" label="Name" type="text" required bind:value={name} />
				<FormField id="location-plane" label="Plane" type="text" bind:value={plane} />
			</CreateModal>
		</section>
	</GmOnly>

	<section>
		<h2>Known locations</h2>
		{#if loading}
			<p>Loading…</p>
		{:else if locations.length === 0}
			<p class="empty-state">No locations you can see yet.</p>
		{:else}
			<ul class="entity-list">
				{#each locations as location (location.id)}
					<li>
						<a class="entity-row" href={resolve('/locations/[id]', { id: location.id })}>
							<span class="entity-name">{location.name}</span>
							{#if location.plane}
								<span class="entity-meta">{location.plane}</span>
							{/if}
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
</main>
