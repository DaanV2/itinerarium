<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import { page } from '$app/state';
	import AdminNavBar from '$lib/components/AdminNavBar.svelte';
	import NavBar from '$lib/components/NavBar.svelte';

	let { children } = $props();

	const NO_NAV_ROUTES = ['/login', '/setup'];

	let showNav = $derived(!NO_NAV_ROUTES.includes(page.url.pathname));
	let isAdminRoute = $derived(page.url.pathname.startsWith('/admin'));
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{#if showNav}
	<div class="app-shell">
		{#if isAdminRoute}
			<AdminNavBar />
		{:else}
			<NavBar />
		{/if}
		<div class="app-content">
			{@render children()}
		</div>
	</div>
{:else}
	{@render children()}
{/if}

<style>
	.app-shell {
		display: flex;
		align-items: flex-start;
	}

	.app-content {
		flex: 1;
		min-width: 0;
	}
</style>
