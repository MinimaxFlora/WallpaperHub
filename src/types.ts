export type Orientation = "landscape" | "portrait" | "square";

export interface ImageMeta {
  id: string;
  path: string;
  title: string;
  category: string;
  tags: string[];
  width: number;
  height: number;
  orientation: Orientation;
  format: string;
  bytes: number;
  hash: string;
}

export interface Manifest {
  version: number;
  generated_at: string;
  timezone: string;
  count: number;
  images: ImageMeta[];
}

export interface ApiImage {
  id: string;
  title: string;
  category: string;
  tags: string[];
  orientation: Orientation;
  width: number;
  height: number;
  format: string;
  bytes: number;
  hash: string;
  url: string;
  page_url: string;
}

export interface RateLimitBinding {
  limit(options: { key: string }): Promise<{ success: boolean }>;
}

export interface Env {
  KV: KVNamespace;
  PUBLIC_BASE_URL?: string;
  MANIFEST_KEY?: string;
  MANIFEST_CACHE_TTL?: string;
  TIMEZONE?: string;
  RATE_LIMIT_ENABLED?: string;
  HOTLINK_ALLOWLIST?: string;
  API_RATE_LIMIT?: RateLimitBinding;
  FILE_RATE_LIMIT?: RateLimitBinding;
}
