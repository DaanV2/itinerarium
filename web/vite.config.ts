import { defineConfig } from 'vitest/config';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';

// The build is a static SPA written straight into the Go module, where
// api/infrastructure/webapp embeds it into the server binary.
const goEmbedDir = '../api/infrastructure/webapp/dist';

export default defineConfig({
	plugins: [
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			adapter: adapter({
				pages: goEmbedDir,
				assets: goEmbedDir,
				// SPA mode: every non-asset path serves the app shell and the
				// client router takes over (ssr is off in src/routes/+layout.ts).
				fallback: 'index.html'
			})
		})
	],
	server: {
		// Dev-only: lets the frontend call the Go API with relative /api paths
		proxy: {
			'/api': 'http://localhost:8080'
		}
	},
	test: {
		expect: { requireAssertions: true },
		projects: [
			{
				extends: './vite.config.ts',
				test: {
					name: 'server',
					environment: 'node',
					include: ['src/**/*.{test,spec}.{js,ts}'],
					exclude: ['src/**/*.svelte.{test,spec}.{js,ts}']
				}
			}
		]
	}
});
