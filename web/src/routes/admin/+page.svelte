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

<div>
	<div>
		<Card title="Users" text="Manage user accounts" href={resolve('/admin/users')} />
		<Card title="Characters" text="Manage all characters" href={resolve('/admin/characters')} />
	</div>
</div>
