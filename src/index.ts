import { ApiError, errorResponse, jsonResponse } from "./core/errors";
import { baseUrlFor } from "./core/serialize";
import { clientKey, enforceRateLimit } from "./http/ratelimit";
import { handleHealth } from "./routes/health";
import { handleImageFile, handleImageMeta } from "./routes/image";
import { handleList } from "./routes/list";
import { handleRandom } from "./routes/random";
import { handleCategories, handleTags } from "./routes/taxonomy";
import type { Env } from "./types";

const IMAGE_META = /^\/v1\/images\/([^/]+)$/;
const IMAGE_FILE = /^\/v1\/images\/([^/]+)\/file$/;

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    const started = Date.now();
    const url = new URL(request.url);

    let response: Response;
    try {
      response = await route(request, url, env, ctx);
    } catch (error) {
      if (!(error instanceof ApiError)) console.error("unhandled error", error);
      response = errorResponse(error);
    }

    response.headers.set("Access-Control-Allow-Origin", "*");
    response.headers.set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS");
    response.headers.set("X-Content-Type-Options", "nosniff");

    console.log(
      JSON.stringify({
        method: request.method,
        path: url.pathname,
        status: response.status,
        ms: Date.now() - started,
      }),
    );
    return response;
  },
} satisfies ExportedHandler<Env>;

async function route(request: Request, url: URL, env: Env, ctx: ExecutionContext): Promise<Response> {
  const path = url.pathname.length > 1 ? url.pathname.replace(/\/+$/, "") : url.pathname;

  if (request.method === "OPTIONS") return new Response(null, { status: 204 });
  if (request.method !== "GET" && request.method !== "HEAD") {
    throw new ApiError(405, "method_not_allowed", "only GET, HEAD and OPTIONS are supported");
  }

  const fileMatch = IMAGE_FILE.exec(path);
  if (path !== "/healthz") {
    const scope = fileMatch ? "file" : "api";
    const limiter = fileMatch ? env.FILE_RATE_LIMIT : env.API_RATE_LIMIT;
    await enforceRateLimit(env, limiter, `${scope}:${clientKey(request)}`);
  }

  if (path === "/" || path === "/v1") return jsonResponse(apiIndex(baseUrlFor(request, env)));
  if (path === "/healthz") return handleHealth(env);
  if (path === "/v1/random") return handleRandom(request, env);
  if (path === "/v1/images") return handleList(request, env);
  if (path === "/v1/tags") return handleTags(env);
  if (path === "/v1/categories") return handleCategories(env);
  if (fileMatch) return handleImageFile(decodeURIComponent(fileMatch[1]), request, env, ctx);

  const metaMatch = IMAGE_META.exec(path);
  if (metaMatch) return handleImageMeta(decodeURIComponent(metaMatch[1]), request, env);

  throw new ApiError(404, "not_found", "no such route");
}

function apiIndex(base: string): Record<string, unknown> {
  return {
    name: "WallpaperHub API",
    version: 2,
    endpoints: {
      random: `${base}/v1/random`,
      images: `${base}/v1/images`,
      image: `${base}/v1/images/{id}`,
      file: `${base}/v1/images/{id}/file`,
      tags: `${base}/v1/tags`,
      categories: `${base}/v1/categories`,
      health: `${base}/healthz`,
    },
  };
}
