import { ApiError } from "./errors";
import type { ImageMeta, Orientation } from "../types";

export const ORIENTATIONS: readonly Orientation[] = ["landscape", "portrait", "square"];

export interface ImageFilter {
  tags: string[];
  category?: string;
  orientation?: Orientation;
  minWidth?: number;
  minHeight?: number;
}

export function parseFilter(params: URLSearchParams): ImageFilter {
  const filter: ImageFilter = { tags: [] };

  const tags = params.get("tags");
  if (tags) {
    filter.tags = [
      ...new Set(
        tags
          .split(",")
          .map((tag) => tag.trim().toLowerCase())
          .filter((tag) => tag.length > 0),
      ),
    ];
  }

  const category = params.get("category");
  if (category) filter.category = category.trim().toLowerCase();

  const orientation = params.get("orientation");
  if (orientation) {
    if (!ORIENTATIONS.includes(orientation as Orientation)) {
      throw new ApiError(400, "invalid_orientation", `orientation must be one of ${ORIENTATIONS.join(", ")}`);
    }
    filter.orientation = orientation as Orientation;
  }

  filter.minWidth = parseSize(params.get("min_width"), "min_width");
  filter.minHeight = parseSize(params.get("min_height"), "min_height");
  return filter;
}

function parseSize(raw: string | null, field: string): number | undefined {
  if (raw === null || raw === "") return undefined;
  if (!/^\d+$/.test(raw)) {
    throw new ApiError(400, "invalid_size", `${field} must be a non-negative integer`);
  }
  return Number(raw);
}

export function matches(image: ImageMeta, filter: ImageFilter): boolean {
  if (filter.tags.length > 0 && !filter.tags.every((tag) => image.tags.includes(tag))) return false;
  if (filter.category !== undefined && image.category !== filter.category) return false;
  if (filter.orientation !== undefined && image.orientation !== filter.orientation) return false;
  if (filter.minWidth !== undefined && image.width < filter.minWidth) return false;
  if (filter.minHeight !== undefined && image.height < filter.minHeight) return false;
  return true;
}

export function applyFilter(images: ImageMeta[], filter: ImageFilter): ImageMeta[] {
  return images.filter((image) => matches(image, filter));
}
