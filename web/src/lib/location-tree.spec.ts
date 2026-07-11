import { describe, it, expect } from 'vitest';
import { buildLocationTree } from './location-tree';
import type { Location } from '$lib/types';

describe('buildLocationTree', () => {
	it('returns parent-less locations as roots (planes), sorted by name', () => {
		const locations: Location[] = [
			{ id: '2', name: 'The Material Plane' },
			{ id: '1', name: 'The Feywild' }
		];

		const tree = buildLocationTree(locations);

		expect(tree.map((n) => n.location.name)).toEqual(['The Feywild', 'The Material Plane']);
		expect(tree.every((n) => n.children.length === 0)).toBe(true);
	});

	it('nests children under their parent, sorted by name', () => {
		const locations: Location[] = [
			{ id: 'plane', name: 'The Material Plane' },
			{ id: 'town', name: 'Neverwinter', parent_id: 'plane' },
			{ id: 'inn', name: 'The Moonstone Mask', parent_id: 'town' },
			{ id: 'town2', name: 'Baldur’s Gate', parent_id: 'plane' }
		];

		const tree = buildLocationTree(locations);

		expect(tree).toHaveLength(1);
		const plane = tree[0];
		expect(plane.location.id).toBe('plane');
		expect(plane.children.map((n) => n.location.name)).toEqual(['Baldur’s Gate', 'Neverwinter']);

		const neverwinter = plane.children.find((n) => n.location.id === 'town');
		expect(neverwinter?.children.map((n) => n.location.name)).toEqual(['The Moonstone Mask']);
	});

	it('treats a location whose parent is missing as a root', () => {
		const locations: Location[] = [{ id: 'orphan', name: 'Lost Room', parent_id: 'gone' }];

		const tree = buildLocationTree(locations);

		expect(tree).toHaveLength(1);
		expect(tree[0].location.id).toBe('orphan');
	});
});
