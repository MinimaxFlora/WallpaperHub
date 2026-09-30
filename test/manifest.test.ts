import { beforeEach, describe, expect, it } from "vitest";
import { ApiError } from "../src/core/errors";
import { loadManifest, resetManifestCache, validateManifest } from "../src/core/manifest";
import { fakeEnv, sampleImage, sampleManifest } from "./helpers";

describe("validateManifest", () => {
  it("drops malformed entries, dedupes ids and sorts by id", () => {
    const manifest = validateManifest({
      version: 2,
      images: [
        { id: "02", path: "images/02.webp", width: 100, height: 200 },
        { id: "01", path: "images/01.webp", width: 300, height: 200 },
        { id: "01", path: "images/dup.webp", width: 10, height: 10 },
        { id: "", path: "images/bad.webp", width: 10, height: 10 },
        { id: "03", path: "images/03.webp", width: "wide", height: 10 },
      ],
    });

    expect(manifest.version).toBe(2);
    expect(manifest.images.map((image) => image.id)).toEqual(["01", "02"]);
    expect(manifest.images[0].orientation).toBe("landscape");
    expect(manifest.images[1].orientation).toBe("portrait");
    expect(manifest.count).toBe(2);
  });

  it("rejects a manifest without an image list", () => {
    expect(() => validateManifest({})).toThrowError(ApiError);
  });
});

describe("loadManifest", () => {
  beforeEach(() => resetManifestCache());

  it("reads and caches the manifest", async () => {
    const env = fakeEnv(sampleManifest([sampleImage()]));
    const first = await loadManifest(env, 1_000);
    const second = await loadManifest(env, 2_000);
    expect(first.images).toHaveLength(1);
    expect(second).toBe(first);
  });

  it("reloads after the cache expires", async () => {
    const env = fakeEnv(sampleManifest([sampleImage()]));
    const first = await loadManifest(env, 1_000);
    const second = await loadManifest(env, 1_000 + 300_001);
    expect(second).not.toBe(first);
  });

  it("reports a missing manifest as unavailable", async () => {
    await expect(loadManifest(fakeEnv(null))).rejects.toThrowError(ApiError);
  });
});
