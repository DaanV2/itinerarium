<script lang="ts">
	import { isGM, isLoggedIn } from '$lib/auth-token';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import Card from '$lib/components/Card.svelte';

	let isgm = $state(false);

	$effect.pre(() => {
		if (!isLoggedIn()) {
			void goto(resolve('/login'));
		}

		isgm = isGM();
	});
</script>

<main class="main-page">
	{#if isgm}
		<Card text="Admin" href={resolve('/admin')} gm />
	{/if}

	<Card text="Characters" href={resolve('/characters')} />
	<Card text="Groups" href={resolve('/groups')} />
	<Card text="Locations" href={resolve('/locations')} />
</main>
