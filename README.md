# WallpaperHub

An edge wallpaper API. Images live in this GitHub repository, a sync pipeline
publishes them to Workers KV, and a Cloudflare Worker serves them through a
small REST API with search, filtering and four wallpaper selection modes.

There is no origin server to run and no image bytes ever travel through a VPS.

## How it works

```
GitHub repo  --push-->  GitHub Actions  --upload-->  Workers KV
                                                          ^
                                  client  --HTTPS-->  Cloudflare Worker
```

1. You add an image file to `images/` and push.
2. The `sync-kv` workflow validates the images, computes their metadata and
   uploads only the new or changed keys to KV, together with `manifest.json`.
3. The Worker reads `manifest.json` (cached briefly) to answer search and
   selection requests, and reads image bytes straight from KV for file requests.

## API

All routes live under `/v1`. Responses are JSON unless the route serves image
bytes. Errors always use `{"error":{"code":"...","message":"..."}}`.

| Route | Purpose |
|-------|---------|
| `GET /v1/random` | Return one image matching the filters |
| `GET /v1/images` | List images with filters and pagination |
| `GET /v1/images/{id}` | Image metadata |
| `GET /v1/images/{id}/file` | Image bytes (supports `Range`, `ETag`, `HEAD`) |
| `GET /v1/tags` | All tags with image counts |
| `GET /v1/categories` | All categories with image counts |
| `GET /healthz` | Liveness plus manifest info |

### Selection modes

`/v1/random` returns a different image on every request by default. Use `mode`
to change that:

| `mode` | Behaviour |
|--------|-----------|
| `random` | Uniform random pick per request (default) |
| `seed` | Same `seed` and filters always return the same image |
| `daily` | Stable within one day in `TIMEZONE`, changes at midnight |
| `session` | Stable per `wh_session` cookie, falling back to client IP |

An unknown `mode` falls back to `random`.

### Filter parameters

Available on `/v1/random` and `/v1/images`:

| Parameter | Meaning |
|-----------|---------|
| `tags` | Comma separated, every tag must match, case insensitive |
| `category` | Single category |
| `orientation` | `landscape`, `portrait` or `square` |
| `min_width` | Minimum width in pixels |
| `min_height` | Minimum height in pixels |
| `page`, `page_size` | Pagination for `/v1/images` (`page_size` max 100) |
| `redirect=1` | On `/v1/random`, reply `302` to the image file instead of JSON |

### Examples

```bash
# Random image matching a tag
curl "https://wallpaper.example.com/v1/random?tags=anime"

# Reproducible pick
curl "https://wallpaper.example.com/v1/random?mode=seed&seed=hello"

# Same image all day in the configured timezone
curl "https://wallpaper.example.com/v1/random?mode=daily"

# Hot-linkable image URL, ready for a CSS background
curl -I "https://wallpaper.example.com/v1/random?redirect=1"

# Portrait wallpapers at least 1440px wide
curl "https://wallpaper.example.com/v1/images?orientation=portrait&min_width=1440"
```

An image response looks like this:

```json
{
  "id": "01",
  "title": "Blue Haired Girl",
  "category": "anime",
  "tags": ["anime", "illustration"],
  "orientation": "landscape",
  "width": 3840,
  "height": 2160,
  "format": "webp",
  "bytes": 504578,
  "hash": "sha256-db9824d46e394426581b4aaf075126289710787194aae2de553723fe3bd50f7f",
  "url": "https://wallpaper.example.com/v1/images/01/file",
  "page_url": "https://wallpaper.example.com/v1/images/01",
  "mode": "random",
  "manifest_version": 1
}
```

## Adding wallpapers

1. Put the file in `images/`. The file name without its extension becomes the
   `id`, so names must be unique, for example `images/09.webp`.
2. Optionally describe it in `metadata.json`, keyed by the repository path,
   the file name or the id:

   ```json
   {
     "images/09.webp": {
       "title": "Night Sky",
       "category": "landscape",
       "tags": ["sky", "night"]
     }
   }
   ```

   Missing fields fall back to the id as title, `uncategorized` as category and
   an empty tag list.
3. Commit and push to `master`. The workflow validates the files, computes
   dimensions and digests, uploads to KV and refreshes the manifest.

Supported formats are `.jpg`, `.jpeg`, `.png`, `.webp`, `.avif` and `.gif`. Any
other file inside `images/` fails the workflow instead of being ignored, so the
directory cannot silently collect junk.

Only changed objects are uploaded, so re-running the workflow is cheap and
idempotent.

## Deploy to Cloudflare

### 1. Prerequisites

- A Cloudflare account. The free plan is enough.
- Node.js 22 or newer.
- Wrangler authenticated locally:

  ```bash
  npx wrangler login
  ```

### 2. Create the KV namespace

```bash
npx wrangler kv namespace create KV
```

Wrangler prints the namespace id. Put it into `kv_namespaces[0].id` in
`wrangler.jsonc`, replacing `REPLACE_WITH_KV_NAMESPACE_ID`.

### 3. Create a Cloudflare API token for the workflow

Workers KV is written through the Cloudflare REST API, so the workflow needs an
API token rather than an S3 key pair. In the dashboard open **My Profile > API
Tokens > Create Token**, start from the **Edit Cloudflare Workers** template or
create a custom token with **Account > Workers KV Storage > Edit** permission.
Then note the token value, your account ID and the namespace id from step 2.

### 4. Configure repository secrets

In **Settings > Secrets and variables > Actions** add:

| Secret | Value |
|--------|-------|
| `CF_API_TOKEN` | API token with Workers KV Storage edit permission |
| `CF_ACCOUNT_ID` | Cloudflare account ID |
| `KV_NAMESPACE_ID` | KV namespace id from step 2 |

