import type { Currency, MoneyBalance } from '$lib/types';

/** A currency paired with the character's current amount in it (0 when there
 * is no balance row yet). Ordered highest-value denomination first. */
export interface CurrencyBalance {
	currency: Currency;
	amount: number;
}

/** Merges the currency catalog with a character's balances so the UI can show
 * every currency — including ones the character holds none of — in a stable,
 * highest-value-first order. */
export function mergeBalances(currencies: Currency[], balances: MoneyBalance[]): CurrencyBalance[] {
	const byCurrency = new Map(balances.map((b) => [b.currency_id, b.amount]));

	return [...currencies]
		.sort((a, b) => b.ratio - a.ratio || a.code.localeCompare(b.code))
		.map((currency) => ({ currency, amount: byCurrency.get(currency.id) ?? 0 }));
}
