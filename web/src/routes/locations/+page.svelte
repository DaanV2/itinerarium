<script lang="ts">
	import { onMount } from 'svelte';
	import { createLocation, deleteLocation, listLocations } from '$lib/api/locations';
	import { getAccessToken } from '$lib/auth-token';
	import { buildLocationTree, type LocationNode } from '$lib/location-tree';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import FormField from '$lib/components/FormField.svelte';
	import SubmitButton from '$lib/components/SubmitButton.svelte';
	import type { Location } from '$lib/types';
	import LocationTree from './LocationTree.svelte';

	let locations = $state<Location[]>([]);
	let loading = $state(true);
	let error = $state('');

	let name = $state('');
	let description = $state('');
	let parentId = $state('');
	let submitting = $state(false);

	let tree = $derived(buildLocationTree(locations));

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

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		submitting = true;

		try {
			await createLocation(
				{ name, description: description || undefined, parentId: parentId || undefined },
				getAccessToken()
			);
			name = '';
			description = '';
			parentId = '';
			await loadLocations();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create location.';
		} finally {
			submitting = false;
		}
	}

	async function handleDelete(location: Location) {
		error = '';
		try {
			await deleteLocation(location.id, getAccessToken());
			await loadLocations();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete location.';
		}
	}

	// Flattened, indented options so a new location can be nested under any
	// existing one; blank keeps it a top-level plane.
	interface ParentOption {
		id: string;
		label: string;
	}
	function flattenOptions(nodes: LocationNode[], depth = 0): ParentOption[] {
		return nodes.flatMap((node) => [
			{ id: node.location.id, label: `${'  '.repeat(depth)}${node.location.name}` },
			...flattenOptions(node.children, depth + 1)
		]);
	}
	let parentOptions = $derived(flattenOptions(tree));
</script>

<main>
	<h1>Locations</h1>

	<ErrorAlert message={error} />

	<section>
		<h2>Create location</h2>
		<form onsubmit={handleCreate}>
			<FormField id="name" label="Name" type="text" required bind:value={name} />

			<label for="description">Description</label>
			<textarea id="description" bind:value={description} rows="3"></textarea>

			<label for="parent">Plane / parent location</label>
			<select id="parent" bind:value={parentId}>
				<option value="">— None (top-level plane) —</option>
				{#each parentOptions as option (option.id)}
					<option value={option.id}>{option.label}</option>
				{/each}
			</select>

			<SubmitButton pending={submitting} label="Create location" pendingLabel="Creating…" />
		</form>
	</section>

	<section>
		<h2>World map</h2>
		{#if loading}
			<p>Loading…</p>
		{:else if tree.length === 0}
			<p>No locations yet.</p>
		{:else}
			<LocationTree nodes={tree} onDelete={handleDelete} />
		{/if}
	</section>
</main>
