import { describe, expect, it } from "vitest";
import { getBuildInfo } from "./build-info.js";

describe("getBuildInfo", () => {
  it("reports the correct package name", () => {
    expect(getBuildInfo().packageName).toBe("@ai-dos/dashboard");
  });

  it("uses the injected clock rather than a hidden real-time dependency", () => {
    const fixed = new Date("2026-01-01T00:00:00.000Z");
    const info = getBuildInfo(() => fixed);
    expect(info.builtAt).toBe("2026-01-01T00:00:00.000Z");
  });
});
