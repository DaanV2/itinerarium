# Configuring Currencies

Two ways to define the currencies (and item catalog) for a campaign:

1. **Seed file** — a JSON/YAML file loaded on server startup. Best for setting a
   campaign up once and keeping it in version control.
2. **API at runtime** — a GM adds entries through the API without a restart.

Both write to the same catalog. For what currencies *are* and how ratios work,
read [currency.md](currency.md) first.

## Seeding from a file

Point the API at a catalog file with the `catalog.path` setting. Like every
setting it can come from a flag, an environment variable, or the YAML config
(priority: flags → env → config file → defaults):

| Source | Value |
|--------|-------|
| Flag | `--catalog.path config/catalog.example.yaml` |
| Environment variable | `CATALOG_PATH=config/catalog.example.yaml` |
| `config/config.yaml` | `catalog.path: "config/catalog.example.yaml"` |

When set, the file is loaded during startup, after migrations. Leaving it unset
skips seeding entirely — the catalog is then managed purely through the API. The
path is resolved relative to the API's working directory (in Docker that is
`/app`, so a bundled `config/catalog.yaml` is `config/catalog.yaml`).

### File format

The file has two top-level lists, `currencies` and `items`; either may be
omitted. A ready-to-copy example lives at
[`config/catalog.example.yaml`](../config/catalog.example.yaml):

```yaml
currencies:
  - code: cp
    name: Copper
    ratio: 1        # base unit
  - code: sp
    name: Silver
    ratio: 10       # 1 sp = 10 cp
  - code: gp
    name: Gold
    ratio: 100      # 1 gp = 100 cp

items:
  - name: Torch
    description: Sheds bright light in a 20-foot radius.
    category: Adventuring Gear
  - name: Healing Potion   # description and category are optional
```

| Section | Field | Required | Notes |
|---------|-------|----------|-------|
| `currencies` | `code` | yes | Unique short identifier |
| `currencies` | `name` | yes | Display name |
| `currencies` | `ratio` | yes | Whole number ≥ 1; the base unit is `1` |
| `items` | `name` | yes | Unique item name |
| `items` | `description` | no | Free text |
| `items` | `category` | no | Free text, for grouping in the UI |

JSON works too — the loader parses JSON and YAML with the same decoder, so a
`.json` file with the same shape is equally valid.

### Upsert behaviour

Seeding is **idempotent** and safe to run on every startup:

- Currencies are matched by `code`, items by `name`.
- A matching entry is **updated in place** (name/ratio for currencies;
  description/category for items); a new entry is **created**.
- Nothing is deleted. Removing an entry from the file does **not** remove it from
  the database — delete it through other means if you need to (there is no
  delete endpoint yet).

So the normal workflow is: edit the file, restart the API, and the changes are
reconciled. Because [changing a ratio never rescales stored balances](currency.md#how-amounts-are-stored),
adjusting ratios between sessions is safe.

### Validation and startup failure

Every entry is validated as it loads. If any currency has an empty `code`/`name`
or a `ratio` below 1, or any item has an empty `name`, **the load fails and the
server refuses to start** with an error naming the offending entry. Fix the file
and start again. This is deliberate: a half-applied catalog is worse than a
clear startup error.

On success the server logs a summary, e.g. `catalog seeded  currencies=4 items=5`.

## Managing currencies through the API (GM)

You do not need a file at all — or you can seed a base set from a file and add
more at runtime. A GM adds a currency with:

```bash
curl -X POST http://localhost:8080/api/currencies \
  -H "Authorization: Bearer $GM_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"code":"favor","name":"Guild Favor","ratio":50}'
```

Item definitions use the same pattern against `POST /api/items` with
`{ "name", "description?", "category?" }`. Both endpoints are **GM-only**;
a player calling them receives `403`. Duplicate `code`/`name` returns `409`.

Remember that the item catalog is a **convenience, not a restriction**: players
can always add free-text items to an inventory that are not in the catalog, so
you only need to seed the items worth offering as presets.

## See also

- [currency.md](currency.md) — currency concepts, ratios, money storage, and the API
- [deployment.md](deployment.md#configuration) — the full configuration key reference
- [`config/catalog.example.yaml`](../config/catalog.example.yaml) — the annotated example file
