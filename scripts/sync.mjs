#!/usr/bin/env node
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { readFile, readdir, unlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import process from "node:process";

import * as imageSizeModule from "image-size";

// image-size exposes the same probe under different names across majors.
const probeImage = imageSizeModule.imageSize ?? imageSizeModule.default ?? imageSizeModule;

const SUPPORTED_EXTENSIONS = new Set([".jpg", ".jpeg", ".png", ".webp", ".avif", ".gif"]);
const MANIFEST_VERSION = 1;

const TARGET = argValue("--target") || process.env.SYNC_TARGET || "remote";
const IMAGES_DIR = process.env.IMAGES_DIR || "images";
const METADATA_FILE = process.env.METADATA_FILE || "metadata.json";
const MANIFEST_KEY = process.env.MANIFEST_KEY || "manifest.json";
const TIMEZONE = process.env.TIMEZONE || "Asia/Shanghai";
const PERSIST_TO = process.env.PERSIST_TO || ".wrangler/state";
const KV_BINDING = process.env.KV_BINDING || "KV";

function argValue(flag) {
  const index = process.argv.indexOf(flag);
  return index >= 0 ? process.argv[index + 1] : undefined;
}

function log(message) {
  process.stdout.write(`${message}\n`);
}

function fail(message) {
  process.stderr.write(`${message}\n`);
  process.exit(1);
}

export function deriveOrientation(width, height) {
  if (width > height) return "landscape";
  if (width < height) return "portrait";
  return "square";
}

async function walk(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const fullPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await walk(fullPath)));
    } else if (entry.isFile()) {
      files.push(fullPath);
    }
  }
  return files.sort();
}

function toPosix(value) {
  return value.split(path.sep).join("/");
}

function extensionOf(file) {
  return path.extname(file).toLowerCase();
}

async function loadMetadata() {
  if (!existsSync(METADATA_FILE)) return {};
  try {
    return JSON.parse(await readFile(METADATA_FILE, "utf8"));
  } catch (error) {
    fail(`failed to parse ${METADATA_FILE}: ${error.message}`);
  }
}

export async function buildManifest() {
  if (!existsSync(IMAGES_DIR)) fail(`image directory ${IMAGES_DIR} does not exist`);

  const metadata = await loadMetadata();
  const files = await walk(IMAGES_DIR);
  const unsupported = files.filter((file) => !SUPPORTED_EXTENSIONS.has(extensionOf(file)));
  if (unsupported.length > 0) {
    fail(
      `unsupported file type in ${IMAGES_DIR}:\n${unsupported
        .map((file) => `  - ${toPosix(file)}`)
        .join("\n")}\nallowed extensions: ${[...SUPPORTED_EXTENSIONS].join(", ")}`,
    );
  }

  const images = [];
  const seen = new Set();

  for (const file of files) {
    const relative = toPosix(file);
    const id = path.basename(file, path.extname(file));
    if (seen.has(id)) fail(`duplicate image id "${id}" (ids come from file names and must be unique)`);
    seen.add(id);

    const buffer = await readFile(file);
    let dimensions;
    try {
      dimensions = probeImage(buffer);
    } catch (error) {
      fail(`failed to read image dimensions for ${relative}: ${error.message}`);
    }
    if (!dimensions?.width || !dimensions?.height) {
      fail(`failed to detect dimensions for ${relative}`);
    }

    const meta = metadata[relative] ?? metadata[path.basename(file)] ?? metadata[id] ?? {};
    const digest = createHash("sha256").update(buffer).digest("hex");

    images.push({
      id,
      path: relative,
      title: meta.title ?? id,
      category: meta.category ?? (path.dirname(relative) === IMAGES_DIR ? "uncategorized" : toPosix(path.dirname(relative)).replace(`${IMAGES_DIR}/`, "")),
      tags: Array.isArray(meta.tags) ? meta.tags.map((tag) => String(tag).toLowerCase()) : [],
      width: dimensions.width,
      height: dimensions.height,
      orientation: deriveOrientation(dimensions.width, dimensions.height),
      format: (dimensions.type ?? extensionOf(file).slice(1)).toLowerCase(),
      bytes: buffer.byteLength,
      hash: `sha256-${digest}`,
    });
  }

  images.sort((a, b) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));

  return {
    version: MANIFEST_VERSION,
    generated_at: new Date().toISOString(),
    timezone: TIMEZONE,
    count: images.length,
    images,
  };
}

