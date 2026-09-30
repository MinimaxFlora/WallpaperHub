import { jsonResponse } from "../core/errors";
import { loadManifest } from "../core/manifest";
import type { Env } from "../types";

export async function handleHealth(env: Env): Promise<Response> {
  const manifest = await loadManifest(env);
  return jsonResponse({
    status: "ok",
    images: manifest.images.length,
    manifest_version: manifest.version,
    manifest_generated_at: manifest.generated_at,
  });
}
