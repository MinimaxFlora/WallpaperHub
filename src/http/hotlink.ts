import { ApiError } from "../core/errors";
import type { Env } from "../types";

export function checkHotlink(request: Request, env: Env): void {
  const patterns = (env.HOTLINK_ALLOWLIST ?? "")
    .split(",")
    .map((pattern) => pattern.trim().toLowerCase())
    .filter((pattern) => pattern.length > 0);

  // An empty allowlist keeps the API usable by any client.
  if (patterns.length === 0) return;

  const host = requestHost(request);
  // Non-browser clients send no origin, so there is nothing to protect against.
  if (host === null) return;
  if (hostAllowed(host, patterns)) return;

  throw new ApiError(403, "forbidden_origin", "this origin is not allowed to load wallpaper files");
}

function requestHost(request: Request): string | null {
  const origin = request.headers.get("Origin");
  if (origin) {
    const host = hostnameOf(origin);
    if (host) return host;
  }
  const referer = request.headers.get("Referer");
  if (referer) return hostnameOf(referer);
  return null;
}

function hostnameOf(value: string): string | null {
  try {
    return new URL(value).hostname.toLowerCase();
  } catch {
    return null;
  }
}

export function hostAllowed(host: string, patterns: string[]): boolean {
  return patterns.some((pattern) => {
    if (pattern.startsWith("*.")) {
      const suffix = pattern.slice(1);
      return host.endsWith(suffix) && host.length > suffix.length;
    }
    return host === pattern;
  });
}
