import { ApiError, jsonResponse } from "../core/errors";
import { loadManifest } from "../core/manifest";
import { baseUrlFor, toApiImage } from "../core/serialize";
import { getEdgeCache, IMAGE_CACHE_CONTROL } from "../http/cache";
import { checkHotlink } from "../http/hotlink";
import { getImageBytes } from "../storage/kv";
import type { Env, ImageMeta } from "../types";

const CONTENT_TYPES: Record<string, string> = {
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  png: "image/png",
  webp: "image/webp",
  avif: "image/avif",
  gif: "image/gif",
};

export async function handleImageMeta(id: string, request: Request, env: Env): Promise<Response> {
  const image = await findImage(env, id);
  return jsonResponse(toApiImage(image, baseUrlFor(request, env)));
}

export async function handleImageFile(
  id: string,
  request: Request,
  env: Env,
  ctx: ExecutionContext,
): Promise<Response> {
  checkHotlink(request, env);
  const image = await findImage(env, id);
  const etag = `"${image.hash}"`;

  if (request.headers.get("If-None-Match") === etag) {
    return new Response(null, { status: 304, headers: { ETag: etag, "Cache-Control": IMAGE_CACHE_CONTROL } });
  }

  // The manifest already records the byte size, so HEAD needs no storage round trip.
  if (request.method === "HEAD") {
    return new Response(null, {
      status: 200,
      headers: { ...imageHeaders(image, etag), "Content-Length": String(image.bytes) },
    });
  }

  const cache = getEdgeCache();
  const cacheKey = cacheKeyFor(request, image);
  const full = await loadFullImage(env, image, etag, cache, cacheKey, ctx);

  const rangeHeader = request.headers.get("Range");
  if (!rangeHeader) return full;

  const range = parseRange(rangeHeader, image.bytes);
  const buffer = await full.arrayBuffer();
  const end = range.offset + range.length - 1;
  return new Response(buffer.slice(range.offset, range.offset + range.length), {
    status: 206,
    headers: {
      ...imageHeaders(image, etag),
      "Content-Length": String(range.length),
      "Content-Range": `bytes ${range.offset}-${end}/${image.bytes}`,
    },
  });
}

async function loadFullImage(
  env: Env,
  image: ImageMeta,
  etag: string,
  cache: Cache | null,
  cacheKey: Request,
  ctx: ExecutionContext,
): Promise<Response> {
  if (cache) {
    const hit = await cache.match(cacheKey);
    if (hit) return hit;
  }

  const data = await getImageBytes(env, image.path);
  if (!data) throw new ApiError(503, "image_unavailable", "image is missing from storage");

  const response = new Response(data, {
    status: 200,
    headers: { ...imageHeaders(image, etag), "Content-Length": String(data.byteLength) },
  });
  if (cache) ctx.waitUntil(cache.put(cacheKey, response.clone()));
  return response;
}

function cacheKeyFor(request: Request, image: ImageMeta): Request {
  const url = new URL(request.url);
  url.pathname = `/v1/images/${encodeURIComponent(image.id)}/file`;
  url.search = `?hash=${encodeURIComponent(image.hash)}`;
  return new Request(url.toString(), { method: "GET" });
}

function imageHeaders(image: ImageMeta, etag: string): Record<string, string> {
  return {
    "Content-Type": contentTypeFor(image),
    "Cache-Control": IMAGE_CACHE_CONTROL,
    "Accept-Ranges": "bytes",
    ETag: etag,
  };
}

export function parseRange(header: string, size: number): { offset: number; length: number } {
  const match = /^bytes=(\d*)-(\d*)$/.exec(header.trim());
  if (!match) throw new ApiError(416, "invalid_range", "only a single byte range is supported");

  const [, rawStart, rawEnd] = match;
  if (rawStart === "" && rawEnd === "") throw new ApiError(416, "invalid_range", "empty byte range");

  if (rawStart === "") {
    const suffix = Number(rawEnd);
    if (suffix <= 0) throw new ApiError(416, "invalid_range", "suffix length must be positive");
    const offset = Math.max(size - suffix, 0);
    return { offset, length: size - offset };
  }

  const offset = Number(rawStart);
  const end = rawEnd === "" ? size - 1 : Math.min(Number(rawEnd), size - 1);
  if (offset >= size || end < offset) throw new ApiError(416, "invalid_range", "range is outside the image");
  return { offset, length: end - offset + 1 };
}

async function findImage(env: Env, id: string): Promise<ImageMeta> {
  const manifest = await loadManifest(env);
  const image = manifest.images.find((candidate) => candidate.id === id);
  if (!image) throw new ApiError(404, "not_found", `image ${id} does not exist`);
  return image;
}

function contentTypeFor(image: ImageMeta): string {
  return CONTENT_TYPES[image.format] ?? "application/octet-stream";
}
