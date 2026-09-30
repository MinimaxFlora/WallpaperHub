import { ApiError } from "../core/errors";
import type { Env, RateLimitBinding } from "../types";

const RETRY_AFTER_SECONDS = 60;
const MEMORY_LIMIT = 120;
const MEMORY_WINDOW_MS = 60_000;

interface Counter {
  count: number;
  resetAt: number;
}

// Fallback used where the rate limit binding is unavailable, such as local
// workerd runs. It is per-isolate and therefore approximate.
const memory = new Map<string, Counter>();

export function resetMemoryCounters(): void {
  memory.clear();
}

export function clientKey(request: Request): string {
  const forwarded = request.headers.get("CF-Connecting-IP") ?? request.headers.get("X-Forwarded-For");
  return forwarded?.split(",")[0]?.trim() || "anonymous";
}

export async function enforceRateLimit(
  env: Env,
  limiter: RateLimitBinding | undefined,
  key: string,
  now: number = Date.now(),
): Promise<void> {
  if (!isEnabled(env)) return;

  if (limiter) {
    let success = true;
    try {
      ({ success } = await limiter.limit({ key }));
    } catch {
      // Fail open: availability matters more than strict accounting here.
      return;
    }
    if (!success) throw tooManyRequests();
    return;
  }

  if (memoryLimitExceeded(key, now)) throw tooManyRequests();
}

function isEnabled(env: Env): boolean {
  return (env.RATE_LIMIT_ENABLED ?? "true").toLowerCase() !== "false";
}

function memoryLimitExceeded(key: string, now: number): boolean {
  const entry = memory.get(key);
  if (!entry || entry.resetAt <= now) {
    memory.set(key, { count: 1, resetAt: now + MEMORY_WINDOW_MS });
    return false;
  }
  entry.count += 1;
  return entry.count > MEMORY_LIMIT;
}

function tooManyRequests(): ApiError {
  return new ApiError(429, "rate_limited", "too many requests", RETRY_AFTER_SECONDS);
}
