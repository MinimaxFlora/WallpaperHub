import { ApiError, jsonResponse } from "../core/errors";
import { applyFilter, parseFilter } from "../core/filter";
import { loadManifest } from "../core/manifest";
import { baseUrlFor, toApiImage } from "../core/serialize";
import type { Env } from "../types";

const DEFAULT_PAGE_SIZE = 24;
const MAX_PAGE_SIZE = 100;

export async function handleList(request: Request, env: Env): Promise<Response> {
  const params = new URL(request.url).searchParams;
  const filter = parseFilter(params);
  const page = parsePositive(params.get("page"), "page", 1);
  const pageSize = Math.min(parsePositive(params.get("page_size"), "page_size", DEFAULT_PAGE_SIZE), MAX_PAGE_SIZE);

  const manifest = await loadManifest(env);
  const matched = applyFilter(manifest.images, filter);
  const start = (page - 1) * pageSize;
  const baseUrl = baseUrlFor(request, env);

  return jsonResponse({
    total: matched.length,
    page,
    page_size: pageSize,
    items: matched.slice(start, start + pageSize).map((image) => toApiImage(image, baseUrl)),
  });
}

function parsePositive(raw: string | null, field: string, fallback: number): number {
  if (raw === null || raw === "") return fallback;
  if (!/^\d+$/.test(raw) || Number(raw) < 1) {
    throw new ApiError(400, "invalid_pagination", `${field} must be a positive integer`);
  }
  return Number(raw);
}
