export class APIError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

async function requestError(response: Response) {
  let message = `Request failed (${response.status})`;
  try {
    const body = await response.json();
    if (typeof body?.error === "string") message = body.error;
  } catch {}
  return new APIError(message, response.status);
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
  if (!response.ok) throw await requestError(response);
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
export async function download(
  path: string,
  filename = "udphttp-diagnostics.ndjson",
  mediaType?: RegExp,
) {
  const response = await fetch(path, { signal: AbortSignal.timeout(10000) });
  if (!response.ok) throw await requestError(response);
  const type = response.headers.get("Content-Type")?.split(";")[0].trim() ?? "";
  if (mediaType && !mediaType.test(type))
    throw new APIError("The daemon did not return a playlist file.", response.status);
  const url = URL.createObjectURL(await response.blob());
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
