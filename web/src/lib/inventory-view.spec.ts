import { describe, it, expect } from 'vitest';
import { mergeBalances } from './inventory-view';
import type { Currency, MoneyBalance } from '$lib/types';

const gp: Currency = { id: 'gp-id', code: 'gp', name: 'Gold', ratio: 100 };
const sp: Currency = { id: 'sp-id', code: 'sp', name: 'Silver', ratio: 10 };
const cp: Currency = { id: 'cp-id', code: 'cp', name: 'Copper', ratio: 1 };

describe('mergeBalances', () => {
	it('orders currencies highest-value first', () => {
		const rows = mergeBalances([cp, gp, sp], []);
		expect(rows.map((r) => r.currency.code)).toEqual(['gp', 'sp', 'cp']);
	});

	it('fills in the amount for held currencies and 0 for the rest', () => {
		const balances: MoneyBalance[] = [
			{ id: 'b1', character_id: 'c1', currency_id: 'gp-id', amount: 42 }
		];

		const rows = mergeBalances([gp, sp], balances);

		expect(rows).toEqual([
			{ currency: gp, amount: 42 },
			{ currency: sp, amount: 0 }
		]);
	});

	it('does not mutate the input array', () => {
		const input = [cp, gp];
		mergeBalances(input, []);
		expect(input).toEqual([cp, gp]);
	});
});
