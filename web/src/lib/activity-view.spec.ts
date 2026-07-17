import { describe, it, expect } from 'vitest';
import { activityKindLabel, describeActivity } from './activity-view';
import type { ActivityEntry } from './types';

function entry(overrides: Partial<ActivityEntry>): ActivityEntry {
	return {
		id: '1',
		game_day: 5,
		action: 'added',
		entity_name: 'Lockpicks',
		announced: false,
		created_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

describe('describeActivity', () => {
	it('describes an entity change with its actor', () => {
		expect(describeActivity(entry({ action: 'added', actor: 'Aria' }))).toBe(
			'Lockpicks was added by Aria'
		);
	});

	it('reads membership events actor-first', () => {
		expect(
			describeActivity(entry({ action: 'joined', entity_name: 'Thieves Guild', actor: 'Aria' }))
		).toBe('Aria joined Thieves Guild');
	});

	it('omits the actor half when the server stripped it', () => {
		// Announced entries reach players without an actor — never invent one.
		expect(
			describeActivity(
				entry({ action: 'stolen', entity_name: 'The Ruby of Vess', announced: true })
			)
		).toBe('The Ruby of Vess was stolen');
	});
});

describe('activityKindLabel', () => {
	it('labels announced entries as announcements', () => {
		expect(activityKindLabel(entry({ announced: true, entity_type: 'item' }))).toBe('announcement');
	});

	it('falls back to the entity type, then a generic label', () => {
		expect(activityKindLabel(entry({ entity_type: 'money' }))).toBe('money');
		expect(activityKindLabel(entry({ entity_type: undefined }))).toBe('event');
	});
});
