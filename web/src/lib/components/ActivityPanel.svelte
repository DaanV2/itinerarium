<script lang="ts">
	import { onMount } from 'svelte';
	import { listCharacterActivity } from '$lib/api/activity';
	import { describeActivity, activityKindLabel } from '$lib/activity-view';
	import { getAccessToken } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import type { ActivityEntry } from '$lib/types';

	let { characterId }: { characterId: string } = $props();

	let entries = $state<ActivityEntry[]>([]);
	let loading = $state(true);
	let error = $state('');

	async function loadAll() {
		loading = true;
		try {
			entries = await listCharacterActivity(characterId, getAccessToken());
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load activity.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAll);
</script>

<section>
	<h2>Activity</h2>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Loading…</p>
	{:else if entries.length === 0}
		<p>Nothing has happened yet — as far as this character knows.</p>
	{:else}
		<ul>
			{#each entries as entry (entry.id)}
				<li>
					<p>
						Game day {entry.game_day} · {activityKindLabel(entry)}
					</p>
					<p>{describeActivity(entry)}</p>
				</li>
			{/each}
		</ul>
	{/if}
</section>
