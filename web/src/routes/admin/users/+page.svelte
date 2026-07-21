<script lang="ts">
	import { onMount } from 'svelte';
	import { createAccount, listAccounts, resetPassword } from '$lib/api/accounts';
	import { getAccessToken } from '$lib/auth-token';
	import CreateModal from '$lib/components/CreateModal.svelte';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import FormField from '$lib/components/FormField.svelte';
	import type { Account, Role } from '$lib/types';

	let accounts = $state<Account[]>([]);
	let loading = $state(true);
	let email = $state('');
	let role = $state<Role>('player');
	let error = $state('');
	let issuedCredential = $state<{ email: string; password: string } | null>(null);

	async function loadAccounts() {
		loading = true;
		try {
			accounts = await listAccounts(getAccessToken());
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load accounts.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAccounts);

	async function handleCreate() {
		const created = await createAccount(email, role, getAccessToken());
		issuedCredential = { email: created.email, password: created.temporary_password };
		email = '';
		role = 'player';
		await loadAccounts();
	}

	async function handleResetPassword(account: Account) {
		error = '';

		try {
			const result = await resetPassword(account.id, getAccessToken());
			issuedCredential = { email: account.email, password: result.temporary_password };
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to reset password.';
		}
	}
</script>

<main class="main-page">
	<h1>Accounts</h1>

	<ErrorAlert message={error} />

	{#if issuedCredential}
		<div role="status">
			<p>
				Temporary password for <strong>{issuedCredential.email}</strong> — hand this to the account holder
				now, it will not be shown again:
			</p>
			<pre>{issuedCredential.password}</pre>
		</div>
	{/if}

	<section>
		<CreateModal triggerLabel="Create account" pendingLabel="Creating…" onSubmit={handleCreate}>
			<FormField
				id="email"
				label="Email"
				type="email"
				required
				autocomplete="off"
				bind:value={email}
			/>

			<label for="role">Role</label>
			<select id="role" bind:value={role}>
				<option value="player">Player</option>
				<option value="gm">GM</option>
			</select>
		</CreateModal>
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
