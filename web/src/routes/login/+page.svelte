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

<div class="login-page">
	<div class="login-card">
		<h1 class="app-name">Itinerarium</h1>

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
	</div>
</div>

<style>
	.login-page {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 1rem;
		box-sizing: border-box;
	}

	.login-card {
		background-color: var(--secondary-color);
		border-radius: 8px;
		padding: 2rem 2.25rem;
		width: 100%;
		max-width: 360px;
		box-shadow: 0 6px 32px rgba(0, 0, 0, 0.2);
	}

	.app-name {
		margin: 0 0 1.75rem;
		font-size: 1.6rem;
		letter-spacing: -0.02em;
	}

	form :global(.field) {
		margin-top: 0.9rem;
	}

	form :global(.field:first-of-type) {
		margin-top: 0;
	}

	form :global(button[type='submit']) {
		width: 100%;
		margin-top: 1.25rem;
		padding: 0.6rem;
		background-color: var(--primary-color);
		border-color: transparent;
	}

	form :global(button[type='submit']:hover:not(:disabled)) {
		background-color: #111;
	}
</style>
