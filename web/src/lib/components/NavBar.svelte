<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { clearAccessToken, isGM } from '$lib/auth-token';

	let isgm = $derived(isGM());

	function isActive(path: string): boolean {
		return path === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(path);
	}

	async function handleLogout() {
		clearAccessToken();
		await goto(resolve('/login'));
	}
</script>

<nav class="sidebar" aria-label="Main navigation">
	<ul>
		<li>
			<a href={resolve('/')} aria-current={isActive('/') ? 'page' : undefined}>Home</a>
		</li>
		<li>
			<a href={resolve('/characters')} aria-current={isActive('/characters') ? 'page' : undefined}
				>Characters</a
			>
		</li>
		<li>
			<a href={resolve('/groups')} aria-current={isActive('/groups') ? 'page' : undefined}>Groups</a
			>
		</li>
		<li>
			<a href={resolve('/locations')} aria-current={isActive('/locations') ? 'page' : undefined}
				>Locations</a
			>
		</li>
		<li>
			<a
				href={resolve('/repositories')}
				aria-current={isActive('/repositories') ? 'page' : undefined}>Repositories</a
			>
		</li>
		<li>
			<a href={resolve('/search')} aria-current={isActive('/search') ? 'page' : undefined}>Search</a
			>
		</li>
		{#if isgm}
			<li>
				<a
					class="admin-link"
					href={resolve('/admin')}
					aria-current={isActive('/admin') ? 'page' : undefined}>Admin</a
				>
			</li>
		{/if}
	</ul>

	<button type="button" class="logout" onclick={handleLogout}>Log out</button>
</nav>

<style>
	.sidebar {
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		width: 200px;
		flex-shrink: 0;
		height: 100vh;
		position: sticky;
		top: 0;
		border-right: 1px solid #ccc;
		padding: 1rem 0.5rem;
		box-sizing: border-box;
	}

	ul {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	a {
		display: block;
		padding: 0.5rem 0.75rem;
		border-radius: 5px;
		color: inherit;
		text-decoration: none;
	}

	a:hover {
		background-color: rgba(128, 128, 128, 0.1);
	}

	a[aria-current='page'] {
		background-color: rgba(128, 128, 128, 0.15);
		font-weight: 600;
	}

	a.admin-link {
		color: rgba(34, 197, 94, 0.9);
	}

	.logout {
		background: none;
		border: 1px solid #ccc;
		border-radius: 5px;
		padding: 0.5rem 0.75rem;
		cursor: pointer;
		color: inherit;
		font: inherit;
	}

	.logout:hover {
		border-color: #888;
	}
</style>
