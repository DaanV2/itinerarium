import { describe, it, expect } from 'vitest';
import {
	newLocationSection,
	grantLabel,
	editFieldsFromLocation,
	buildLocationUpdate
} from './location-editor';
import type { Character, Group, Location, LocationAccess } from './types';

const characters = [{ id: 'c1', name: 'Aria' }] as Character[];
const groups = [{ id: 'g1', name: 'The Guild' }] as Group[];

describe('newLocationSection', () => {
	it('is blank and player-visible', () => {
		expect(newLocationSection()).toEqual({ id: '', content: '', gm_only: false });
	});
});

describe('grantLabel', () => {
	it('labels a character grant by name', () => {
		const grant = { id: 'a1', character_id: 'c1' } as LocationAccess;
		expect(grantLabel(grant, characters, groups)).toBe('Character: Aria');
	});

	it('labels a group grant by name', () => {
		const grant = { id: 'a2', group_id: 'g1' } as LocationAccess;
		expect(grantLabel(grant, characters, groups)).toBe('Group: The Guild');
	});

	it('falls back to the raw id when the target is unknown', () => {
		const grant = { id: 'a3', character_id: 'ghost' } as LocationAccess;
		expect(grantLabel(grant, characters, groups)).toBe('Character: ghost');
	});
});

describe('editFieldsFromLocation / buildLocationUpdate', () => {
	const location = {
		id: 'l1',
		name: 'The Keep',
		plane: 'Material',
		shared_on_game_day: 2,
		sections: [{ id: 's1', content: 'A tall keep.', gm_only: false }]
	} as Location;

	it('snapshots the location and deep-copies sections', () => {
		const fields = editFieldsFromLocation(location);

		expect(fields).toEqual({
			name: 'The Keep',
			plane: 'Material',
			sharedOnGameDay: 2,
			sections: [{ id: 's1', content: 'A tall keep.', gm_only: false }]
		});

		fields.sections[0].content = 'changed';
		expect(location.sections[0].content).toBe('A tall keep.');
	});

	it('defaults a missing plane to an empty string', () => {
		const noPlane = { ...location, plane: undefined } as Location;
		expect(editFieldsFromLocation(noPlane).plane).toBe('');
	});

	it('builds the snake_case update payload', () => {
		const fields = editFieldsFromLocation(location);
		expect(buildLocationUpdate(fields)).toEqual({
			name: 'The Keep',
			plane: 'Material',
			shared_on_game_day: 2,
			sections: [{ id: 's1', content: 'A tall keep.', gm_only: false }]
		});
	});
});
