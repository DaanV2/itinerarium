<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getCharacter } from '$lib/api/characters';
	import {
		listInventory,
		addInventoryItem,
		removeInventoryItem,
		listMoney,
		setMoney
	} from '$lib/api/inventory';
	import { listCurrencies, listItemDefinitions } from '$lib/api/catalog';
	import { getAccessToken } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import FormField from '$lib/components/FormField.svelte';
	import SubmitButton from '$lib/components/SubmitButton.svelte';
	import { mergeBalances, type CurrencyBalance } from '$lib/inventory-view';
	import type {
		Character,
		Currency,
		InventoryItem,
		ItemDefinition,
		MoneyBalance
	} from '$lib/types';

	// Always present for this route; `?? ''` keeps the type a plain string.
	const characterId = page.params.id ?? '';

	let character = $state<Character | null>(null);
	let items = $state<InventoryItem[]>([]);
	let currencies = $state<Currency[]>([]);
	let balances = $state<MoneyBalance[]>([]);
	let itemDefinitions = $state<ItemDefinition[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Add-item form state.
	let itemName = $state('');
	let itemDefinitionId = $state('');
	let itemQuantity = $state(1);
	let addingItem = $state(false);

	let moneyRows = $derived<CurrencyBalance[]>(mergeBalances(currencies, balances));

	async function loadAll() {
		loading = true;
		const token = getAccessToken();
		try {
			[character, items, currencies, balances, itemDefinitions] = await Promise.all([
				getCharacter(characterId, token),
				listInventory(characterId, token),
				listCurrencies(token),
				listMoney(characterId, token),
				listItemDefinitions(token)
			]);
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load character.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAll);

	// When a catalog item is picked, mirror its name into the free-text field so
	// the label stays visible and editable.
	function onPickDefinition() {
		const def = itemDefinitions.find((d) => d.id === itemDefinitionId);
		if (def) {
			itemName = def.name;
		}
	}

	async function refreshInventory() {
		items = await listInventory(characterId, getAccessToken());
	}

	async function handleAddItem(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		addingItem = true;
		try {
			await addInventoryItem(
				characterId,
				{
					name: itemName,
					quantity: itemQuantity,
					item_definition_id: itemDefinitionId || undefined
				},
				getAccessToken()
			);
			itemName = '';
			itemDefinitionId = '';
			itemQuantity = 1;
			await refreshInventory();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to add item.';
		} finally {
			addingItem = false;
		}
	}

	async function handleRemoveItem(itemId: string) {
		error = '';
		try {
			await removeInventoryItem(characterId, itemId, getAccessToken());
			await refreshInventory();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to remove item.';
		}
	}

	async function handleSetMoney(currencyId: string, amount: number) {
		error = '';
		try {
			await setMoney(characterId, currencyId, amount, getAccessToken());
			balances = await listMoney(characterId, getAccessToken());
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to update money.';
		}
	}
</script>

<main>
	<p><a href={resolve('/characters')}>← Characters</a></p>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading…</p>
	{:else if !character}
		<p>Character not found.</p>
	{:else}
		<h1>{character.name}</h1>
		<p>Game day {character.current_game_day}</p>

		<section>
			<h2>Inventory</h2>
			{#if items.length === 0}
				<p>No items yet.</p>
			{:else}
				<ul>
					{#each items as item (item.id)}
						<li>
							{item.quantity}× {item.name}
							{#if item.description}<span> — {item.description}</span>{/if}
							<button type="button" onclick={() => handleRemoveItem(item.id)}>Remove</button>
						</li>
					{/each}
				</ul>
			{/if}

			<form onsubmit={handleAddItem}>
				<h3>Add item</h3>
				<label for="item-def">From catalog (optional)</label>
				<select id="item-def" bind:value={itemDefinitionId} onchange={onPickDefinition}>
					<option value="">Free-text item…</option>
					{#each itemDefinitions as def (def.id)}
						<option value={def.id}>{def.name}</option>
					{/each}
				</select>

				<FormField id="item-name" label="Name" type="text" required bind:value={itemName} />

				<FormField
					id="item-qty"
					label="Quantity"
					type="number"
					min={1}
					required
					bind:value={itemQuantity}
				/>

				<SubmitButton pending={addingItem} label="Add item" pendingLabel="Adding…" />
			</form>
		</section>

		<section>
			<h2>Money</h2>
			{#if moneyRows.length === 0}
				<p>No currencies defined. A GM can add them to the catalog.</p>
			{:else}
				<ul>
					{#each moneyRows as row (row.currency.id)}
						<li>
							<label for="money-{row.currency.id}">{row.currency.name} ({row.currency.code})</label>
							<input
								id="money-{row.currency.id}"
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
	{/if}
</main>
