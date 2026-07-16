import { describe, it, expect } from 'vitest';
import { describeAudience } from './document-reveal';
import type { Character, Group, Repository } from './types';

const groups: Group[] = [{ id: 'g1', name: 'The Guild', type: 'organization', members: [] }];
const characters: Character[] = [{ id: 'c1', name: 'Aria', current_game_day: 3, user_id: 'u1' }];

describe('describeAudience', () => {
	it('describes the general repository as everyone', () => {
		expect(describeAudience({ id: 'r1', type: 'general' }, [], [])).toBe('everyone');
	});

	it('describes the template repository as everyone, marked as templates', () => {
		expect(describeAudience({ id: 'r1', type: 'template' }, [], [])).toBe('everyone (templates)');
	});

	it('names the group for a group repository', () => {
		const repo: Repository = { id: 'r1', type: 'group', group_id: 'g1' };
		expect(describeAudience(repo, characters, groups)).toBe('The Guild members');
	});

	it('falls back when the group is unknown', () => {
		const repo: Repository = { id: 'r1', type: 'group', group_id: 'missing' };
		expect(describeAudience(repo, characters, groups)).toBe('group members');
	});

	it('names the character for a character repository', () => {
		const repo: Repository = { id: 'r1', type: 'character', character_id: 'c1' };
		expect(describeAudience(repo, characters, groups)).toBe('Aria and the GM');
	});

	it('falls back when the character is unknown', () => {
		const repo: Repository = { id: 'r1', type: 'character', character_id: 'missing' };
		expect(describeAudience(repo, characters, groups)).toBe('a character and the GM');
	});
});
