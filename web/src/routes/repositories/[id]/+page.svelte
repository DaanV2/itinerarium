<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getDocumentFolderTree, getRepository } from '$lib/api/repositories';
	import { listCharacters } from '$lib/api/characters';
	import { listGroups } from '$lib/api/groups';
	import { getAccessToken } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import FolderTree from '$lib/components/FolderTree.svelte';
	import type { Character, FolderTreeNode, Group, Repository } from '$lib/types';

	const repositoryId = page.params.id ?? '';

	let repository = $state<Repository | null>(null);
	let tree = $state<FolderTreeNode | null>(null);
	let characters = $state<Character[]>([]);
	let groups = $state<Group[]>([]);
	let loading = $state(true);
	let error = $state('');

	let title = $derived.by(() => {
		if (!repository) return '';
		switch (repository.type) {
			case 'general':
				return 'General';
			case 'template':
				return 'Templates';
			case 'group':
				return groups.find((g) => g.id === repository?.group_id)?.name ?? 'Group repository';
			case 'character':
				return (
					characters.find((c) => c.id === repository?.character_id)?.name ?? 'Character repository'
				);
		}
	});

	async function loadAll() {
		loading = true;
		const token = getAccessToken();
		try {
			[repository, tree, characters, groups] = await Promise.all([
				getRepository(repositoryId, token),
				getDocumentFolderTree(repositoryId, token),
				listCharacters(token),
				listGroups(token)
			]);
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load repository.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAll);
</script>

<main class="main-page">
	<p><a href={resolve('/repositories')}>← Repositories</a></p>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading…</p>
	{:else if !repository || !tree}
		<p>Repository not found.</p>
	{:else}
		<h1>{title}</h1>

		{#if tree.folders.length === 0 && tree.documents.length === 0}
			<p>No documents you can see yet.</p>
		{:else}
			<FolderTree node={tree} />
		{/if}
	{/if}
</main>
