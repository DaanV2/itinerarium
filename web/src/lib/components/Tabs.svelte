<script lang="ts">
	import type { Snippet } from 'svelte';

	interface TabDef {
		id: string;
		label: string;
		panel: Snippet;
	}

	let { tabs }: { tabs: TabDef[] } = $props();

	let selectedId = $state('');
	let activeId = $derived(selectedId || tabs[0]?.id || '');
	let activePanel = $derived(tabs.find((t) => t.id === activeId)?.panel);
</script>

<div class="tabs">
	<div class="tab-list" role="tablist">
		{#each tabs as tab (tab.id)}
			<button
				role="tab"
				type="button"
				aria-selected={activeId === tab.id}
				onclick={() => (selectedId = tab.id)}
				class:active={activeId === tab.id}
			>
				{tab.label}
			</button>
		{/each}
	</div>

	<div class="tab-panel" role="tabpanel">
		{#if activePanel}
			{@render activePanel()}
		{/if}
	</div>
</div>

<style>
	.tabs {
		margin-top: 1.5rem;
	}

	.tab-list {
		display: flex;
		border-bottom: 2px solid var(--color-border, #e5e7eb);
	}

	button[role='tab'] {
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		margin-bottom: -2px;
		padding: 0.5rem 1.25rem;
		cursor: pointer;
		font-size: 0.95rem;
		color: var(--color-muted, #6b7280);
		transition:
			color 0.15s,
			border-color 0.15s;
	}

	button[role='tab']:hover {
		color: var(--color-text, #111827);
	}

	button[role='tab'].active {
		color: var(--color-accent, #4f46e5);
		border-bottom-color: var(--color-accent, #4f46e5);
		font-weight: 600;
	}

	.tab-panel {
		padding-top: 1rem;
	}
</style>
