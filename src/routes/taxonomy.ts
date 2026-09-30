import { jsonResponse } from "../core/errors";
import { loadManifest } from "../core/manifest";
import type { Env } from "../types";

export async function handleTags(env: Env): Promise<Response> {
  const manifest = await loadManifest(env);
  return jsonResponse({ items: countBy(manifest.images.flatMap((image) => image.tags)) });
}

export async function handleCategories(env: Env): Promise<Response> {
  const manifest = await loadManifest(env);
  return jsonResponse({ items: countBy(manifest.images.map((image) => image.category)) });
}

interface Count {
  name: string;
  count: number;
}

function countBy(values: string[]): Count[] {
  const counts = new Map<string, number>();
  for (const value of values) counts.set(value, (counts.get(value) ?? 0) + 1);
  return [...counts.entries()]
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count || (a.name < b.name ? -1 : 1));
}
