import { describe, expect, it } from "vitest";
import { clampLimit, DEFAULT_PAGE_LIMIT, MAX_PAGE_LIMIT } from "./pagination.js";

describe("clampLimit", () => {
  it("returns the default when no limit is requested", () => {
    expect(clampLimit(undefined)).toBe(DEFAULT_PAGE_LIMIT);
  });

  it("returns the default when a non-positive limit is requested", () => {
    expect(clampLimit(0)).toBe(DEFAULT_PAGE_LIMIT);
    expect(clampLimit(-5)).toBe(DEFAULT_PAGE_LIMIT);
  });

  it("returns the requested limit when within bounds", () => {
    expect(clampLimit(50)).toBe(50);
  });

  it("clamps to MAX_PAGE_LIMIT when the request exceeds it", () => {
    expect(clampLimit(1000)).toBe(MAX_PAGE_LIMIT);
  });

  it("allows exactly MAX_PAGE_LIMIT", () => {
    expect(clampLimit(MAX_PAGE_LIMIT)).toBe(MAX_PAGE_LIMIT);
  });
});
