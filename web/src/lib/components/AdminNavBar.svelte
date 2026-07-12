<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { clearAccessToken } from '$lib/auth-token';

	function isActive(path: string): boolean {
		return path === '/admin' ? page.url.pathname === '/admin' : page.url.pathname.startsWith(path);
	}

	async function handleLogout() {
		clearAccessToken();
		await goto(resolve('/login'));
	}
</script>

<nav class="sidebar" aria-label="Admin navigation">
	<p class="section-label">Admin</p>

	<ul>
		<li>
			<a href={resolve('/admin')} aria-current={isActive('/admin') ? 'page' : undefined}>Overview</a
			>
		</li>
		<li>
			<a href={resolve('/admin/users')} aria-current={isActive('/admin/users') ? 'page' : undefined}
				>Users</a
			>
		</li>
		<li>
			<a
				href={resolve('/admin/characters')}
				aria-current={isActive('/admin/characters') ? 'page' : undefined}>Characters</a
			>
		</li>
	</ul>

	<div class="footer">
		<a class="back-link" href={resolve('/')}>← Back to app</a>
		<button type="button" class="logout" onclick={handleLogout}>Log out</button>
	</div>
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
		border-right: 1px dashed rgba(34, 197, 94, 0.6);
		background-color: rgba(34, 197, 94, 0.06);
		padding: 1rem 0.5rem;
		box-sizing: border-box;
	}

	.section-label {
		margin: 0 0 0.75rem 0.75rem;
		font-size: 0.75rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: rgba(34, 197, 94, 0.9);
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
		background-color: rgba(34, 197, 94, 0.12);
	}

	a[aria-current='page'] {
		background-color: rgba(34, 197, 94, 0.18);
		font-weight: 600;
	}

	.footer {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.back-link {
		font-size: 0.9rem;
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
