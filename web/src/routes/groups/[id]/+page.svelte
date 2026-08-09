<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getGroup, joinGroup, leaveGroup } from '$lib/api/groups';
	import { listCharacters } from '$lib/api/characters';
	import { getAccessToken, isGM } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import InventoryPanel from '$lib/components/InventoryPanel.svelte';
	import MoneyPanel from '$lib/components/MoneyPanel.svelte';
	import SubmitButton from '$lib/components/SubmitButton.svelte';
	import Tabs from '$lib/components/Tabs.svelte';
	import type { Character, Group, InventoryOwnerRef } from '$lib/types';

	const groupId = page.params.id ?? '';
	const owner: InventoryOwnerRef = { kind: 'group', id: groupId };

	let group = $state<Group | null>(null);
	let myCharacters = $state<Character[]>([]);
	let loading = $state(true);
	let error = $state('');

	let joinCharacterId = $state('');
	let joining = $state(false);

	// Characters of mine that are current members / could still join.
	let memberIds = $derived(new Set((group?.members ?? []).map((m) => m.id)));
	let joinable = $derived(myCharacters.filter((c) => !memberIds.has(c.id)));
	// GMs always have access server-side; for players the shared content is
	// member-only. Client checks are UX sugar — the API enforces the rule.
	let canSeeContent = $derived(isGM() || myCharacters.some((c) => memberIds.has(c.id)));

	async function loadAll() {
		loading = true;
		const token = getAccessToken();
		try {
			[group, myCharacters] = await Promise.all([getGroup(groupId, token), listCharacters(token)]);
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load group.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAll);

	async function handleJoin(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		joining = true;
		try {
			await joinGroup(groupId, joinCharacterId, getAccessToken());
			joinCharacterId = '';
			group = await getGroup(groupId, getAccessToken());
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to join group.';
		} finally {
			joining = false;
		}
	}

	async function handleLeave(characterId: string) {
		error = '';
		try {
			await leaveGroup(groupId, characterId, getAccessToken());
			group = await getGroup(groupId, getAccessToken());
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to leave group.';
		}
	}
</script>

<main class="main-page">
	<p><a href={resolve('/groups')}>← Groups</a></p>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading…</p>
	{:else if !group}
		<p>Group not found.</p>
	{:else}
		<h1>{group.name}</h1>
		<p class="group-meta">{group.type}{group.description ? ` — ${group.description}` : ''}</p>

		{#snippet membersPanel()}
			<section>
				{#if group && group.members.length === 0}
					<p class="empty-state">No members yet.</p>
				{:else if group}
					<ul class="member-list">
						{#each group.members as member (member.id)}
							<li class="member-row">
								<span>{member.name}</span>
								{#if myCharacters.some((c) => c.id === member.id)}
									<button type="button" onclick={() => handleLeave(member.id)}>Leave</button>
								{/if}
							</li>
						{/each}
					</ul>
				{/if}

				{#if joinable.length > 0}
					<form class="join-form" onsubmit={handleJoin}>
						<h3>Join with a character</h3>
						<div class="join-row">
							<label for="join-character">Character</label>
							<select id="join-character" bind:value={joinCharacterId} required>
								<option value="" disabled>Pick a character…</option>
								{#each joinable as character (character.id)}
									<option value={character.id}>{character.name}</option>
								{/each}
							</select>
							<SubmitButton pending={joining} label="Join" pendingLabel="Joining…" />
						</div>
					</form>
				{/if}
			</section>
		{/snippet}

		{#snippet inventoryPanel()}
			{#if canSeeContent}
				<InventoryPanel {owner} />
			{:else}
				<p class="empty-state">
					Join the group with one of your characters to see shared inventory.
				</p>
			{/if}
		{/snippet}

		{#snippet moneyPanel()}
			{#if canSeeContent}
				<MoneyPanel {owner} />
			{:else}
				<p class="empty-state">Join the group with one of your characters to see shared money.</p>
			{/if}
		{/snippet}

		<Tabs
			tabs={[
				{ id: 'members', label: 'Members', panel: membersPanel },
				{ id: 'inventory', label: 'Inventory', panel: inventoryPanel },
				{ id: 'money', label: 'Money', panel: moneyPanel }
			]}
		/>
	{/if}
</main>

<style>
	.group-meta {
		margin: 0.25rem 0 0;
		color: var(--color-muted);
		font-size: 0.9rem;
	}

	.member-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
	}

	.member-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--color-border);
		border-radius: 5px;
	}

	.join-form {
		margin-top: 1.25rem;
		border-top: 1px solid var(--color-border);
		padding-top: 1rem;
	}

	.join-form h3 {
		margin: 0 0 0.6rem;
		font-size: 0.95rem;
	}

	.join-row {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		flex-wrap: wrap;
	}

	.join-row label {
		font-size: 0.875rem;
	}

	.join-row select {
		flex: 1;
		min-width: 160px;
	}
</style>
