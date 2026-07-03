<script lang="ts">
	import { onMount } from 'svelte';
	import { createAccount, listAccounts, resetPassword } from '$lib/api/accounts';
	import type { Account, Role } from '$lib/types';

	let accounts = $state<Account[]>([]);
	let loading = $state(true);
	let email = $state('');
	let role = $state<Role>('player');
	let submitting = $state(false);
	let error = $state('');
	let issuedCredential = $state<{ email: string; password: string } | null>(null);

	function accessToken(): string {
		return localStorage.getItem('itinerarium_access_token') ?? '';
	}

	async function loadAccounts() {
		loading = true;
		try {
			accounts = await listAccounts(accessToken());
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load accounts.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAccounts);

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		submitting = true;

		try {
			const created = await createAccount(email, role, accessToken());
			issuedCredential = { email: created.email, password: created.temporary_password };
			email = '';
			role = 'player';
			await loadAccounts();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create account.';
		} finally {
			submitting = false;
		}
	}

	async function handleResetPassword(account: Account) {
		error = '';

		try {
			const result = await resetPassword(account.id, accessToken());
			issuedCredential = { email: account.email, password: result.temporary_password };
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to reset password.';
		}
	}
</script>

<main>
	<h1>Accounts</h1>

	{#if error}
		<p role="alert">{error}</p>
	{/if}

	{#if issuedCredential}
		<p role="status">
			Temporary password for <strong>{issuedCredential.email}</strong>:
			<code>{issuedCredential.password}</code>
			— hand this to the account holder now, it will not be shown again.
		</p>
	{/if}

	<section>
		<h2>Create account</h2>
		<form onsubmit={handleCreate}>
			<label for="email">Email</label>
			<input id="email" type="email" required autocomplete="off" bind:value={email} />

			<label for="role">Role</label>
			<select id="role" bind:value={role}>
				<option value="player">Player</option>
				<option value="gm">GM</option>
			</select>

			<button type="submit" disabled={submitting}>
				{submitting ? 'Creating…' : 'Create account'}
			</button>
		</form>
	</section>

	<section>
		<h2>Existing accounts</h2>
		{#if loading}
			<p>Loading…</p>
		{:else if accounts.length === 0}
			<p>No accounts yet.</p>
		{:else}
			<ul>
				{#each accounts as account (account.id)}
					<li>
						{account.email} ({account.role})
						<button type="button" onclick={() => handleResetPassword(account)}>
							Reset password
						</button>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
</main>
