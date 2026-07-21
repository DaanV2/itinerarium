// The one place that talks to the Go API. Every endpoint wrapper under
// `lib/api/` funnels through `apiFetch` / `apiSend` so the bearer-header,
// JSON-body, and `{ error, code }` error-parsing rules live once instead of
// being copy-pasted per file.

/** An unsuccessful API response, carrying the HTTP `status` and the optional
 * machine-readable `code` the server attaches to some errors (e.g.
 * `path_collision`, `concurrent_edit`). Callers that need to branch on a
 * specific failure catch a subclass such as {@link DocumentConflictError};
 * everyone else can treat it as a plain `Error` with a human message. */
export class ApiError extends Error {
	status: number;
	code?: string;

	constructor(message: string, status: number, code?: string) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.code = code;
	}
}

/** Shape of the JSON error body the API returns on failure. */
interface ErrorBody {
	error?: string;
	code?: string;
}

/** Options for a single API call. `token` is omitted for the unauthenticated
 * endpoints (setup, login); `body`, when given, is JSON-serialized and sent
 * with a JSON content-type. `errorContext` is the human prefix used when the
 * error body carries no `error` message — the fallback reads
 * `"<context>: <status>"`. */
export interface ApiRequestOptions {
	method?: string;
	token?: string;
	body?: unknown;
	errorContext?: string;
	fetchFn?: typeof fetch;
}

async function parseErrorBody(res: Response): Promise<ErrorBody | null> {
	const body: unknown = await res.json().catch(() => null);
	if (!body || typeof body !== 'object') return null;

	const obj = body as { error?: unknown; code?: unknown };
	return {
		error: typeof obj.error === 'string' ? obj.error : undefined,
		code: typeof obj.code === 'string' ? obj.code : undefined
	};
}

/** Builds the `fetch` init object, adding only the keys a given call needs so
 * an authenticated GET stays `{ headers }` and an unauthenticated GET needs no
 * init at all. Returns `undefined` when nothing needs to be set. */
function buildInit(
	method: string,
	token: string | undefined,
	body: unknown
): RequestInit | undefined {
	const headers: Record<string, string> = {};
	if (token) headers.Authorization = `Bearer ${token}`;

	const hasBody = body !== undefined;
	if (hasBody) headers['Content-Type'] = 'application/json';

	const init: RequestInit = {};
	if (method !== 'GET') init.method = method;
	if (Object.keys(headers).length > 0) init.headers = headers;
	if (hasBody) init.body = JSON.stringify(body);

	return Object.keys(init).length > 0 ? init : undefined;
}

/** Runs the request and, on a non-2xx response, throws an {@link ApiError}
 * built from the `{ error, code }` body (falling back to
 * `"<errorContext>: <status>"`). Returns the raw `Response` on success. */
async function request(path: string, options: ApiRequestOptions): Promise<Response> {
	const { method = 'GET', token, body, errorContext, fetchFn = fetch } = options;
	const init = buildInit(method, token, body);
	const res = init ? await fetchFn(path, init) : await fetchFn(path);

	if (!res.ok) {
		const parsed = await parseErrorBody(res);
		const message = parsed?.error ?? `${errorContext ?? 'request failed'}: ${res.status}`;
		throw new ApiError(message, res.status, parsed?.code);
	}

	return res;
}

/** Performs an API call and returns the parsed JSON body as `T`. */
export async function apiFetch<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
	const res = await request(path, options);
	return (await res.json()) as T;
}

/** Performs an API call whose success response carries no body (e.g. a
 * `204 No Content` delete). Errors are raised exactly as {@link apiFetch}. */
export async function apiSend(path: string, options: ApiRequestOptions = {}): Promise<void> {
	await request(path, options);
}
