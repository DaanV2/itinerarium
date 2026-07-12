import type { Role } from '$lib/types';

const TOKEN_KEY = 'itinerarium_access_token';
const ROLE_KEY = 'itinerarium_role';

/**
 * Reads the stored access token, or an empty string when none is set (or when
 * running server-side where `localStorage` is unavailable). API wrappers treat
 * an empty token as "unauthenticated".
 */
export function getAccessToken(): string {
	if (typeof localStorage === 'undefined') return '';
	return localStorage.getItem(TOKEN_KEY) ?? '';
}

/** Persists the access token issued after login/setup. */
export function setAccessToken(token: string): void {
	if (typeof localStorage === 'undefined') return;
	localStorage.setItem(TOKEN_KEY, token);
}

/** Clears the stored access token (e.g. on logout). */
export function clearAccessToken(): void {
	if (typeof localStorage === 'undefined') return;
	localStorage.removeItem(TOKEN_KEY);
	localStorage.removeItem(ROLE_KEY);
}

/** Persists the account role from login/setup. UX sugar only: it decides what
 * the UI offers, never what the API allows — the server enforces every rule. */
export function setUserRole(role: Role): void {
	if (typeof localStorage === 'undefined') return;
	localStorage.setItem(ROLE_KEY, role);
}

/** Whether the logged-in account is a GM, per the stored role. */
export function isGM(): boolean {
	if (typeof localStorage === 'undefined') return false;
	return localStorage.getItem(ROLE_KEY) === 'gm';
}
