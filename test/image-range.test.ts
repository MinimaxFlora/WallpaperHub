import { beforeEach, describe, expect, it } from "vitest";
import { ApiError } from "../src/core/errors";
import { resetManifestCache } from "../src/core/manifest";
import { handleImageFile } from "../src/routes/image";
import { fakeContext, fakeEnv, sampleImage, sampleManifest } from "./helpers";

describe("parseRange", () => {
  it("parses a bounded range", async () => {
    const { parseRange } = await import("../src/routes/image");
    expect(parseRange("bytes=10-19", 100)).toEqual({ offset: 10, length: 10 });
  });

  it("parses an open-ended range", async () => {
    const { parseRange } = await import("../src/routes/image");
    expect(parseRange("bytes=90-", 100)).toEqual({ offset: 90, length: 10 });
  });

  it("parses a suffix range", async () => {
    const { parseRange } = await import("../src/routes/image");
    expect(parseRange("bytes=-20", 100)).toEqual({ offset: 80, length: 20 });
  });

  it("rejects ranges outside the image", async () => {
    const { parseRange } = await import("../src/routes/image");
    expect(() => parseRange("bytes=200-300", 100)).toThrowError(ApiError);
  });
});

describe("handleImageFile", () => {
  const bytes = new Uint8Array(100).map((_, index) => index);

  beforeEach(() => resetManifestCache());

  it("serves the full image with a content type and etag", async () => {
    const env = fakeEnv(sampleManifest([sampleImage()]), { "images/01.webp": bytes });
    const response = await handleImageFile(
      "01",
      new Request("https://api.test/v1/images/01/file"),
      env,
      fakeContext,
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("image/webp");
    expect(response.headers.get("ETag")).toBe('"sha256-abc"');
    expect((await response.arrayBuffer()).byteLength).toBe(100);
  });

  it("returns 304 for a matching If-None-Match", async () => {
    const env = fakeEnv(sampleManifest([sampleImage()]), { "images/01.webp": bytes });
    const response = await handleImageFile(
      "01",
      new Request("https://api.test/v1/images/01/file", { headers: { "If-None-Match": '"sha256-abc"' } }),
      env,
      fakeContext,
    );
    expect(response.status).toBe(304);
  });

  it("serves a byte range", async () => {
    const env = fakeEnv(sampleManifest([sampleImage()]), { "images/01.webp": bytes });
    const response = await handleImageFile(
      "01",
      new Request("https://api.test/v1/images/01/file", { headers: { Range: "bytes=10-19" } }),
      env,
      fakeContext,
    );
    expect(response.status).toBe(206);
    expect(response.headers.get("Content-Range")).toBe("bytes 10-19/100");
  });

  it("returns 404 for an unknown id", async () => {
    const env = fakeEnv(sampleManifest([sampleImage()]), { "images/01.webp": bytes });
    await expect(
      handleImageFile("99", new Request("https://api.test/v1/images/99/file"), env, fakeContext),
    ).rejects.toThrowError(ApiError);
  });
});
