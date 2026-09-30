import { describe, expect, it } from "vitest";
import { ApiError } from "../src/core/errors";
import { checkHotlink, hostAllowed } from "../src/http/hotlink";
import { fakeEnv } from "./helpers";

describe("hostAllowed", () => {
  it("matches exact hosts", () => {
    expect(hostAllowed("example.com", ["example.com"])).toBe(true);
    expect(hostAllowed("evil.com", ["example.com"])).toBe(false);
  });

  it("matches wildcard subdomains only", () => {
    expect(hostAllowed("a.example.com", ["*.example.com"])).toBe(true);
    expect(hostAllowed("example.com", ["*.example.com"])).toBe(false);
    expect(hostAllowed("notexample.com", ["*.example.com"])).toBe(false);
  });
});

describe("checkHotlink", () => {
  it("allows every origin when the allowlist is empty", () => {
    const request = new Request("https://api.test/v1/images/01/file", {
      headers: { Referer: "https://other.test/page" },
    });
    expect(() => checkHotlink(request, fakeEnv(null))).not.toThrow();
  });

  it("blocks origins outside the allowlist", () => {
    const env = { ...fakeEnv(null), HOTLINK_ALLOWLIST: "example.com" };
    const request = new Request("https://api.test/v1/images/01/file", {
      headers: { Origin: "https://other.test" },
    });
    expect(() => checkHotlink(request, env)).toThrowError(ApiError);
  });

  it("allows listed origins and clients without an origin", () => {
    const env = { ...fakeEnv(null), HOTLINK_ALLOWLIST: "example.com,*.ok.test" };
    const allowed = new Request("https://api.test/v1/images/01/file", {
      headers: { Origin: "https://app.ok.test" },
    });
    expect(() => checkHotlink(allowed, env)).not.toThrow();

    const noOrigin = new Request("https://api.test/v1/images/01/file");
    expect(() => checkHotlink(noOrigin, env)).not.toThrow();
  });
});
