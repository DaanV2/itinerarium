// Pure helpers for the location detail/edit page: description-section editing,
// the GM access-grant label, and the edit-form <-> update-payload mapping.
// Kept out of the `.svelte` file so they are testable without mounting.

import type { LocationSectionInput } from '$lib/api/locations';
import type { Character, Group, Location, LocationAccess, LocationSection } from '$lib/types';

/** A blank, player-visible description section for the "Add section" button. */
export function newLocationSection(): LocationSection {
	return { id: '', content: '', gm_only: false };
}

/** Human label for one access grant, resolved against the character/group
 * rosters. Falls back to the raw id when the target isn't in the roster. */
export function grantLabel(
	grant: LocationAccess,
	characters: Character[],
	groups: Group[]
): string {
	if (grant.character_id) {
		const character = characters.find((c) => c.id === grant.character_id);
		return `Character: ${character?.name ?? grant.character_id}`;
	}
	const group = groups.find((g) => g.id === grant.group_id);
	return `Group: ${group?.name ?? grant.group_id}`;
}

/** The mutable fields the edit form binds to. */
export interface LocationEditFields {
	name: string;
	plane: string;
	sharedOnGameDay: number;
	sections: LocationSection[];
}

/** The empty form used before a location is loaded / while not editing. */
export function emptyLocationEditFields(): LocationEditFields {
	return { name: '', plane: '', sharedOnGameDay: 0, sections: [] };
}

/** Snapshots a loaded location into editable form fields — sections are
 * deep-copied so edits don't mutate the loaded location. */
export function editFieldsFromLocation(location: Location): LocationEditFields {
	return {
		name: location.name,
		plane: location.plane ?? '',
		sharedOnGameDay: location.shared_on_game_day,
		sections: location.sections.map((s) => ({ ...s }))
	};
}

/** Builds the API update payload from the edit fields. */
export function buildLocationUpdate(fields: LocationEditFields): {
	name: string;
	plane: string;
	shared_on_game_day: number;
	sections: LocationSectionInput[];
} {
	return {
		name: fields.name,
		plane: fields.plane,
		shared_on_game_day: fields.sharedOnGameDay,
		sections: fields.sections
	};
}
