export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly retryAfter?: number;

  constructor(status: number, code: string, message: string, retryAfter?: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.retryAfter = retryAfter;
  }
}

export function jsonResponse(body: unknown, status = 200, extra?: HeadersInit): Response {
  const headers = new Headers(extra);
  headers.set("Content-Type", "application/json; charset=utf-8");
  return new Response(JSON.stringify(body), { status, headers });
}

export function errorResponse(error: unknown): Response {
  const apiError =
    error instanceof ApiError ? error : new ApiError(500, "internal", "internal server error");

  const headers = new Headers();
  if (apiError.retryAfter !== undefined) headers.set("Retry-After", String(apiError.retryAfter));

  return jsonResponse(
    { error: { code: apiError.code, message: apiError.message } },
    apiError.status,
    headers,
  );
}
