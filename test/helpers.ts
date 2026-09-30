import type { Env, ImageMeta, Manifest } from "../src/types";

export function sampleImage(overrides: Partial<ImageMeta> = {}): ImageMeta {
  return {
    id: "01",
    path: "images/01.webp",
    title: "Sample",
    category: "anime",
    tags: ["anime"],
    width: 1920,
    height: 1080,
    orientation: "landscape",
    format: "webp",
    bytes: 100,
    hash: "sha256-abc",
    ...overrides,
  };
}

export function sampleManifest(images: ImageMeta[]): Manifest {
  return {
    version: 1,
    generated_at: "2026-09-30T00:00:00Z",
    timezone: "UTC",
    count: images.length,
    images,
  };
}

export function fakeEnv(manifest: Manifest | null, objects: Record<string, Uint8Array> = {}): Env {
  const store = new Map<string, string | Uint8Array>();
  if (manifest !== null) store.set("manifest.json", JSON.stringify(manifest));
  for (const [key, value] of Object.entries(objects)) store.set(key, value);

  const KV = {
    get: async (key: string, options?: string | { type?: string }) => {
      const value = store.get(key);
      if (value === undefined) return null;
      const type = typeof options === "string" ? options : options?.type;
      const bytes = typeof value === "string" ? new TextEncoder().encode(value) : value;
      if (type === "arrayBuffer") {
        return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
      }
      return typeof value === "string" ? value : new TextDecoder().decode(value);
    },
    put: async (key: string, value: string | Uint8Array) => {
      store.set(key, value);
    },
    delete: async (key: string) => {
      store.delete(key);
    },
    list: async () => ({ keys: [], list_complete: true, cacheStatus: null }),
  };

  return {
    KV: KV as unknown as KVNamespace,
    TIMEZONE: "UTC",
    MANIFEST_CACHE_TTL: "300",
    RATE_LIMIT_ENABLED: "false",
    HOTLINK_ALLOWLIST: "",
  };
}

export const fakeContext = {
  waitUntil: () => {},
  passThroughOnException: () => {},
} as unknown as ExecutionContext;
