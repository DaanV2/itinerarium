<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import {
		getLocation,
		updateLocation,
		listLocationAccess,
		grantLocationAccess,
		revokeLocationAccess
	} from '$lib/api/locations';
	import { listCharacters } from '$lib/api/characters';
	import { listGroups } from '$lib/api/groups';
	import { getAccessToken, isGM } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import FormField from '$lib/components/FormField.svelte';
	import GmOnly from '$lib/components/GmOnly.svelte';
	import InventoryPanel from '$lib/components/InventoryPanel.svelte';
	import SubmitButton from '$lib/components/SubmitButton.svelte';
	import type { Character, Group, InventoryOwnerRef, Location, LocationAccess } from '$lib/types';

	const locationId = page.params.id ?? '';
	const owner: InventoryOwnerRef = { kind: 'location', id: locationId };
	const gm = isGM();

	let location = $state<Location | null>(null);
	let loading = $state(true);
	let error = $state('');

	// Edit form — anyone who can see the location can edit it.
	let editName = $state('');
	let editPlane = $state('');
	let editDescription = $state('');
	let saving = $state(false);

	// GM-only access management.
	let grants = $state<LocationAccess[]>([]);
	let allCharacters = $state<Character[]>([]);
	let allGroups = $state<Group[]>([]);
	let grantKind = $state<'character' | 'group'>('character');
	let grantTargetId = $state('');
	let granting = $state(false);

	function grantLabel(grant: LocationAccess): string {
		if (grant.character_id) {
			const character = allCharacters.find((c) => c.id === grant.character_id);
			return `Character: ${character?.name ?? grant.character_id}`;
		}
		const group = allGroups.find((g) => g.id === grant.group_id);
		return `Group: ${group?.name ?? grant.group_id}`;
	}

	async function loadAll() {
		loading = true;
		const token = getAccessToken();
		try {
			location = await getLocation(locationId, token);
			editName = location.name;
			editPlane = location.plane ?? '';
			editDescription = location.description ?? '';

			if (gm) {
				[grants, allCharacters, allGroups] = await Promise.all([
					listLocationAccess(locationId, token),
					listCharacters(token),
					listGroups(token)
				]);
			}
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load location.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAll);

	async function handleSave(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		saving = true;
		try {
			location = await updateLocation(
				locationId,
				{ name: editName, plane: editPlane, description: editDescription },
				getAccessToken()
			);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save location.';
		} finally {
			saving = false;
		}
	}

	async function handleGrant(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		granting = true;
		try {
			await grantLocationAccess(
				locationId,
				grantKind === 'character' ? { character_id: grantTargetId } : { group_id: grantTargetId },
				getAccessToken()
			);
			grantTargetId = '';
			grants = await listLocationAccess(locationId, getAccessToken());
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to grant access.';
		} finally {
			granting = false;
		}
	}

	async function handleRevoke(accessId: string) {
		error = '';
		try {
			await revokeLocationAccess(locationId, accessId, getAccessToken());
			grants = await listLocationAccess(locationId, getAccessToken());
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to revoke access.';
		}
	}
</script>

<main class="main-page">
	<p><a href={resolve('/locations')}>← Locations</a></p>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading…</p>
	{:else if !location}
		<p>Location not found.</p>
	{:else}
		<h1>{location.name}</h1>
		{#if location.plane}<p>Plane: {location.plane}</p>{/if}
		{#if location.description}<p>{location.description}</p>{/if}

		<section>
			<h2>Edit</h2>
			<form onsubmit={handleSave}>
				<FormField id="edit-name" label="Name" type="text" required bind:value={editName} />
				<FormField id="edit-plane" label="Plane" type="text" bind:value={editPlane} />
				<FormField
					id="edit-description"
					label="Description"
					type="text"
					bind:value={editDescription}
				/>

				<SubmitButton pending={saving} label="Save" pendingLabel="Saving…" />
			</form>
		</section>

		<GmOnly>
			<section>
				<h2>Access</h2>
				{#if grants.length === 0}
					<p>No grants yet — only GMs can see this location.</p>
				{:else}
					<ul>
						{#each grants as grant (grant.id)}
							<li>
								{grantLabel(grant)}
								<button type="button" onclick={() => handleRevoke(grant.id)}>Revoke</button>
							</li>
						{/each}
					</ul>
				{/if}

				<form onsubmit={handleGrant}>
					<h3>Grant access</h3>
					<label for="grant-kind">To a</label>
					<select id="grant-kind" bind:value={grantKind} onchange={() => (grantTargetId = '')}>
						<option value="character">Character</option>
						<option value="group">Group</option>
					</select>

					<label for="grant-target">Target</label>
					<select id="grant-target" bind:value={grantTargetId} required>
						<option value="" disabled>Pick…</option>
						{#if grantKind === 'character'}
							{#each allCharacters as character (character.id)}
								<option value={character.id}>{character.name}</option>
							{/each}
						{:else}
							{#each allGroups as group (group.id)}
								<option value={group.id}>{group.name}</option>
							{/each}
						{/if}
					</select>

					<SubmitButton pending={granting} label="Grant" pendingLabel="Granting…" />
				</form>
			</section>
		</GmOnly>

		<InventoryPanel {owner} />
	{/if}
</main>
