<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { createInitialAccount } from '$lib/api/setup';

	let email = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let error = $state('');
	let submitting = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';

		if (password !== confirmPassword) {
			error = 'Passwords do not match.';
			return;
		}

		submitting = true;
		try {
			const account = await createInitialAccount(email, password);
			localStorage.setItem('itinerarium_access_token', account.access_token);
			await goto(resolve('/'));
		} catch (err) {
			error = err instanceof Error ? err.message : 'Setup failed.';
		} finally {
			submitting = false;
		}
	}
</script>

<main>
	<h1>Welcome to Itinerarium</h1>
	<p>This installation has no accounts yet. Create the game master account to get started.</p>

	<form onsubmit={handleSubmit}>
		<label for="email">Email</label>
		<input id="email" type="email" required autocomplete="username" bind:value={email} />

		<label for="password">Password</label>
		<input
			id="password"
			type="password"
			required
			minlength="8"
			autocomplete="new-password"
			bind:value={password}
		/>

		<label for="confirm-password">Confirm password</label>
		<input
			id="confirm-password"
			type="password"
			required
			autocomplete="new-password"
			bind:value={confirmPassword}
		/>

		{#if error}
			<p role="alert">{error}</p>
		{/if}

		<button type="submit" disabled={submitting}>
			{submitting ? 'Creating account…' : 'Create GM account'}
		</button>
	</form>
</main>
