import { describe, it, expect, vi } from 'vitest';
import { apiFetch, apiSend, ApiError } from './client';

function jsonResponse(body: unknown, ok = true, status = ok ? 200 : 400): Response {
	return {
		ok,
		status,
		json: () => Promise.resolve(body)
	} as Response;
}

describe('apiFetch', () => {
	it('sends an authenticated GET as bare headers with no method key', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ ok: true }));

		const result = await apiFetch('/api/thing', { token: 'token-123', fetchFn });

		expect(result).toEqual({ ok: true });
		expect(fetchFn).toHaveBeenCalledWith('/api/thing', {
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('calls fetch with no init at all for an unauthenticated GET', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ needs_setup: true }));

		await apiFetch('/api/setup', { fetchFn });

		expect(fetchFn).toHaveBeenCalledWith('/api/setup');
	});

	it('serializes a JSON body and sets the content-type on a write', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ id: '1' }, true, 201));

		await apiFetch('/api/things', {
			method: 'POST',
			token: 'token-123',
			body: { name: 'x' },
			fetchFn
		});

		expect(fetchFn).toHaveBeenCalledWith('/api/things', {
			method: 'POST',
			headers: { Authorization: 'Bearer token-123', 'Content-Type': 'application/json' },
			body: JSON.stringify({ name: 'x' })
		});
	});

	it('omits the content-type for a body-less write (e.g. POST reset)', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ ok: true }));

		await apiFetch('/api/reset', { method: 'POST', token: 'token-123', fetchFn });

		expect(fetchFn).toHaveBeenCalledWith('/api/reset', {
			method: 'POST',
			headers: { Authorization: 'Bearer token-123' }
		});
	});

	it('throws an ApiError carrying the status, message, and code from the body', async () => {
		const fetchFn = vi
			.fn()
			.mockResolvedValue(jsonResponse({ error: 'nope', code: 'concurrent_edit' }, false, 409));

		const err = await apiFetch('/api/thing', { token: 't', fetchFn }).catch((e: unknown) => e);

		expect(err).toBeInstanceOf(ApiError);
		const apiErr = err as ApiError;
		expect(apiErr.message).toBe('nope');
		expect(apiErr.status).toBe(409);
		expect(apiErr.code).toBe('concurrent_edit');
	});

	it('falls back to "<context>: <status>" when the body has no error message', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({}, false, 500));

		await expect(
			apiFetch('/api/thing', { errorContext: 'failed to load thing', fetchFn })
		).rejects.toThrow('failed to load thing: 500');
	});

	it('falls back even when the error body is not JSON', async () => {
		const fetchFn = vi.fn().mockResolvedValue({
			ok: false,
			status: 502,
			json: () => Promise.reject(new Error('not json'))
		} as unknown as Response);

		await expect(apiFetch('/api/thing', { errorContext: 'boom', fetchFn })).rejects.toThrow(
			'boom: 502'
		);
	});
});

describe('apiSend', () => {
	it('resolves without reading the body of a 204 response', async () => {
		const json = vi.fn(() => Promise.reject(new Error('no body')));
		const fetchFn = vi
			.fn()
			.mockResolvedValue({ ok: true, status: 204, json } as unknown as Response);

		await expect(
			apiSend('/api/thing/1', { method: 'DELETE', token: 't', fetchFn })
		).resolves.toBeUndefined();
		expect(json).not.toHaveBeenCalled();
		expect(fetchFn).toHaveBeenCalledWith('/api/thing/1', {
			method: 'DELETE',
			headers: { Authorization: 'Bearer t' }
		});
	});

	it('still raises an ApiError on failure', async () => {
		const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: 'gone' }, false, 404));

		await expect(
			apiSend('/api/thing/1', { method: 'DELETE', token: 't', fetchFn })
		).rejects.toThrow('gone');
	});
});
