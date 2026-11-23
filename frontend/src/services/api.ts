export class APIError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}
export async function api<T>(
  path: string,
  options: RequestInit = {},
  signal?: AbortSignal,
): Promise<T> {
  const response = await fetch(path, {
    ...options,
    signal: signal ?? options.signal,
    headers: { Accept: "application/json", ...options.headers },
  });
  if (!response.ok) {
    let message = `Request failed (${response.status})`;
    try {
      const body = await response.json();
      message = body.error ?? message;
    } catch {}
    throw new APIError(message, response.status);
  }
  if (response.status === 204) return undefined as T;
  if (!response.headers.get("Content-Type")?.includes("application/json"))
    throw new APIError("The service did not return JSON", response.status);
  return response.json() as Promise<T>;
}
export async function mutate<T>(
  path: string,
  method: string,
  body?: unknown,
): Promise<T> {
  const signal = AbortSignal.timeout(15000);
  const { token } = await api<{ token: string }>("/api/v1/csrf", {}, signal);
  return api<T>(
    path,
    {
      method,
      headers: { "Content-Type": "application/json", "X-CSRF-Token": token },
      body: body === undefined ? undefined : JSON.stringify(body),
    },
    signal,
  );
}
export async function download(path: string, filename = "udphttp-diagnostics.ndjson") {
  const response = await fetch(path, { signal: AbortSignal.timeout(10000) });
  if (!response.ok)
    throw new APIError("Diagnostic export failed", response.status);
  const url = URL.createObjectURL(await response.blob());
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
