import { describe, expect, it } from "vitest";
import { err, isErr, isOk, map, ok, unwrapOr } from "./result.js";

describe("Result", () => {
  it("ok() produces a success result carrying the value", () => {
    const result = ok(42);
    expect(result.ok).toBe(true);
    expect(isOk(result)).toBe(true);
    expect(isErr(result)).toBe(false);
  });

  it("err() produces a failure result carrying the error", () => {
    const result = err("not found");
    expect(result.ok).toBe(false);
    expect(isErr(result)).toBe(true);
    expect(isOk(result)).toBe(false);
  });

  it("unwrapOr returns the value for a success result", () => {
    expect(unwrapOr(ok(42), 0)).toBe(42);
  });

  it("unwrapOr returns the fallback for a failure result", () => {
    expect(unwrapOr(err("boom"), 0)).toBe(0);
  });

  it("map transforms a success value", () => {
    const result = map(ok(2), (n) => n * 10);
    expect(isOk(result) && result.value).toBe(20);
  });

  it("map passes an error through unchanged, never calling the transform", () => {
    let called = false;
    const result = map(err<string>("boom"), (n: number) => {
      called = true;
      return n * 10;
    });
    expect(isErr(result) && result.error).toBe("boom");
    expect(called).toBe(false);
  });

  it("narrows the type correctly through isOk (compile-time check via runtime access)", () => {
    const result: ReturnType<typeof ok<number>> | ReturnType<typeof err<string>> = ok(5);
    if (isOk(result)) {
      // If this line didn't type-check, `tsc --noEmit` would fail the build.
      const doubled: number = result.value * 2;
      expect(doubled).toBe(10);
    } else {
      throw new Error("expected ok result");
    }
  });
});
