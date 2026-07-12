# web/ — SvelteKit frontend

Read the root `CLAUDE.md` first for domain rules. This app is **Svelte 5 with runes mode forced** (see `vite.config.ts`) — Svelte 4 syntax will not compile.

## Commands

Preferred: `just web-verify` from the repo root (lint + check + test + build), `just web` for the dev server, `just web-fmt` to format. Raw commands from `web/`:

```bash
npm run dev             # dev server on :5173, /api proxied to the Go API on :8080
npm run check           # svelte-check (type checking)
npm run lint            # prettier + eslint
npm run format          # auto-format
npm run test            # vitest, single run
npm run build           # production build (adapter-node)
```

Package manager is **npm** (plain `package-lock.json`, `npm ci` in CI/Docker) — don't introduce pnpm/yarn/bun.

Before finishing: `npm run format && npm run lint && npm run check && npm run test`.

CI runs Prettier and ESLint as two separate steps (`npx prettier --check .` then `npx eslint .`), not the combined `npm run lint`, so a Prettier-only failure is never masked by ESLint's exit code or vice versa — fix and verify each independently. On a checkout with `core.autocrlf=true` (common on Windows), `npm run lint` / `npx prettier --check .` over the **whole tree** will flag every pre-existing file with a line-ending diff that isn't a real problem, purely from CRLF vs. the LF committed in git. That noise makes it easy to miss a genuine formatting bug in a file you just touched — always run `npx prettier --check <the files you changed>` scoped to your own changes before finishing, even if the whole-tree check is red for unrelated reasons, and don't bulk-reformat the tree to silence it.

## Svelte 5 runes — required syntax

```svelte
<script lang="ts">
	// props
	let { character, onSave }: { character: Character; onSave: (c: Character) => void } = $props();

	// reactive state
	let count = $state(0);

	// computed
	let doubled = $derived(count * 2);

	// side effects (rarely needed — prefer $derived)
	$effect(() => {
		console.log(count);
	});
</script>

<!-- events are plain attributes, NOT on:click -->
<button onclick={() => count++}>+</button>
```

Never use: `export let`, `$:` reactive statements, `on:click`, stores for component-local state.

## Structure

```
src/
├── lib/
│   ├── api/            # typed API client — ALL fetch calls to the Go API live here
│   └── types.ts        # TypeScript types mirroring the API models
├── routes/             # +page.svelte / +page.ts / +layout.svelte per URL
└── app.html
```

`lib/components/` doesn't exist yet — create it for shared components once more than one route needs the same one; don't add it speculatively.

## Talking to the API

- Always call the API with **relative paths** (`/api/...`). In dev, Vite proxies them to `:8080`; in production a reverse proxy does.
- Load data in `+page.ts` `load` functions using the provided `fetch` (works server- and client-side):

```ts
// src/routes/characters/[id]/+page.ts
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
	const res = await fetch(`/api/characters/${params.id}`);
	if (!res.ok) throw error(res.status, 'Character not found');
	return { character: (await res.json()) as Character };
};
```

- Put each endpoint wrapper in `src/lib/api/` so components never call `fetch` directly.

## Security posture (repeated because it matters)

The client renders **only what the API returns**. Never:

- filter GM-only content client-side (the server already stripped it — if you can see it in the payload, that is an API bug to report, not to patch over),
- hide "forbidden" items with CSS/conditionals (the API must not send them at all),
- gate features on a role field alone for anything security-relevant — client checks are UX sugar, the API is the enforcement.

A 404 from the API on something that "should exist" may be intentional (hidden means invisible) — surface it as not-found, don't retry or special-case it.

### Marking GM-only controls

Actions/sections that only make sense for a GM (create forms, access grants, admin nav) get a visual cue — transparent green background + dashed green border — so it's obvious at a glance without reading copy. Wrap block-level content in `<GmOnly>` (`lib/components/GmOnly.svelte`); it checks `isGM()` and renders nothing for players. For `Card`, pass the `gm` prop instead of wrapping it, so the tint applies to the card itself rather than nesting another bordered box.

This is the same client-side check called out above: styling, not access control. The API still enforces who can actually call the create/grant endpoint — don't skip that because the button is only visible to GMs.

## Testing

- Unit tests: `*.spec.ts` / `*.test.ts` next to the code, run by Vitest in node environment (see `src/lib/vitest-examples/`).
- Keep logic (formatting, game-day math, API response mapping) in plain `.ts` files under `lib/` so it's testable without mounting components.
