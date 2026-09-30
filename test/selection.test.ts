import { describe, expect, it } from "vitest";
import { ApiError } from "../src/core/errors";
import { formatDateInZone, parseMode, resolveDate, safeTimezone, selectIndex } from "../src/core/selection";

describe("parseMode", () => {
  it("accepts known modes", () => {
    expect(parseMode("seed")).toBe("seed");
    expect(parseMode("daily")).toBe("daily");
  });

  it("falls back to random for unknown or missing modes", () => {
    expect(parseMode(null)).toBe("random");
    expect(parseMode("bogus")).toBe("random");
  });
});

describe("selectIndex", () => {
  it("is deterministic for the seed mode", () => {
    const first = selectIndex({ mode: "seed", seed: "abc", timezone: "UTC" }, 8);
    const second = selectIndex({ mode: "seed", seed: "abc", timezone: "UTC" }, 8);
    expect(first).toBe(second);
  });

  it("changes with the seed", () => {
    const indexes = new Set(
      ["a", "b", "c", "d", "e", "f", "g", "h"].map((seed) =>
        selectIndex({ mode: "seed", seed, timezone: "UTC" }, 1000),
      ),
    );
    expect(indexes.size).toBeGreaterThan(1);
  });

  it("is stable per day and varies across days", () => {
    const monday = selectIndex({ mode: "daily", date: "2026-09-28", timezone: "UTC" }, 64);
    expect(selectIndex({ mode: "daily", date: "2026-09-28", timezone: "UTC" }, 64)).toBe(monday);
    const tuesday = selectIndex({ mode: "daily", date: "2026-09-29", timezone: "UTC" }, 64);
    expect(tuesday).not.toBe(monday);
  });

  it("stays within bounds for every mode", () => {
    for (const mode of ["random", "seed", "daily", "session"] as const) {
      for (let i = 0; i < 200; i += 1) {
        const index = selectIndex({ mode, seed: "s", date: "2026-09-30", timezone: "UTC", sessionId: "sess" }, 7);
        expect(index).toBeGreaterThanOrEqual(0);
        expect(index).toBeLessThan(7);
      }
    }
  });

  it("rejects an empty candidate set", () => {
    expect(() => selectIndex({ mode: "seed", seed: "x", timezone: "UTC" }, 0)).toThrowError(ApiError);
  });
});

describe("dates", () => {
  it("formats the current date in the requested zone", () => {
    const now = new Date("2026-09-30T23:30:00Z");
    expect(formatDateInZone(now, "UTC")).toBe("2026-09-30");
    expect(formatDateInZone(now, "Asia/Shanghai")).toBe("2026-10-01");
  });

  it("validates explicit dates", () => {
    expect(resolveDate("2026-01-02", "UTC")).toBe("2026-01-02");
    expect(() => resolveDate("02/01/2026", "UTC")).toThrowError(ApiError);
  });

  it("falls back to UTC for an invalid timezone", () => {
    expect(safeTimezone("Not/AZone")).toBe("UTC");
    expect(safeTimezone("Asia/Shanghai")).toBe("Asia/Shanghai");
  });
});
