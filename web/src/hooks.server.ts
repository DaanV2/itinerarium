import type { Handle } from '@sveltejs/kit';

// Dev uses vite's server.proxy (see vite.config.ts). The production build
// (adapter-node) has no such proxy, so requests to /api on the web origin
// need to be forwarded here to the Go API's compose-network address.
const API_URL = process.env.API_URL ?? 'http://localhost:8080';

export const handle: Handle = async ({ event, resolve }) => {
	if (!event.url.pathname.startsWith('/api')) {
		return resolve(event);
	}

	const target = new URL(event.url.pathname + event.url.search, API_URL);
	const hasBody = event.request.method !== 'GET' && event.request.method !== 'HEAD';

	return fetch(target, {
		method: event.request.method,
		headers: event.request.headers,
		body: hasBody ? await event.request.arrayBuffer() : undefined,
		redirect: 'manual'
	});
};
