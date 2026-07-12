<script lang="ts">
	import { isGM } from '$lib/auth-token';
	import type { Snippet } from 'svelte';

	// Client-side visual gate only — a UX cue that this content/action is
	// GM-only. The API is the real enforcement (see docs/development.md).
	let { children }: { children: Snippet } = $props();
</script>

{#if isGM()}
	<div class="gm-only">
		<p class="gm-notice-text">Admin Only</p>
		{@render children()}
	</div>
{/if}

<style>
	.gm-only {
		background-color: rgba(34, 197, 94, 0.08);
		border: 1px dashed rgba(34, 197, 94, 0.6);
		border-radius: 5px;
		padding: 1rem;
	}

	.gm-notice-text {
		font-size: 0.875rem;
		font-weight: 500;
		font: bold;
		color: rgba(34, 197, 94, 0.8);
		margin-bottom: 0.5rem;
	}
</style>
