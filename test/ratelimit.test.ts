import { beforeEach, describe, expect, it } from "vitest";
import { ApiError } from "../src/core/errors";
import { clientKey, enforceRateLimit, resetMemoryCounters } from "../src/http/ratelimit";
import { fakeEnv } from "./helpers";

describe("clientKey", () => {
  it("prefers the Cloudflare connecting IP", () => {
    const request = new Request("https://api.test/", {
      headers: { "CF-Connecting-IP": "1.2.3.4", "X-Forwarded-For": "9.9.9.9" },
    });
    expect(clientKey(request)).toBe("1.2.3.4");
  });
});

describe("enforceRateLimit", () => {
  beforeEach(() => resetMemoryCounters());

  it("does nothing when disabled", async () => {
    const env = { ...fakeEnv(null), RATE_LIMIT_ENABLED: "false" };
    await expect(enforceRateLimit(env, undefined, "k")).resolves.toBeUndefined();
  });

  it("uses the binding when present", async () => {
    const env = { ...fakeEnv(null), RATE_LIMIT_ENABLED: "true" };
    let called = 0;
    const limiter = {
      limit: async () => {
        called += 1;
        return { success: false };
      },
    };
    await expect(enforceRateLimit(env, limiter, "k")).rejects.toThrowError(ApiError);
    expect(called).toBe(1);
  });

  it("falls back to an in-memory counter", async () => {
    const env = { ...fakeEnv(null), RATE_LIMIT_ENABLED: "true" };
    const key = "api:memory";
    for (let i = 0; i < 120; i += 1) {
      await enforceRateLimit(env, undefined, key, 1_000);
    }
    await expect(enforceRateLimit(env, undefined, key, 1_000)).rejects.toThrowError(ApiError);
  });

  it("resets the in-memory window after it elapses", async () => {
    const env = { ...fakeEnv(null), RATE_LIMIT_ENABLED: "true" };
    const key = "api:window";
    for (let i = 0; i < 120; i += 1) {
      await enforceRateLimit(env, undefined, key, 1_000);
    }
    await expect(enforceRateLimit(env, undefined, key, 62_000)).resolves.toBeUndefined();
  });
});
