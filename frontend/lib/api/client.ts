export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message); }
}

export async function request<T>(baseUrl: string, path: string, init: RequestInit = {}, timeout = 8000): Promise<T> {
  const controller = new AbortController();
  const timer = window.setTimeout(() => controller.abort(), timeout);
  try {
    const headers = new Headers(init.headers);
    headers.set("Content-Type", "application/json");
    const response = await fetch(`${baseUrl}${path}`, {
      ...init, signal: controller.signal, headers,
    });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) {
      const message = body?.error?.message ?? body?.error ?? "The service could not complete that request.";
      throw new ApiError(response.status, message);
    }
    return body as T;
  } catch (error) {
    if (error instanceof ApiError) throw error;
    if (error instanceof DOMException && error.name === "AbortError") throw new Error("Request timed out.");
    throw new Error("Service is unreachable right now.");
  } finally {
    window.clearTimeout(timer);
  }
}

export const json = (body: unknown): RequestInit => ({ method: "POST", body: JSON.stringify(body) });
