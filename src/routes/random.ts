import { jsonResponse } from "../core/errors";
import { applyFilter, parseFilter } from "../core/filter";
import { loadManifest } from "../core/manifest";
import { baseUrlFor, toApiImage } from "../core/serialize";
import { parseMode, resolveDate, safeTimezone, selectIndex } from "../core/selection";
import { sessionId } from "../core/session";
import type { Env } from "../types";

export async function handleRandom(request: Request, env: Env): Promise<Response> {
  const params = new URL(request.url).searchParams;
  const filter = parseFilter(params);
  const mode = parseMode(params.get("mode"));
  const timezone = safeTimezone(env.TIMEZONE);

  const manifest = await loadManifest(env);
  const candidates = applyFilter(manifest.images, filter);
  const date = mode === "daily" ? resolveDate(params.get("date"), timezone) : undefined;

  const index = selectIndex(
    {
      mode,
      seed: params.get("seed") ?? undefined,
      date,
      timezone,
      sessionId: sessionId(request),
    },
    candidates.length,
  );
  const image = candidates[index];
  const baseUrl = baseUrlFor(request, env);

  if (params.get("redirect") === "1") {
    return new Response(null, {
      status: 302,
      headers: { Location: `${baseUrl}/v1/images/${encodeURIComponent(image.id)}/file` },
    });
  }

  return jsonResponse({
    ...toApiImage(image, baseUrl),
    mode,
    ...(date ? { date } : {}),
    manifest_version: manifest.version,
  });
}