function kvConfig() {
  const { CF_API_TOKEN, CF_ACCOUNT_ID, KV_NAMESPACE_ID } = process.env;
  if (!CF_API_TOKEN || !CF_ACCOUNT_ID || !KV_NAMESPACE_ID) {
    fail("remote sync requires CF_API_TOKEN, CF_ACCOUNT_ID and KV_NAMESPACE_ID");
  }
  const base = process.env.CF_API_BASE || "https://api.cloudflare.com/client/v4";
  return { token: CF_API_TOKEN, base, account: CF_ACCOUNT_ID, namespace: KV_NAMESPACE_ID };
}

function valueUrl({ base, account, namespace }, key) {
  return `${base}/accounts/${account}/storage/kv/namespaces/${namespace}/values/${encodeURIComponent(key)}`;
}

async function getRemoteManifest(config) {
  const response = await fetch(valueUrl(config, MANIFEST_KEY), {
    headers: { Authorization: `Bearer ${config.token}` },
  });
  if (response.status === 404) return null;
  if (!response.ok) fail(`failed to read ${MANIFEST_KEY}: ${response.status} ${await response.text()}`);
  return response.json();
}

function contentTypeFor(format) {
  return { jpg: "image/jpeg", jpeg: "image/jpeg", png: "image/png", webp: "image/webp", avif: "image/avif", gif: "image/gif" }[format] ?? "application/octet-stream";
}

async function putRemote(config, key, body, contentType) {
  const response = await fetch(valueUrl(config, key), {
    method: "PUT",
    body,
    headers: { Authorization: `Bearer ${config.token}`, "Content-Type": contentType },
  });
  if (!response.ok) fail(`failed to upload ${key}: ${response.status} ${await response.text()}`);
}

function putLocal(key, filePath) {
  const result = spawnSync(
    "npx",
    ["wrangler", "kv", "key", "put", key, "--binding", KV_BINDING, "--path", filePath, "--local", "--persist-to", PERSIST_TO],
    { stdio: "inherit" },
  );
  if (result.status !== 0) fail(`failed to upload ${key} to local KV`);
}

async function putLocalManifest(manifest) {
  const tempFile = path.join(tmpdir(), `wallpaperhub-manifest-${process.pid}.json`);
  await writeFile(tempFile, `${JSON.stringify(manifest, null, 2)}\n`, "utf8");
  try {
    putLocal(MANIFEST_KEY, tempFile);
  } finally {
    await unlink(tempFile).catch(() => {});
  }
}

export async function sync({ target = TARGET } = {}) {
  const manifest = await buildManifest();
  log(`discovered ${manifest.count} images in ${IMAGES_DIR}`);

  if (target === "local") {
    for (const image of manifest.images) {
      putLocal(image.path, image.path);
    }
    await putLocalManifest(manifest);
    log(`uploaded ${manifest.count} images and ${MANIFEST_KEY} to local KV (${PERSIST_TO})`);
    return manifest;
  }

  const config = kvConfig();
  const existing = await getRemoteManifest(config);
  const knownHashes = new Map((existing?.images ?? []).map((image) => [image.path, image.hash]));

  let uploaded = 0;
  for (const image of manifest.images) {
    if (knownHashes.get(image.path) === image.hash) continue;
    await putRemote(config, image.path, await readFile(image.path), contentTypeFor(image.format));
    uploaded += 1;
    log(`uploaded ${image.path}`);
  }

  const unchanged = existing && JSON.stringify(existing.images) === JSON.stringify(manifest.images);
  if (!unchanged) {
    await putRemote(config, MANIFEST_KEY, JSON.stringify(manifest, null, 2), "application/json");
    log(`uploaded ${MANIFEST_KEY}`);
  }

  log(`sync complete: ${uploaded} image(s) uploaded, manifest ${unchanged ? "unchanged" : "updated"}`);
  return manifest;
}

const invokedDirectly = process.argv[1] && path.resolve(process.argv[1]) === path.resolve(new URL(import.meta.url).pathname);
if (invokedDirectly) {
  sync().catch((error) => fail(error?.stack ?? String(error)));
}
