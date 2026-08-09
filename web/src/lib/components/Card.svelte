<script lang="ts">
	let {
		title,
		text,
		href,
		gm = false
	}: { title?: string | null; text?: string | null; href?: string | null; gm?: boolean } = $props();
</script>

{#snippet content()}
	{#if title}
		<h3>{title}</h3>
	{/if}
	{#if text}
		<p>{text}</p>
	{/if}
{/snippet}

{#if href}
	<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- href is caller-provided; callers resolve() typed internal routes before passing them in -->
	<a class="card" class:gm {href}>
		{@render content()}
	</a>
{:else}
	<div class:gm class="card">
		{@render content()}
	</div>
{/if}

<style>
	.card {
		display: block;
		border: 1px solid #ccc;
		border-radius: 5px;
		padding: 1rem;
		margin: 0.2rem;
		color: inherit;
		text-decoration: none;
		background-color: var(--tertiary-color);
		transition: all 0.2s ease-in-out;
	}

	a.card:hover {
		border-color: #888;
	}

	.card.gm {
		background-color: var(--gm-color);
		border: 1px dashed var(--gm-accent-color);
	}

	a.card.gm:hover {
		border-color: var(--gm-highlight-color);
	}

	.card h3 {
		margin: 0 0 0.5rem;
	}

	.card p {
		margin: 0;
	}
</style>
