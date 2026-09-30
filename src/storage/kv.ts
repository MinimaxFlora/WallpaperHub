import type { Env } from "../types";

// Workers KV has no native range reads, so the whole value is fetched and the
// caller slices it. Values are capped at 25 MiB by the platform.
export async function getImageBytes(env: Env, key: string): Promise<ArrayBuffer | null> {
  return env.KV.get(key, "arrayBuffer");
}
