import type { ApiImage, Env, ImageMeta } from "../types";

export function baseUrlFor(request: Request, env: Env): string {
  const configured = env.PUBLIC_BASE_URL?.trim();
  if (configured) return configured.replace(/\/+$/, "");
  return new URL(request.url).origin;
}

export function toApiImage(image: ImageMeta, baseUrl: string): ApiImage {
  const id = encodeURIComponent(image.id);
  return {
    id: image.id,
    title: image.title,
    category: image.category,
    tags: image.tags,
    orientation: image.orientation,
    width: image.width,
    height: image.height,
    format: image.format,
    bytes: image.bytes,
    hash: image.hash,
    url: `${baseUrl}/v1/images/${id}/file`,
    page_url: `${baseUrl}/v1/images/${id}`,
  };
}
