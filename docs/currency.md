# Currency & Money

How money works in Itinerarium: what a currency is, how conversion ratios are
defined, and how a character's money is stored and accessed. For the practical
steps of defining currencies, see [currency-configuration.md](currency-configuration.md).

## Concepts

A **currency** is a GM-defined unit of money — gold, silver, credits, favours,
whatever the campaign uses. Currencies are campaign-wide: the same catalog is
shared by every inventory (character now; group and location in M2). A currency
has three fields:

| Field | Meaning |
|-------|---------|
| `code` | Short unique identifier, e.g. `gp` |
| `name` | Display name, e.g. `Gold` |
| `ratio` | Value of one unit in the **base unit** (see below) |

### Conversion ratios and the base unit

The `ratio` expresses how much one unit of the currency is worth in the
campaign's **base unit** — the smallest denomination, which itself has
`ratio` 1. For the classic "1 gold = 10 silver = 100 copper":

| Currency | `code` | `ratio` |
|----------|--------|---------|
| Copper (base) | `cp` | 1 |
| Silver | `sp` | 10 |
| Gold | `gp` | 100 |
| Platinum | `pp` | 1000 |

A value in base units is `amount × ratio`. So 3 gp = 300 cp, and to convert
between two currencies you go through the base unit: 3 gp = 300 cp = 30 sp.

Ratios are **whole numbers ≥ 1**. Keeping them integers means all money
arithmetic stays in whole base units, so balances never accumulate
floating-point rounding error. Pick your smallest real denomination as the base
(`ratio` 1) and express everything else as a multiple of it.

### How amounts are stored

A character's money is stored **per currency, in that currency's own unit** —
not pre-converted to base units. A character holding "150 gold" has a balance
row of `amount: 150` against the gold currency. Balances themselves are never
auto-converted or merged server-side; the [calculator endpoints](#currency-calculator)
below do the conversion math on demand from amounts the caller supplies, they
don't read or write any balance.

A consequence worth knowing: **changing a currency's ratio does not rescale
existing balances.** A character with 150 gp still has 150 gp after you change
gold's ratio — only the *base-unit value* of that 150 gp changes. Ratios
describe relative worth, not the stored amount.

## Per-character money (M1)

In M1, money is held per character:

- A character has at most **one balance per currency** (a `MoneyBalance` row).
- Setting money is **absolute**: `SetMoney` writes the new total (it upserts the
  balance row), and amounts must be **≥ 0**.
- Setting a balance against a currency that is not in the catalog is rejected.

> M1 scopes money to characters. M2 adds shared group money and location money;
> the same currency catalog and ratios apply to all of them.

## Permissions

| Action | Who |
|--------|-----|
| Read the currency catalog | Any authenticated user (currencies are not secret) |
| Add/edit a currency | GM only |
| Read a character's money | The owning player + GM only |
| Set a character's money | The owning player + GM only |

A character's money follows the same visibility as the character itself. A
caller who is neither the owner nor a GM receives **404, not 403** — the
existence of the character (and therefore its money) is hidden, never confirmed
through an error code. This is enforced server-side in the service layer; see
[architecture.md](architecture.md#inventory--currency).

## API

| Method & path | Who | Purpose |
|---------------|-----|---------|
| `GET /api/currencies` | any authenticated | List the currency catalog |
| `POST /api/currencies` | GM | Add a currency (`{ code, name, ratio }`) |
| `GET /api/characters/{id}/money` | owner + GM | List a character's balances |
| `PUT /api/characters/{id}/money/{currencyId}` | owner + GM | Set a balance to an absolute `{ amount }` |

Balances are returned highest-value denomination first (by ratio). Each balance
carries its `currency_id`; join it against `GET /api/currencies` to render names
and compute conversions.

## Currency calculator

Two stateless endpoints do conversion/addition/simplification math over the
catalog's ratios. Neither reads nor writes any balance — they take amounts
in the request body and return a computed result. Both are open to **any
authenticated user**, same as reading the catalog. A currency in the request
body may be identified by its `code` (e.g. `"pp"`) or its `id` — either works.

| Method & path | Purpose |
|---------------|---------|
| `POST /api/currencies/convert` | Add up one or more currency amounts and express the total in a target currency |
| `POST /api/currencies/simplify` | Add up one or more currency amounts and break the total into the fewest coins across the whole catalog |

### Convert (and add)

`POST /api/currencies/convert` answers "how much of X is Y" — and, since
`amounts` is a list, also adds amounts across different currencies together
before converting. A single-entry list is a plain conversion; more than one
entry sums them first.

```json
// Request: how much is 5 platinum in silver?
{ "amounts": [{ "currency": "pp", "amount": 5 }], "to": "sp" }

// Response
{ "currency": { "id": "...", "code": "sp", "name": "Silver", "ratio": 10 },
  "whole": 500, "remainder": 0, "base_value": 5000 }
```

`base_value` is the sum in the campaign's base unit. `whole` is how many
units of the target currency that's worth; `remainder` is whatever is left
over, in base units, when the total isn't an exact multiple of the target's
ratio (e.g. converting 137 copper into gold at ratio 100 gives `whole: 1,
remainder: 37` — 37 copper doesn't make a whole gold piece).

### Simplify

`POST /api/currencies/simplify` adds up the given amounts and re-expresses
the total as the fewest coins: greedily filling the highest-ratio currency
first, then the next, down to the base unit. Denominations the total doesn't
need are omitted from the response.

```json
// Request: simplify 1234 copper (with cp=1, sp=10, gp=100 in the catalog)
{ "amounts": [{ "currency": "cp", "amount": 1234 }] }

// Response
[
  { "currency": { "id": "...", "code": "gp", "name": "Gold", "ratio": 100 }, "amount": 12 },
  { "currency": { "id": "...", "code": "sp", "name": "Silver", "ratio": 10 }, "amount": 3 },
  { "currency": { "id": "...", "code": "cp", "name": "Copper", "ratio": 1 }, "amount": 4 }
]
```

The greedy approach is optimal for standard denomination systems (each
higher ratio a clean multiple of the ones below it, as `docs/currency.md`
recommends); it is not a general knapsack solver.

## See also

- [currency-configuration.md](currency-configuration.md) — defining currencies via the catalog file or the API
- [architecture.md](architecture.md#inventory--currency) — the entity and permission model (source of truth)
- [features.md](features.md#items--currencies) — the player/GM-facing feature description
