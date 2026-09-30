import { describe, expect, it } from "vitest";
import { applyFilter, matches, parseFilter } from "../src/core/filter";
import { ApiError } from "../src/core/errors";
import { sampleImage } from "./helpers";

describe("parseFilter", () => {
  it("normalizes and dedupes tags", () => {
    const filter = parseFilter(new URLSearchParams("tags=Anime, anime ,Night"));
    expect(filter.tags).toEqual(["anime", "night"]);
  });

  it("rejects an unknown orientation", () => {
    expect(() => parseFilter(new URLSearchParams("orientation=wide"))).toThrowError(ApiError);
  });

  it("rejects a non-numeric minimum size", () => {
    expect(() => parseFilter(new URLSearchParams("min_width=1080p"))).toThrowError(ApiError);
  });

  it("returns an empty filter when no parameters are given", () => {
    expect(parseFilter(new URLSearchParams())).toEqual({ tags: [] });
  });
});

describe("matches", () => {
  const image = sampleImage({ tags: ["anime", "night"], category: "anime", width: 1920, height: 1080 });

  it("requires every requested tag", () => {
    expect(matches(image, { tags: ["anime"] })).toBe(true);
    expect(matches(image, { tags: ["anime", "city"] })).toBe(false);
  });

  it("matches category and orientation", () => {
    expect(matches(image, { tags: [], category: "anime", orientation: "landscape" })).toBe(true);
    expect(matches(image, { tags: [], orientation: "portrait" })).toBe(false);
  });

  it("applies minimum dimensions independently", () => {
    expect(matches(image, { tags: [], minWidth: 1920 })).toBe(true);
    expect(matches(image, { tags: [], minHeight: 1200 })).toBe(false);
  });
});

describe("applyFilter", () => {
  it("keeps only matching images", () => {
    const images = [
      sampleImage({ id: "01", tags: ["anime"] }),
      sampleImage({ id: "02", tags: ["landscape"], category: "nature" }),
    ];
    expect(applyFilter(images, { tags: ["anime"] }).map((image) => image.id)).toEqual(["01"]);
  });
});
