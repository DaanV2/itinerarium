<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { createGroup, listGroups } from '$lib/api/groups';
	import { getAccessToken } from '$lib/auth-token';
	import CreateModal from '$lib/components/CreateModal.svelte';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import FormField from '$lib/components/FormField.svelte';
	import GmOnly from '$lib/components/GmOnly.svelte';
	import type { Group, GroupType } from '$lib/types';

	let groups = $state<Group[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Create form (GM only — the API rejects players with 403).
	let name = $state('');
	let type = $state<GroupType>('organization');
	let description = $state('');

	async function loadGroups() {
		loading = true;
		try {
			groups = await listGroups(getAccessToken());
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load groups.';
		} finally {
			loading = false;
		}
	}

	onMount(loadGroups);

	async function handleCreate() {
		await createGroup({ name, type, description: description || undefined }, getAccessToken());
		name = '';
		type = 'organization';
		description = '';
		await loadGroups();
	}
</script>

<main class="main-page">
	<p><a href={resolve('/characters')}>← Characters</a></p>

	<h1>Groups</h1>

	<ErrorAlert message={error} />

	<GmOnly>
		<section>
			<CreateModal triggerLabel="Create group" pendingLabel="Creating..." onSubmit={handleCreate}>
				<FormField id="group-name" label="Name" type="text" required bind:value={name} />

				<div class="field">
					<label for="group-type">Type</label>
					<select id="group-type" bind:value={type}>
						<option value="organization">Organization</option>
						<option value="family">Family</option>
						<option value="other">Other</option>
					</select>
				</div>

				<FormField
					id="group-description"
					label="Description"
					type="text"
					bind:value={description}
				/>
			</CreateModal>
		</section>
	</GmOnly>

	<section>
		<h2>All groups</h2>
		{#if loading}
			<p>Loading…</p>
		{:else if groups.length === 0}
			<p class="empty-state">No groups yet.</p>
		{:else}
			<ul class="entity-list">
				{#each groups as group (group.id)}
					<li>
						<a class="entity-row" href={resolve('/groups/[id]', { id: group.id })}>
							<span class="entity-name">{group.name}</span>
							<span class="entity-meta"
								>{group.type} · {group.members.length}
								{group.members.length === 1 ? 'member' : 'members'}</span
							>
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
</main>

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		margin-top: 0.75rem;
	}

	label {
		font-size: 0.875rem;
		font-weight: 500;
	}

	select {
		width: 100%;
	}
</style>
