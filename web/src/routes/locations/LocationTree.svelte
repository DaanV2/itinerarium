<script lang="ts">
	import type { LocationNode } from '$lib/location-tree';
	import type { Location } from '$lib/types';
	import Self from './LocationTree.svelte';

	let { nodes, onDelete }: { nodes: LocationNode[]; onDelete: (location: Location) => void } =
		$props();
</script>

<ul>
	{#each nodes as node (node.location.id)}
		<li>
			<strong>{node.location.name}</strong>
			{#if node.location.description}
				— {node.location.description}
			{/if}
			<button type="button" onclick={() => onDelete(node.location)}>Delete</button>
			{#if node.children.length > 0}
				<Self nodes={node.children} {onDelete} />
			{/if}
		</li>
	{/each}
</ul>
