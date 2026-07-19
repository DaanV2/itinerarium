<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { listRepositories } from '$lib/api/repositories';
	import { listCharacters } from '$lib/api/characters';
	import { listGroups } from '$lib/api/groups';
	import { getAccessToken } from '$lib/auth-token';
	import Card from '$lib/components/Card.svelte';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import type { Character, Group, Repository } from '$lib/types';

	let repositories = $state<Repository[]>([]);
	let characters = $state<Character[]>([]);
	let groups = $state<Group[]>([]);
	let loading = $state(true);
	let error = $state('');

	function repositoryLabel(repo: Repository): string {
		switch (repo.type) {
			case 'general':
				return 'General';
			case 'template':
				return 'Templates';
			case 'group': {
				const group = groups.find((g) => g.id === repo.group_id);
				return group ? `${group.name} (group)` : 'Group repository';
			}
			case 'character': {
				const character = characters.find((c) => c.id === repo.character_id);
				return character ? `${character.name} (character)` : 'Character repository';
			}
		}
	}

	async function loadAll() {
		loading = true;
		const token = getAccessToken();
		try {
			[repositories, characters, groups] = await Promise.all([
				listRepositories(token),
				listCharacters(token),
				listGroups(token)
			]);
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load repositories.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAll);
</script>

<main class="main-page">
	<h1>Repositories</h1>

	<p>
		<a href={resolve('/repositories/import')}>Import an Obsidian vault…</a>
	</p>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading…</p>
	{:else if repositories.length === 0}
		<p>No repositories you can see yet.</p>
	{:else}
		<div class="repo-list">
			{#each repositories as repo (repo.id)}
				<Card title={repositoryLabel(repo)} href={resolve('/repositories/[id]', { id: repo.id })} />
			{/each}
		</div>
	{/if}
</main>

<style>
	.repo-list {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}
</style>
