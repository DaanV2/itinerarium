<script lang="ts">
	import { onMount } from 'svelte';
	import {
		listInventory,
		addInventoryItem,
		removeInventoryItem,
		moveInventoryItem
	} from '$lib/api/inventory';
	import { listCharacters } from '$lib/api/characters';
	import { listGroups } from '$lib/api/groups';
	import { listLocations } from '$lib/api/locations';
	import { listItemDefinitions } from '$lib/api/catalog';
	import { getAccessToken } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import FormField from '$lib/components/FormField.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import SubmitButton from '$lib/components/SubmitButton.svelte';
	import type { InventoryItem, InventoryOwnerRef, ItemDefinition } from '$lib/types';

	let { owner }: { owner: InventoryOwnerRef } = $props();

	interface MoveTarget {
		ref: InventoryOwnerRef;
		label: string;
	}

	let items = $state<InventoryItem[]>([]);
	let itemDefinitions = $state<ItemDefinition[]>([]);
	let moveTargets = $state<MoveTarget[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Add-item modal + form state.
	let showAddModal = $state(false);
	let addError = $state('');
	let itemName = $state('');
	let itemDefinitionId = $state('');
	let itemQuantity = $state(1);
	let addingItem = $state(false);

	// Per-item move form state: which line is open, where to, how many.
	let movingItemId = $state('');
	let moveTargetIndex = $state(-1);
	let moveQuantity = $state(1);
	let movePending = $state(false);

	function sameOwner(ref: InventoryOwnerRef): boolean {
		return ref.kind === owner.kind && ref.id === owner.id;
	}

	/** Everywhere the caller could move an item: their characters, groups one
	 * of their characters belongs to, and locations they can see — minus this
	 * inventory itself. The server re-checks access on every move. */
	async function loadMoveTargets(token: string): Promise<MoveTarget[]> {
		const [characters, groups, locations] = await Promise.all([
			listCharacters(token),
			listGroups(token),
			listLocations(token)
		]);

		const characterIds = new Set(characters.map((c) => c.id));
		const targets: MoveTarget[] = [
			...characters.map((c) => ({
				ref: { kind: 'character', id: c.id } as InventoryOwnerRef,
				label: `Character: ${c.name}`
			})),
			...groups
				.filter((g) => g.members.some((m) => characterIds.has(m.id)))
				.map((g) => ({
					ref: { kind: 'group', id: g.id } as InventoryOwnerRef,
					label: `Group: ${g.name}`
				})),
			...locations.map((l) => ({
				ref: { kind: 'location', id: l.id } as InventoryOwnerRef,
				label: `Location: ${l.name}`
			}))
		];

		return targets.filter((t) => !sameOwner(t.ref));
	}

	async function loadAll() {
		loading = true;
		const token = getAccessToken();
		try {
			[items, itemDefinitions, moveTargets] = await Promise.all([
				listInventory(owner, token),
				listItemDefinitions(token),
				loadMoveTargets(token)
			]);
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load inventory.';
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

	async function refreshItems() {
		items = await listInventory(owner, getAccessToken());
	}

	function openAddModal() {
		addError = '';
		showAddModal = true;
	}

	async function handleAddItem(event: SubmitEvent) {
		event.preventDefault();
		addError = '';
		addingItem = true;
		try {
			await addInventoryItem(
				owner,
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
			showAddModal = false;
			await refreshItems();
		} catch (err) {
			addError = err instanceof Error ? err.message : 'Failed to add item.';
		} finally {
			addingItem = false;
		}
	}

	async function handleRemoveItem(itemId: string) {
		error = '';
		try {
			await removeInventoryItem(owner, itemId, getAccessToken());
			await refreshItems();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to remove item.';
		}
	}

	function toggleMove(item: InventoryItem) {
		if (movingItemId === item.id) {
			movingItemId = '';
			return;
		}
		movingItemId = item.id;
		moveTargetIndex = -1;
		moveQuantity = item.quantity;
	}

	async function handleMove(event: SubmitEvent) {
		event.preventDefault();
		const target = moveTargets[moveTargetIndex];
		if (!target) return;

		error = '';
		movePending = true;
		try {
			await moveInventoryItem(movingItemId, target.ref, moveQuantity, getAccessToken());
			movingItemId = '';
			await refreshItems();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to move item.';
		} finally {
			movePending = false;
		}
	}
</script>

<section>
	<h2>Inventory</h2>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading...</p>
	{:else}
		<div class="inventory-header">
			<button type="button" class="add-btn" onclick={openAddModal} aria-label="Add item">+</button>
		</div>

		{#if items.length === 0}
			<p class="empty-state">No items yet. Use the + button to add one.</p>
		{:else}
			<table class="inventory-table">
				<thead class="inventory-table-head">
					<tr>
						<th scope="col">Name</th>
						<th scope="col">Quantity</th>
						<th scope="col">Description</th>
						<th scope="col">Actions</th>
					</tr>
				</thead>
				<tbody class="inventory-table-body">
					{#each items as item (item.id)}
						<tr>
							<td>{item.name}</td>
							<td>{item.quantity}</td>
							<td>{item.description}</td>
							<td>
								<button type="button" onclick={() => toggleMove(item)}>Move</button>
								<button type="button" onclick={() => handleRemoveItem(item.id)}>Remove</button>

								{#if movingItemId === item.id}
									<form onsubmit={handleMove}>
										<label for="move-target-{item.id}">To</label>
										<select id="move-target-{item.id}" bind:value={moveTargetIndex} required>
											<option value={-1} disabled>Pick a destination...</option>
											{#each moveTargets as target, index (target.label)}
												<option value={index}>{target.label}</option>
											{/each}
										</select>

										<FormField
											id="move-qty-{item.id}"
											label="Quantity"
											type="number"
											min={1}
											max={item.quantity}
											required
											bind:value={moveQuantity}
										/>

										<SubmitButton pending={movePending} label="Move" pendingLabel="Moving..." />
									</form>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}

		<Modal bind:open={showAddModal} title="Add item">
			<form onsubmit={handleAddItem}>
				<ErrorAlert message={addError} />

				<label for="item-def-{owner.kind}-{owner.id}">From catalog (optional)</label>
				<select
					id="item-def-{owner.kind}-{owner.id}"
					bind:value={itemDefinitionId}
					onchange={onPickDefinition}
				>
					<option value="">Free-text item...</option>
					{#each itemDefinitions as def (def.id)}
						<option value={def.id}>{def.name}</option>
					{/each}
				</select>

				<FormField
					id="item-name-{owner.kind}-{owner.id}"
					label="Name"
					type="text"
					required
					bind:value={itemName}
				/>

				<FormField
					id="item-qty-{owner.kind}-{owner.id}"
					label="Quantity"
					type="number"
					min={1}
					required
					bind:value={itemQuantity}
				/>

				<SubmitButton pending={addingItem} label="Add item" pendingLabel="Adding..." />
			</form>
		</Modal>
	{/if}
</section>

<style>
	.inventory-header {
		display: flex;
		justify-content: flex-end;
		margin-top: 0.75rem;
	}

	.add-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 2.5rem;
		height: 2.5rem;
		border-radius: 50%;
		border: none;
		background: var(--color-accent, #4f46e5);
		color: #fff;
		font-size: 1.5rem;
		line-height: 1;
		cursor: pointer;
	}

	.add-btn:hover {
		opacity: 0.85;
	}
</style>
