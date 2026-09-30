import { ApiError } from "./errors";
import { fnv1a } from "./hash";

export type SelectionMode = "random" | "seed" | "daily" | "session";

const MODES: readonly SelectionMode[] = ["random", "seed", "daily", "session"];

export interface SelectionInput {
  mode: SelectionMode;
  seed?: string;
  date?: string;
  timezone: string;
  sessionId?: string;
}

export function parseMode(raw: string | null): SelectionMode {
  return raw !== null && (MODES as readonly string[]).includes(raw) ? (raw as SelectionMode) : "random";
}

export function safeTimezone(raw: string | undefined): string {
  const zone = raw && raw.length > 0 ? raw : "UTC";
  try {
    new Intl.DateTimeFormat("en-CA", { timeZone: zone }).format(new Date());
    return zone;
  } catch {
    return "UTC";
  }
}

export function formatDateInZone(now: Date, timezone: string): string {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(now);
}

export function resolveDate(raw: string | null, timezone: string, now: Date = new Date()): string {
  if (raw !== null && raw !== "") {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(raw)) {
      throw new ApiError(400, "invalid_date", "date must be formatted as YYYY-MM-DD");
    }
    return raw;
  }
  return formatDateInZone(now, timezone);
}

export function selectIndex(input: SelectionInput, count: number): number {
  if (count <= 0) {
    throw new ApiError(404, "no_match", "no image matches the given filters");
  }
  switch (input.mode) {
    case "random":
      return randomIndex(count);
    case "seed":
      return fnv1a(`seed\u0000${input.seed ?? ""}`) % count;
    case "daily":
      return fnv1a(`daily\u0000${input.date ?? ""}\u0000${input.timezone}`) % count;
    case "session":
      return fnv1a(`session\u0000${input.sessionId ?? ""}`) % count;
  }
}

function randomIndex(count: number): number {
  const buffer = new Uint32Array(1);
  crypto.getRandomValues(buffer);
  return buffer[0] % count;
}
