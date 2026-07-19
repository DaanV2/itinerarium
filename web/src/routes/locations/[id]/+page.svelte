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
	import type {
		Character,
		Group,
		InventoryOwnerRef,
		Location,
		LocationAccess,
		LocationSection
	} from '$lib/types';

	const locationId = page.params.id ?? '';
	const owner: InventoryOwnerRef = { kind: 'location', id: locationId };
	const gm = isGM();

	let location = $state<Location | null>(null);
	let loading = $state(true);
	let error = $state('');

	// Edit form — anyone who can see the location can edit it. The
	// description follows the same game-day/GM-only rules as a document.
	let editing = $state(false);
	let editName = $state('');
	let editPlane = $state('');
	let editSharedOnGameDay = $state(0);
	let editSections = $state<LocationSection[]>([]);
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

	function startEditing() {
		if (!location) return;
		editName = location.name;
		editPlane = location.plane ?? '';
		editSharedOnGameDay = location.shared_on_game_day;
		editSections = location.sections.map((s) => ({ ...s }));
		editing = true;
	}

	function cancelEditing() {
		editing = false;
	}

	function addSection() {
		editSections = [...editSections, { id: '', content: '', gm_only: false }];
	}

	function removeSection(index: number) {
		editSections = editSections.filter((_, i) => i !== index);
	}

	async function handleSave(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		saving = true;
		try {
			location = await updateLocation(
				locationId,
				{
					name: editName,
					plane: editPlane,
					shared_on_game_day: editSharedOnGameDay,
					sections: editSections
				},
				getAccessToken()
			);
			editing = false;
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
	{:else if editing}
		<h1>{location.name}</h1>

		<form onsubmit={handleSave}>
			<FormField id="edit-name" label="Name" type="text" required bind:value={editName} />
			<FormField id="edit-plane" label="Plane" type="text" bind:value={editPlane} />
			{#if gm}
				<label>
					Description revealed at game day
					<input type="number" min="0" bind:value={editSharedOnGameDay} />
				</label>
			{/if}

			<div class="sections">
				{#each editSections as section, i (section.id || `new-${i}`)}
					<section class="loc-section" class:gm-only={section.gm_only}>
						<p class="section-banner">{section.gm_only ? 'GM only' : 'Visible to players'}</p>
						<textarea class="section-content" bind:value={editSections[i].content} rows="4"
						></textarea>
						<div class="section-actions">
							{#if gm}
								<label class="gm-only-toggle">
									<input type="checkbox" bind:checked={editSections[i].gm_only} />
									GM only
								</label>
							{/if}
							<button type="button" onclick={() => removeSection(i)}>Remove section</button>
						</div>
					</section>
				{/each}
			</div>

			<button type="button" onclick={addSection}>Add description section</button>

			<div class="edit-actions">
				<button type="button" onclick={cancelEditing} disabled={saving}>Cancel</button>
				<SubmitButton pending={saving} label="Save" pendingLabel="Saving…" />
			</div>
		</form>
	{:else}
		<h1>{location.name}</h1>
		{#if location.plane}<p>Plane: {location.plane}</p>{/if}

		{#if gm}
			<p class="reveal-banner">
				Description revealed at game day {location.shared_on_game_day}.
			</p>
		{/if}

		<div class="sections">
			{#each location.sections as section (section.id)}
				<section class="loc-section" class:gm-only={section.gm_only}>
					<p class="section-banner">{section.gm_only ? 'GM only' : 'Visible to players'}</p>
					<p class="section-content">{section.content}</p>
				</section>
			{/each}
		</div>

		<button type="button" onclick={startEditing}>Edit</button>

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

<style>
	.reveal-banner {
		background-color: rgba(59, 130, 246, 0.1);
		border: 1px solid rgba(59, 130, 246, 0.4);
		border-radius: 5px;
		padding: 0.6rem 0.9rem;
		font-size: 0.875rem;
		margin-top: 0.5rem;
	}

	.sections {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		margin-top: 1rem;
	}

	.loc-section {
		border: 1px solid #ccc;
		border-radius: 5px;
		overflow: hidden;
	}

	.loc-section.gm-only {
		border: 1px dashed rgba(34, 197, 94, 0.6);
	}

	.section-banner {
		margin: 0;
		padding: 0.3rem 0.75rem;
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.03em;
		background-color: rgba(128, 128, 128, 0.12);
		color: rgba(128, 128, 128, 1);
	}

	.gm-only .section-banner {
		background-color: rgba(34, 197, 94, 0.15);
		color: rgba(21, 128, 61, 1);
	}

	.section-content {
		margin: 0;
		width: 100%;
		box-sizing: border-box;
		border: none;
		padding: 0.75rem;
		font: inherit;
		white-space: pre-wrap;
		resize: vertical;
	}

	.section-actions {
		display: flex;
		align-items: center;
		gap: 1rem;
		padding: 0.5rem 0.75rem;
		border-top: 1px solid rgba(128, 128, 128, 0.2);
	}

	.gm-only-toggle {
		display: flex;
		align-items: center;
		gap: 0.3rem;
	}

	label {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		margin-top: 0.75rem;
		font-size: 0.875rem;
	}

	.edit-actions {
		display: flex;
		gap: 0.5rem;
		margin-top: 1rem;
	}
</style>
