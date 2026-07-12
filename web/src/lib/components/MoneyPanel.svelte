<script lang="ts">
	import { onMount } from 'svelte';
	import { listMoney, setMoney } from '$lib/api/inventory';
	import { listCurrencies } from '$lib/api/catalog';
	import { getAccessToken } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import { mergeBalances, type CurrencyBalance } from '$lib/inventory-view';
	import type { Currency, InventoryOwnerRef, MoneyBalance } from '$lib/types';

	let { owner }: { owner: InventoryOwnerRef } = $props();

	let currencies = $state<Currency[]>([]);
	let balances = $state<MoneyBalance[]>([]);
	let loading = $state(true);
	let error = $state('');

	let moneyRows = $derived<CurrencyBalance[]>(mergeBalances(currencies, balances));

	async function loadAll() {
		loading = true;
		const token = getAccessToken();
		try {
			[currencies, balances] = await Promise.all([listCurrencies(token), listMoney(owner, token)]);
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load money.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAll);

	async function handleSetMoney(currencyId: string, amount: number) {
		error = '';
		try {
			await setMoney(owner, currencyId, amount, getAccessToken());
			balances = await listMoney(owner, getAccessToken());
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to update money.';
		}
	}
</script>

<section>
	<h2>Money</h2>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading…</p>
	{:else if moneyRows.length === 0}
		<p>No currencies defined. A GM can add them to the catalog.</p>
	{:else}
		<ul>
			{#each moneyRows as row (row.currency.id)}
				<li>
					<label for="money-{owner.kind}-{owner.id}-{row.currency.id}">
						{row.currency.name} ({row.currency.code})
					</label>
					<input
						id="money-{owner.kind}-{owner.id}-{row.currency.id}"
						type="number"
						min="0"
						value={row.amount}
						onchange={(e) =>
							handleSetMoney(row.currency.id, Number((e.target as HTMLInputElement).value))}
					/>
				</li>
			{/each}
		</ul>
	{/if}
</section>
