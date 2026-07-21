import { describe, it, expect } from 'vitest';
import {
	parseTags,
	newDocumentSection,
	sharedCharacterNames,
	editFieldsFromDocument,
	buildDocumentUpdate
} from './document-editor';
import type { Character, Document, DocumentShare } from './types';

describe('parseTags', () => {
	it('trims and drops empty tags', () => {
		expect(parseTags('  a , b ,, c ,  ')).toEqual(['a', 'b', 'c']);
	});

	it('returns an empty array for a blank string', () => {
		expect(parseTags('   ')).toEqual([]);
	});
});

describe('newDocumentSection', () => {
	it('is blank and player-visible', () => {
		expect(newDocumentSection()).toEqual({ id: '', content: '', gm_only: false });
	});
});

describe('sharedCharacterNames', () => {
	const characters = [
		{ id: 'c1', name: 'Aria' },
		{ id: 'c2', name: 'Borin' }
	] as Character[];

	it('resolves share character ids to names', () => {
		const shares = [{ character_id: 'c2' }, { character_id: 'c1' }] as DocumentShare[];
		expect(sharedCharacterNames(shares, characters)).toEqual(['Borin', 'Aria']);
	});

	it('falls back to a neutral label for an unknown character', () => {
		const shares = [{ character_id: 'ghost' }] as DocumentShare[];
		expect(sharedCharacterNames(shares, characters)).toEqual(['a character']);
	});
});

describe('editFieldsFromDocument / buildDocumentUpdate', () => {
	const doc = {
		id: 'd1',
		title: 'Notes',
		path: '/notes',
		tags: ['lore', 'secret'],
		shared_on_game_day: 4,
		version: 7,
		sections: [{ id: 's1', content: 'hi', gm_only: false }]
	} as Document;

	it('flattens tags to a comma-separated string and copies sections', () => {
		const fields = editFieldsFromDocument(doc);

		expect(fields).toEqual({
			title: 'Notes',
			path: '/notes',
			tags: 'lore, secret',
			sharedOnGameDay: 4,
			sections: [{ id: 's1', content: 'hi', gm_only: false }],
			expectedVersion: 7
		});

		// sections are deep-copied, not the loaded doc's own array
		fields.sections[0].content = 'changed';
		expect(doc.sections[0].content).toBe('hi');
	});

	it('parses tags back to an array and threads force through on save', () => {
		const fields = editFieldsFromDocument(doc);
		fields.tags = 'a,  b ,';

		expect(buildDocumentUpdate(fields, true)).toEqual({
			title: 'Notes',
			path: '/notes',
			tags: ['a', 'b'],
			sharedOnGameDay: 4,
			sections: [{ id: 's1', content: 'hi', gm_only: false }],
			expectedVersion: 7,
			force: true
		});
	});
});
