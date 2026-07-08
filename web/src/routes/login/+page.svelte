<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { login } from '$lib/api/auth';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let submitting = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		submitting = true;

		try {
			const account = await login(email, password);
			localStorage.setItem('itinerarium_access_token', account.access_token);
			await goto(resolve('/'));
		} catch (err) {
			error = err instanceof Error ? err.message : 'Login failed.';
		} finally {
			submitting = false;
		}
	}
</script>

<main>
	<h1>Log in</h1>

	<form onsubmit={handleSubmit}>
		<label for="email">Email</label>
		<input id="email" type="email" required autocomplete="username" bind:value={email} />

		<label for="password">Password</label>
		<input
			id="password"
			type="password"
			required
			autocomplete="current-password"
			bind:value={password}
		/>

		{#if error}
			<p role="alert">{error}</p>
		{/if}

		<button type="submit" disabled={submitting}>
			{submitting ? 'Logging in…' : 'Log in'}
		</button>
	</form>
</main>
