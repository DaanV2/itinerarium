import type { Location } from '$lib/types';

/** A location together with its nested child locations, ready for recursive
 * rendering. */
export interface LocationNode {
	location: Location;
	children: LocationNode[];
}

/** Builds the plane/place tree from a flat location list. Top-level planes
 * (no `parent_id`, or a `parent_id` that isn't present in the list) become
 * roots; every other location hangs under its parent. Siblings are ordered by
 * name so the tree is stable regardless of input order. */
export function buildLocationTree(locations: Location[]): LocationNode[] {
	const nodes = new Map<string, LocationNode>(
		locations.map((location) => [location.id, { location, children: [] }])
	);

	const roots: LocationNode[] = [];
	for (const node of nodes.values()) {
		const parentId = node.location.parent_id;
		const parent = parentId ? nodes.get(parentId) : undefined;
		if (parent) {
			parent.children.push(node);
		} else {
			roots.push(node);
		}
	}

	const byName = (a: LocationNode, b: LocationNode) =>
		a.location.name.localeCompare(b.location.name);
	const sort = (list: LocationNode[]) => {
		list.sort(byName);
		for (const node of list) sort(node.children);
	};
	sort(roots);

	return roots;
}
