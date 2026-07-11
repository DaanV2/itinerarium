<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { login } from '$lib/api/auth';
	import { setAccessToken, setUserRole } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import FormField from '$lib/components/FormField.svelte';
	import SubmitButton from '$lib/components/SubmitButton.svelte';

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
			setAccessToken(account.access_token);
			setUserRole(account.role);
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
			autocomplete="current-password"
			bind:value={password}
		/>

		<ErrorAlert message={error} />

		<SubmitButton pending={submitting} label="Log in" pendingLabel="Logging in…" />
	</form>
</main>
