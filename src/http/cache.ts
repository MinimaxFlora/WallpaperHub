export const IMAGE_CACHE_CONTROL = "public, max-age=31536000, immutable";

// The Cache API exists inside the Workers runtime only; tests run on Node.
export function getEdgeCache(): Cache | null {
  if (typeof caches === "undefined") return null;
  const scoped = caches as unknown as { default?: Cache };
  return scoped.default ?? null;
}
