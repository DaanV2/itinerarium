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
row of `amount: 150` against the gold currency. The API does not auto-convert
between currencies; it returns each balance and the currency ratios, and the
client derives conversions when it needs to display a combined total.

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

## See also

- [currency-configuration.md](currency-configuration.md) — defining currencies via the catalog file or the API
- [architecture.md](architecture.md#inventory--currency) — the entity and permission model (source of truth)
- [features.md](features.md#items--currencies) — the player/GM-facing feature description
