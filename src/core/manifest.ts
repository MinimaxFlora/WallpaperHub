import { ApiError } from "./errors";
import type { Env, ImageMeta, Manifest, Orientation } from "../types";

const DEFAULT_TTL_SECONDS = 300;
const DEFAULT_MANIFEST_KEY = "manifest.json";
const ORIENTATIONS: Orientation[] = ["landscape", "portrait", "square"];

interface CacheEntry {
  value: Manifest;
  expiresAt: number;
}

// Module scope survives across requests inside a Worker isolate, so a short TTL
// avoids re-reading KV on every request without making updates invisible.
let cached: CacheEntry | null = null;

export function resetManifestCache(): void {
  cached = null;
}

export function parseCacheTtl(raw: string | undefined): number {
  const seconds = Number(raw);
  if (!Number.isFinite(seconds) || seconds <= 0) return DEFAULT_TTL_SECONDS;
  return Math.floor(seconds);
}

export async function loadManifest(env: Env, now: number = Date.now()): Promise<Manifest> {
  const ttl = parseCacheTtl(env.MANIFEST_CACHE_TTL);
  if (cached && cached.expiresAt > now) return cached.value;

  const key = env.MANIFEST_KEY && env.MANIFEST_KEY.length > 0 ? env.MANIFEST_KEY : DEFAULT_MANIFEST_KEY;

  let text: string | null;
  try {
    text = await env.KV.get(key, "text");
  } catch {
    throw new ApiError(503, "manifest_unavailable", "manifest storage is unreachable");
  }
  if (text === null) {
    throw new ApiError(503, "manifest_unavailable", "manifest has not been published yet");
  }

  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    throw new ApiError(503, "manifest_unavailable", "manifest is not valid JSON");
  }

  const manifest = validateManifest(raw);
  cached = { value: manifest, expiresAt: now + ttl * 1000 };
  return manifest;
}

export function validateManifest(raw: unknown): Manifest {
  if (typeof raw !== "object" || raw === null) {
    throw new ApiError(503, "manifest_unavailable", "manifest is not an object");
  }
  const source = raw as Record<string, unknown>;
  if (!Array.isArray(source.images)) {
    throw new ApiError(503, "manifest_unavailable", "manifest has no image list");
  }

  const seen = new Set<string>();
  const images: ImageMeta[] = [];
  for (const entry of source.images) {
    const image = normalizeImage(entry);
    if (!image || seen.has(image.id)) continue;
    seen.add(image.id);
    images.push(image);
  }
  images.sort((a, b) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));

  return {
    version: typeof source.version === "number" ? source.version : 1,
    generated_at: typeof source.generated_at === "string" ? source.generated_at : "",
    timezone: typeof source.timezone === "string" ? source.timezone : "UTC",
    count: images.length,
    images,
  };
}

function normalizeImage(raw: unknown): ImageMeta | null {
  if (typeof raw !== "object" || raw === null) return null;
  const value = raw as Record<string, unknown>;

  const id = typeof value.id === "string" ? value.id.trim() : "";
  const path = typeof value.path === "string" ? value.path.trim() : "";
  const width = toPositiveInt(value.width);
  const height = toPositiveInt(value.height);
  if (!id || !path || width === null || height === null) return null;

  const orientation = ORIENTATIONS.includes(value.orientation as Orientation)
    ? (value.orientation as Orientation)
    : deriveOrientation(width, height);

  return {
    id,
    path,
    title: typeof value.title === "string" && value.title.length > 0 ? value.title : id,
    category: typeof value.category === "string" && value.category.length > 0 ? value.category : "uncategorized",
    tags: Array.isArray(value.tags)
      ? value.tags.filter((tag): tag is string => typeof tag === "string").map((tag) => tag.toLowerCase())
      : [],
    width,
    height,
    orientation,
    format: typeof value.format === "string" ? value.format : extensionOf(path),
    bytes: toPositiveInt(value.bytes) ?? 0,
    hash: typeof value.hash === "string" ? value.hash : "",
  };
}

function toPositiveInt(value: unknown): number | null {
  return typeof value === "number" && Number.isInteger(value) && value > 0 ? value : null;
}

export function deriveOrientation(width: number, height: number): Orientation {
  if (width > height) return "landscape";
  if (width < height) return "portrait";
  return "square";
}

function extensionOf(path: string): string {
  const dot = path.lastIndexOf(".");
  return dot >= 0 ? path.slice(dot + 1).toLowerCase() : "";
}
