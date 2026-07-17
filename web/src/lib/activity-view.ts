// Presentation helpers for activity entries. Pure functions kept out of the
// component so they are testable without mounting anything.

import type { ActivityEntry } from '$lib/types';

const ACTION_PHRASES: Record<string, string> = {
	joined: 'joined',
	left: 'left',
	added: 'was added',
	updated: 'was updated',
	removed: 'was removed',
	destroyed: 'was destroyed',
	stolen: 'was stolen'
};

/** Renders one entry as a human sentence, e.g. "Lockpicks was added by Aria".
 * Membership events read the other way around: the actor did the joining.
 * The actor half is omitted when the server stripped it (announced entries
 * for players) — never guess at who did something the API hid. */
export function describeActivity(entry: ActivityEntry): string {
	if (entry.action === 'joined' || entry.action === 'left') {
		const who = entry.actor || 'Someone';
		return `${who} ${entry.action} ${entry.entity_name}`;
	}

	const phrase = ACTION_PHRASES[entry.action] ?? entry.action;
	const sentence = `${entry.entity_name} ${phrase}`;

	return entry.actor ? `${sentence} by ${entry.actor}` : sentence;
}

/** A short label for what kind of thing the entry is about. */
export function activityKindLabel(entry: ActivityEntry): string {
	if (entry.announced) return 'announcement';

	return entry.entity_type || 'event';
}
