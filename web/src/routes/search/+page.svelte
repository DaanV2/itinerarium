<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { searchDocuments } from '$lib/api/search';
	import { getAccessToken } from '$lib/auth-token';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import type { SearchResult } from '$lib/types';

	const initialQuery = page.url.searchParams.get('q') ?? '';

	let query = $state(initialQuery);
	let results = $state<SearchResult[]>([]);
	let searched = $state(false);
	let loading = $state(false);
	let error = $state('');

	const fieldLabels: Record<string, string> = {
		title: 'title',
		path: 'path',
		tags: 'tags',
		content: 'content'
	};

	async function runSearch(event?: SubmitEvent) {
		event?.preventDefault();
		if (!query.trim()) {
			return;
		}

		loading = true;
		try {
			results = await searchDocuments(query, getAccessToken());
			searched = true;
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Search failed.';
		} finally {
			loading = false;
		}
	}

	// Support arriving with ?q= already set (e.g. a shared link).
	if (initialQuery.trim()) {
		void runSearch();
	}
</script>

<main class="main-page">
	<h1>Search</h1>

	<form class="search-bar" onsubmit={runSearch}>
		<input
			type="search"
			placeholder="Search titles, file names, tags, and content…"
			bind:value={query}
			aria-label="Search query"
		/>
		<button type="submit" disabled={loading || !query.trim()}>Search</button>
	</form>

	<ErrorAlert message={error} />

	{#if loading}
		<p>Searching…</p>
	{:else if searched && results.length === 0}
		<p>No documents found.</p>
	{:else if results.length > 0}
		<p class="result-count">
			{results.length}
			{results.length === 1 ? 'document' : 'documents'} found
		</p>
		<ul class="results">
			{#each results as result (result.id)}
				<li>
					<a class="result" href={resolve('/documents/[id]', { id: result.id })}>
						<div class="result-head">
							<span class="result-title">{result.title}</span>
							<span class="result-path">{result.path}</span>
						</div>
						<div class="result-meta">
							{#each result.matched_in as field (field)}
								<span class="badge">{fieldLabels[field] ?? field}</span>
							{/each}
							{#each result.tags as tag (tag)}
								<span class="tag">#{tag}</span>
							{/each}
						</div>
						{#if result.snippet}
							<p class="snippet">{result.snippet}</p>
						{/if}
					</a>
				</li>
			{/each}
		</ul>
	{/if}
</main>

<style>
	.search-bar {
		display: flex;
		gap: 0.5rem;
		margin-bottom: 1rem;
	}

	.search-bar input {
		flex: 1;
		max-width: 32rem;
		padding: 0.5rem 0.75rem;
		border: 1px solid #ccc;
		border-radius: 5px;
		font: inherit;
	}

	.search-bar button {
		padding: 0.5rem 1rem;
		border: 1px solid #ccc;
		border-radius: 5px;
		background: none;
		font: inherit;
		cursor: pointer;
	}

	.search-bar button:disabled {
		opacity: 0.5;
		cursor: default;
	}

	.result-count {
		color: #666;
	}

	.results {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.result {
		display: block;
		border: 1px solid #ccc;
		border-radius: 5px;
		padding: 0.75rem 1rem;
		color: inherit;
		text-decoration: none;
	}

	.result:hover {
		border-color: #888;
	}

	.result-head {
		display: flex;
		align-items: baseline;
		gap: 0.75rem;
		flex-wrap: wrap;
	}

	.result-title {
		font-weight: 600;
	}

	.result-path {
		color: #666;
		font-size: 0.85rem;
		font-family: monospace;
	}

	.result-meta {
		display: flex;
		gap: 0.4rem;
		margin-top: 0.35rem;
		flex-wrap: wrap;
	}

	.badge {
		font-size: 0.75rem;
		border: 1px solid #ccc;
		border-radius: 999px;
		padding: 0.05rem 0.5rem;
		color: #555;
	}

	.tag {
		font-size: 0.75rem;
		color: #2563eb;
	}

	.snippet {
		margin: 0.5rem 0 0;
		color: #444;
		font-size: 0.9rem;
	}
</style>
