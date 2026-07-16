<script lang="ts">
	import { resolve } from '$app/paths';
	import type { FolderTreeNode } from '$lib/types';
	import FolderTree from './FolderTree.svelte';

	// Recursive: a folder tree node renders its own subfolders via itself.
	let { node }: { node: FolderTreeNode } = $props();
</script>

<ul class="tree-level">
	{#each node.folders as folder (folder.path)}
		<li>
			<details open>
				<summary>{folder.name}</summary>
				<FolderTree node={folder} />
			</details>
		</li>
	{/each}
	{#each node.documents as doc (doc.id)}
		<li>
			<a href={resolve('/documents/[id]', { id: doc.id })}>{doc.title}</a>
		</li>
	{/each}
</ul>

<style>
	.tree-level {
		list-style: none;
		margin: 0;
		padding-left: 1rem;
	}

	summary {
		cursor: pointer;
		font-weight: 600;
	}
</style>
