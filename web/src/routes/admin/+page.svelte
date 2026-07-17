<script lang="ts">
	import { isGM, isLoggedIn } from '$lib/auth-token';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import Card from '$lib/components/Card.svelte';

	$effect.pre(() => {
		if (!isLoggedIn()) {
			void goto(resolve('/login'));
		}

		if (!isGM()) {
			void goto(resolve('/'));
		}
	});
</script>

<main class="main-page">
	<h1>Admin Control</h1>
	<Card title="Users" text="Manage user accounts" href={resolve('/admin/users')} />
	<Card title="Characters" text="Manage all characters" href={resolve('/admin/characters')} />
	<Card title="Activity" text="Campaign log and announcements" href={resolve('/admin/activity')} />
</main>
