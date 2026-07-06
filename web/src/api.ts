export class ApiError extends Error {
	status: number;
	constructor(status: number, message: string) {
		super(message);
		this.name = "ApiError";
		this.status = status;
	}
}

const apiBase = import.meta.env.VITE_API_URL ?? "";

function apiUrl(path: string): string {
	return `${apiBase}${path}`;
}

export async function fetchJson<T>(url: string): Promise<T> {
	const r = await fetch(apiUrl(url), { credentials: "include" });
	if (!r.ok) {
		throw new ApiError(r.status, `${r.status} ${r.statusText}`);
	}
	return r.json();
}

async function mutateJson<T>(method: string, url: string, body: unknown): Promise<T> {
	const r = await fetch(apiUrl(url), {
		method,
		credentials: "include",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(body),
	});
	if (!r.ok) {
		throw new ApiError(r.status, `${r.status} ${r.statusText}`);
	}
	if (r.status === 204) {
		return undefined as T;
	}
	return r.json();
}

export function postJson<T>(url: string, body: unknown): Promise<T> {
	return mutateJson("POST", url, body);
}

export function patchJson<T>(url: string, body: unknown): Promise<T> {
	return mutateJson("PATCH", url, body);
}

export async function deleteJson(url: string): Promise<void> {
	const r = await fetch(apiUrl(url), { method: "DELETE", credentials: "include" });
	if (!r.ok) {
		throw new ApiError(r.status, `${r.status} ${r.statusText}`);
	}
}

export function bookmarksKey(userId: string): ["bookmarks", string] {
	return ["bookmarks", userId];
}

export async function trackPageview(path: string, visitorId: string): Promise<void> {
	try {
		await fetch(apiUrl("/collect"), {
			method: "POST",
			credentials: "include",
			keepalive: true,
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ path, referrer: document.referrer, visitor_id: visitorId }),
		});
	} catch {
		// best-effort analytics — never surface to the user
	}
}