Optionally add the repository variable `WALLPAPER_TIMEZONE` (for example
`Asia/Shanghai`) to control the `daily` mode timezone.

### 5. Publish the images and manifest

Either push to `master`, or trigger the `sync-kv` workflow manually from the
Actions tab. To do it from your machine without a push:

```bash
npm install

CF_API_TOKEN=... CF_ACCOUNT_ID=... KV_NAMESPACE_ID=... \
  node scripts/sync.mjs --target remote
```

### 6. Deploy the Worker

```bash
npm install
npx wrangler deploy
```

Wrangler prints the `*.workers.dev` URL. Verify it:

```bash
curl "https://wallpaperhub.<subdomain>.workers.dev/healthz"
```

### 7. Attach a custom domain

In the dashboard open **Workers & Pages > wallpaperhub > Settings > Domains &
Routes**, add your custom domain, or declare a route in `wrangler.jsonc`. Once
the domain is live, set `PUBLIC_BASE_URL` so absolute URLs in responses point at
it:

```bash
npx wrangler deploy
```

`PUBLIC_BASE_URL` is a plain `vars` entry in `wrangler.jsonc`. Editing `vars`
requires a redeploy, or you can override it from the dashboard under
**Settings > Variables**.

## Local development

```bash
npm install

# Seed the local KV namespace with the current images and manifest
node scripts/sync.mjs --target local

# Start the Worker on http://127.0.0.1:8787
npm run dev
```

```bash
npm run typecheck
npm test
npx wrangler deploy --dry-run
```

Local state is stored under `.wrangler/state`. Delete that directory if you want
a clean namespace.

## Self-hosting with Docker

Docker runs the same Worker through `wrangler dev` with a local KV backed by a
volume, so nothing is sent to Cloudflare.

```bash
docker compose up -d --build

# Seed the container's local KV namespace
docker compose run --rm wallpaper npm run sync -- --target local

curl "http://127.0.0.1:8787/healthz"
```

After adding images, rebuild and re-run the sync command. State lives in the
`wallpaper-state` volume.

The image is based on `node:22-bookworm-slim` because the `workerd` runtime
linked against glibc and is unreliable on musl-based images.

## Storage limits

Workers KV on the free plan allows 1 GB of storage, 100,000 reads per day,
1,000 writes per day to distinct keys, and 25 MiB per value. A Worker on the
free plan is itself capped at 100,000 requests per day, so in normal use that
request cap is reached before the KV read cap.

Image responses are cached in the edge Cache API, so repeated requests for the
same image do not consume KV reads. The manifest is cached in Worker memory for
`MANIFEST_CACHE_TTL` seconds, so an isolate reads it at most once per window.

KV has no native byte-range reads. `/v1/images/{id}/file` fetches the whole
value and slices it for `Range` requests, which is fine for wallpaper-sized
files. Keep individual values below 25 MiB.

Switching storage backends stays confined to `src/storage/kv.ts`, the binding in
`wrangler.jsonc` and the upload path in `scripts/sync.mjs`.

## Configuration

Variables live in `wrangler.jsonc` under `vars`. Env vars with the same names
override them in Docker and local runs.

| Variable | Default | Meaning |
|----------|---------|---------|
| `PUBLIC_BASE_URL` | request origin | Base URL used for absolute links in responses |
| `MANIFEST_KEY` | `manifest.json` | KV key of the manifest |
| `MANIFEST_CACHE_TTL` | `300` | Seconds a Worker isolate caches the manifest |
| `TIMEZONE` | `UTC` | Timezone used by `mode=daily` |
| `RATE_LIMIT_ENABLED` | `true` | Set to `false` to disable rate limiting |
| `HOTLINK_ALLOWLIST` | empty | Comma separated hosts allowed to load image files |

`HOTLINK_ALLOWLIST` accepts exact hosts and `*.example.com` wildcards. When it is
empty, image files are served to any client. Requests without an `Origin` and
`Referer` are always allowed so direct links and scripts keep working.

### Rate limiting

The `unsafe.bindings` section declares two Cloudflare rate limiters, one for the
JSON API and one for image files. They are marked experimental by Cloudflare.
When a binding is missing or errors, the Worker falls back to a per-isolate
in-memory counter instead of failing requests. To change the limits, edit the
`simple.limit` values in `wrangler.jsonc` and redeploy.

## Migration from the Go/VPS version

The previous implementation ran a Go binary on a VPS and proxied GitHub raw
bytes. It is gone from this repository; the running VPS service is untouched
until you switch DNS.

1. Deploy the Worker and verify it against a custom domain using the steps
   above, keeping the old endpoint live.
2. Confirm image bytes match the old service:

   ```bash
   curl -s "https://wallpaper.YOURDOMAIN/v1/images/01/file" | md5sum
   ```

3. Point the DNS record at the Worker, or attach the domain in the Cloudflare
   dashboard.
4. Once traffic has moved, shut down the old service and Caddy site block on the
   VPS.

Rollback is a DNS change back to the VPS; the previous Go sources remain in git
history.

## Repository layout

```
src/                 Worker source
  core/              Manifest, filtering, selection, serialisation
  http/              Caching, hotlink protection, rate limiting
  routes/            Route handlers
  storage/           KV access
test/                Vitest suites
scripts/sync.mjs     Sync pipeline used by CI and local runs
images/              Wallpaper files
metadata.json        Optional titles, categories and tags
.github/workflows/   GitHub Actions pipeline
wrangler.jsonc       Worker configuration
docker-compose.yml   Self-hosting stack
```
