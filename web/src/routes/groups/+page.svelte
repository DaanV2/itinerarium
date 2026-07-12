<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { createGroup, listGroups } from '$lib/api/groups';
	import { getAccessToken, isGM } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import FormField from '$lib/components/FormField.svelte';
	import SubmitButton from '$lib/components/SubmitButton.svelte';
	import type { Group, GroupType } from '$lib/types';

	let groups = $state<Group[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Create form (GM only — the API rejects players with 403).
	let name = $state('');
	let type = $state<GroupType>('organization');
	let description = $state('');
	let submitting = $state(false);

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

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		submitting = true;
		try {
			await createGroup({ name, type, description: description || undefined }, getAccessToken());
			name = '';
			type = 'organization';
			description = '';
			await loadGroups();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create group.';
		} finally {
			submitting = false;
		}
	}
</script>

<main>
	<p><a href={resolve('/characters')}>← Characters</a></p>

	<h1>Groups</h1>

	<ErrorAlert message={error} />

	{#if isGM()}
		<section>
			<h2>Create group</h2>
			<form onsubmit={handleCreate}>
				<FormField id="group-name" label="Name" type="text" required bind:value={name} />

				<label for="group-type">Type</label>
				<select id="group-type" bind:value={type}>
					<option value="organization">Organization</option>
					<option value="family">Family</option>
					<option value="other">Other</option>
				</select>

				<FormField
					id="group-description"
					label="Description"
					type="text"
					bind:value={description}
				/>

				<SubmitButton pending={submitting} label="Create group" pendingLabel="Creating…" />
			</form>
		</section>
	{/if}

	<section>
		<h2>All groups</h2>
		{#if loading}
			<p>Loading…</p>
		{:else if groups.length === 0}
			<p>No groups yet.</p>
		{:else}
			<ul>
				{#each groups as group (group.id)}
					<li>
						<a href={resolve('/groups/[id]', { id: group.id })}>{group.name}</a>
						({group.type}) — {group.members.length}
						{group.members.length === 1 ? 'member' : 'members'}
					</li>
				{/each}
			</ul>
		{/if}
	</section>
</main>
