import { describe, expect, it } from "vitest";
import {
  AIDosError,
  ConflictError,
  ForbiddenError,
  InternalError,
  NotFoundError,
  UnauthorizedError,
  ValidationError,
} from "./errors.js";

describe("AIDosError hierarchy", () => {
  it("each subclass carries the correct code and HTTP status", () => {
    expect(new NotFoundError("x").httpStatus).toBe(404);
    expect(new ValidationError("x").httpStatus).toBe(400);
    expect(new UnauthorizedError("x").httpStatus).toBe(401);
    expect(new ForbiddenError("x").httpStatus).toBe(403);
    expect(new ConflictError("x").httpStatus).toBe(409);
    expect(new InternalError("x").httpStatus).toBe(500);
  });

  it("every subclass is an instanceof both itself and the AIDosError base", () => {
    // This is the exact check that silently breaks if the
    // Object.setPrototypeOf calls in errors.ts are ever removed.
    const notFound = new NotFoundError("missing");
    expect(notFound instanceof NotFoundError).toBe(true);
    expect(notFound instanceof AIDosError).toBe(true);
    expect(notFound instanceof Error).toBe(true);
  });

  it("is catchable and distinguishable in a try/catch by instanceof", () => {
    function throwIt(): never {
      throw new ValidationError("bad input");
    }

    try {
      throwIt();
      expect.fail("expected throwIt to throw");
    } catch (e) {
      expect(e instanceof ValidationError).toBe(true);
      expect(e instanceof NotFoundError).toBe(false);
      if (e instanceof AIDosError) {
        expect(e.code).toBe("validation_error");
      } else {
        expect.fail("expected e to be an AIDosError");
      }
    }
  });

  it("toProblemDetails never includes the cause", () => {
    const cause = new Error("postgres: connection refused on 10.0.0.5");
    const wrapped = new InternalError("could not complete request", cause);

    const pd = wrapped.toProblemDetails();
    const serialized = JSON.stringify(pd);

    expect(pd.detail).toBe("could not complete request");
    expect(serialized.includes("10.0.0.5")).toBe(false);
    expect(serialized.includes("postgres")).toBe(false);
  });

  it("preserves the cause on the instance itself, for logging, even though it's excluded from serialization", () => {
    const cause = new Error("root cause");
    const wrapped = new InternalError("wrapped", cause);
    expect(wrapped.cause).toBe(cause);
  });

  it("has the correct name property per subclass, for log readability", () => {
    expect(new NotFoundError("x").name).toBe("NotFoundError");
    expect(new ConflictError("x").name).toBe("ConflictError");
  });
});
