<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { createInitialAccount } from '$lib/api/setup';
	import { setAccessToken, setUserRole } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import FormField from '$lib/components/FormField.svelte';
	import SubmitButton from '$lib/components/SubmitButton.svelte';

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
			setAccessToken(account.access_token);
			// The setup wizard only ever creates the initial GM account.
			setUserRole('gm');
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
		<FormField
			id="email"
			label="Email"
			type="email"
			required
			autocomplete="username"
			bind:value={email}
		/>

		<FormField
			id="password"
			label="Password"
			type="password"
			required
			minlength={8}
			autocomplete="new-password"
			bind:value={password}
		/>

		<FormField
			id="confirm-password"
			label="Confirm password"
			type="password"
			required
			autocomplete="new-password"
			bind:value={confirmPassword}
		/>

		<ErrorAlert message={error} />

		<SubmitButton pending={submitting} label="Create GM account" pendingLabel="Creating account…" />
	</form>
</main>
