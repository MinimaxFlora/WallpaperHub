import { beforeEach, describe, expect, it } from "vitest";
import worker from "../src/index";
import { resetManifestCache } from "../src/core/manifest";
import { fakeContext, fakeEnv, sampleImage, sampleManifest } from "./helpers";

const images = [
  sampleImage({ id: "01", tags: ["anime"], category: "anime" }),
  sampleImage({ id: "02", path: "images/02.webp", tags: ["nature"], category: "nature", hash: "sha256-def" }),
];

function call(path: string, init?: RequestInit) {
  const env = fakeEnv(sampleManifest(images), { "images/01.webp": new Uint8Array([1, 2, 3]) });
  return worker.fetch(new Request(`https://api.test${path}`, init), env, fakeContext);
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function asJson(response: Response): Promise<any> {
  return response.json();
}

describe("worker routes", () => {
  beforeEach(() => resetManifestCache());

  it("returns the api index", async () => {
    const response = await call("/");
    const body = await asJson(response);
    expect(response.status).toBe(200);
    expect(body.endpoints.random).toBe("https://api.test/v1/random");
  });

  it("picks a deterministic image for a seed", async () => {
    const first = await asJson(await call("/v1/random?mode=seed&seed=hello"));
    const second = await asJson(await call("/v1/random?mode=seed&seed=hello"));
    expect(first.id).toBe(second.id);
    expect(first.mode).toBe("seed");
  });

  it("redirects to the image file", async () => {
    const response = await call("/v1/random?redirect=1", { redirect: "manual" });
    expect(response.status).toBe(302);
    expect(response.headers.get("Location")).toMatch(/\/v1\/images\/0[12]\/file$/);
  });

  it("returns 404 when no image matches", async () => {
    const response = await call("/v1/random?tags=missing");
    expect(response.status).toBe(404);
  });

  it("lists images with pagination", async () => {
    const body = await asJson(await call("/v1/images?page=1&page_size=1"));
    expect(body.total).toBe(2);
    expect(body.items).toHaveLength(1);
    expect(body.page).toBe(1);
  });

  it("filters the list by category", async () => {
    const body = await asJson(await call("/v1/images?category=nature"));
    expect(body.total).toBe(1);
    expect(body.items[0].id).toBe("02");
  });

  it("serves image metadata", async () => {
    const body = await asJson(await call("/v1/images/01"));
    expect(body.id).toBe("01");
    expect(body.url).toBe("https://api.test/v1/images/01/file");
  });

  it("reports taxonomy counts", async () => {
    const tags = await asJson(await call("/v1/tags"));
    expect(tags.items).toEqual([
      { name: "anime", count: 1 },
      { name: "nature", count: 1 },
    ]);
    const categories = await asJson(await call("/v1/categories"));
    expect(categories.items).toHaveLength(2);
  });

  it("exposes a health endpoint", async () => {
    const body = await asJson(await call("/healthz"));
    expect(body.status).toBe("ok");
    expect(body.images).toBe(2);
  });

  it("rejects unsupported methods", async () => {
    const response = await call("/v1/random", { method: "POST" });
    expect(response.status).toBe(405);
  });

  it("returns 404 for unknown routes", async () => {
    const response = await call("/nope");
    expect(response.status).toBe(404);
  });
});
