// Pure helpers for the document edit page. Kept out of the `.svelte` file so
// the tag/section/update-payload logic is testable without mounting anything —
// the same split the repo already uses for `document-reveal.ts` and friends.

import type { UpdateDocumentInput } from '$lib/api/documents';
import type { Character, Document, DocumentSection, DocumentShare } from '$lib/types';

/** Splits the comma-separated tag input into trimmed, non-empty tags. */
export function parseTags(raw: string): string[] {
	return raw
		.split(',')
		.map((t) => t.trim())
		.filter(Boolean);
}

/** A blank, player-visible section for the "Add section" button. */
export function newDocumentSection(): DocumentSection {
	return { id: '', content: '', gm_only: false };
}

/** The editor's "Also directly shared with …" names, resolved from the GM-only
 * share list against the character roster. Falls back to a neutral label for a
 * share whose character isn't in the roster. */
export function sharedCharacterNames(shares: DocumentShare[], characters: Character[]): string[] {
	return shares.map((s) => characters.find((c) => c.id === s.character_id)?.name ?? 'a character');
}

/** The mutable fields the edit form binds to. */
export interface DocumentEditFields {
	title: string;
	path: string;
	/** Comma-separated, exactly as typed; parsed on save. */
	tags: string;
	sharedOnGameDay: number;
	sections: DocumentSection[];
	expectedVersion: number;
}

/** The empty form used before a document is loaded / while not editing. */
export function emptyDocumentEditFields(): DocumentEditFields {
	return { title: '', path: '', tags: '', sharedOnGameDay: 0, sections: [], expectedVersion: 0 };
}

/** Snapshots a loaded document into editable form fields — tags flattened to a
 * comma-separated string and sections deep-copied so edits don't mutate the
 * loaded document. */
export function editFieldsFromDocument(doc: Document): DocumentEditFields {
	return {
		title: doc.title,
		path: doc.path,
		tags: doc.tags.join(', '),
		sharedOnGameDay: doc.shared_on_game_day,
		sections: doc.sections.map((s) => ({ ...s })),
		expectedVersion: doc.version
	};
}

/** Builds the API update payload from the edit fields, parsing the tag string.
 * `force` overrides a concurrent-edit conflict once the user has confirmed. */
export function buildDocumentUpdate(
	fields: DocumentEditFields,
	force: boolean
): UpdateDocumentInput {
	return {
		title: fields.title,
		path: fields.path,
		tags: parseTags(fields.tags),
		sharedOnGameDay: fields.sharedOnGameDay,
		sections: fields.sections,
		expectedVersion: fields.expectedVersion,
		force
	};
}
