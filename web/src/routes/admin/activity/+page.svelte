<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { announceActivity, listAllActivity } from '$lib/api/activity';
	import { listCharacters } from '$lib/api/characters';
	import { listGroups } from '$lib/api/groups';
	import { describeActivity, activityKindLabel } from '$lib/activity-view';
	import { getAccessToken, isGM, isLoggedIn } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import SubmitButton from '$lib/components/SubmitButton.svelte';
	import type { ActivityAction, ActivityEntry, Character, Group } from '$lib/types';

	$effect.pre(() => {
		if (!isLoggedIn()) {
			void goto(resolve('/login'));
		}

		if (!isGM()) {
			void goto(resolve('/'));
		}
	});

	let entries = $state<ActivityEntry[]>([]);
	let characters = $state<Character[]>([]);
	let groups = $state<Group[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Announcement form.
	let gameDay = $state(0);
	let action = $state<ActivityAction>('stolen');
	let entityType = $state('');
	let entityName = $state('');
	let actor = $state('');
	let isPublic = $state(false);
	let characterIds = $state<string[]>([]);
	let groupIds = $state<string[]>([]);
	let announcing = $state(false);

	const actions: ActivityAction[] = ['added', 'updated', 'removed', 'destroyed', 'stolen'];

	async function loadAll() {
		loading = true;
		const token = getAccessToken();
		try {
			[entries, characters, groups] = await Promise.all([
				listAllActivity(token),
				listCharacters(token),
				listGroups(token)
			]);
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load activity.';
		} finally {
			loading = false;
		}
	}

	onMount(loadAll);

	async function handleAnnounce(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		announcing = true;
		try {
			await announceActivity(
				{
					game_day: gameDay,
					action,
					entity_type: entityType || undefined,
					entity_name: entityName,
					actor: actor || undefined,
					public: isPublic,
					character_ids: characterIds,
					group_ids: groupIds
				},
				getAccessToken()
			);
			entityName = '';
			actor = '';
			characterIds = [];
			groupIds = [];
			isPublic = false;
			await loadAll();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to announce.';
		} finally {
			announcing = false;
		}
	}
</script>

<main class="main-page">
	<p><a href={resolve('/admin')}>← Admin</a></p>

	<h1>Campaign Activity</h1>

	<ErrorAlert message={error} />

	<section>
		<h2>Announce an event</h2>
		<p>
			Announcements surface to their targets at the chosen game day, even when the targets have no
			access to the thing itself. Players see what happened and to what — the actor stays GM-only.
		</p>

		<form onsubmit={handleAnnounce}>
			<label for="announce-day">Game day</label>
			<input id="announce-day" type="number" min="0" bind:value={gameDay} required />

			<label for="announce-action">Action</label>
			<select id="announce-action" bind:value={action}>
				{#each actions as a (a)}
					<option value={a}>{a}</option>
				{/each}
			</select>

			<label for="announce-entity-type">What kind of thing (optional)</label>
			<input id="announce-entity-type" bind:value={entityType} placeholder="item, document, …" />

			<label for="announce-entity-name">Name of the thing</label>
			<input id="announce-entity-name" bind:value={entityName} required />

			<label for="announce-actor">Actor (GM-only, hidden from players)</label>
			<input id="announce-actor" bind:value={actor} placeholder="Who did it" />

			<label>
				<input type="checkbox" bind:checked={isPublic} />
				Public — everyone sees it at the chosen game day
			</label>

			<label for="announce-characters">Target characters</label>
			<select id="announce-characters" multiple bind:value={characterIds}>
				{#each characters as character (character.id)}
					<option value={character.id}>{character.name}</option>
				{/each}
			</select>

			<label for="announce-groups">Target groups</label>
			<select id="announce-groups" multiple bind:value={groupIds}>
				{#each groups as group (group.id)}
					<option value={group.id}>{group.name}</option>
				{/each}
			</select>

			<SubmitButton pending={announcing} label="Announce" pendingLabel="Announcing…" />
		</form>
	</section>

	<section>
		<h2>Full log</h2>
		{#if loading}
			<p>Loading…</p>
		{:else if entries.length === 0}
			<p>No activity recorded yet.</p>
		{:else}
			<ul>
				{#each entries as entry (entry.id)}
					<li>
						<p>Game day {entry.game_day} · {activityKindLabel(entry)}</p>
						<p>{describeActivity(entry)}</p>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
</main>
