import type { Character, Group, Repository } from './types';

/** Describes who a document becomes visible to once its reveal day is
 * reached, based on the repository it lives in. This is the audience half of
 * the editor's "Revealed at game day X to …" banner — direct character
 * shares are additional to this and listed separately. */
export function describeAudience(
	repository: Repository,
	characters: Character[],
	groups: Group[]
): string {
	switch (repository.type) {
		case 'general':
			return 'everyone';
		case 'template':
			return 'everyone (templates)';
		case 'group': {
			const group = groups.find((g) => g.id === repository.group_id);
			return group ? `${group.name} members` : 'group members';
		}
		case 'character': {
			const character = characters.find((c) => c.id === repository.character_id);
			return character ? `${character.name} and the GM` : 'a character and the GM';
		}
	}
}
