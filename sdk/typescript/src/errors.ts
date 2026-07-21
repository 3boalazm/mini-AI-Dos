/**
 * AIDosError is the base of every error this SDK throws. It mirrors
 * services/foundation/errors' Code enum on the Go side field for
 * field — both halves of this polyglot foundation draw the same line
 * between what's generic infrastructure (here) and what's
 * provider-specific (the ProviderError taxonomy — RateLimitedError,
 * AuthError, and so on — which belongs to the Provider Adapter
 * Framework, roadmap epic 5/6, not Project Foundation, on both the Go
 * and TypeScript sides equally).
 */
export type ErrorCode =
  "not_found" | "validation_error" | "unauthorized" | "forbidden" | "conflict" | "internal_error";

const HTTP_STATUS_BY_CODE: Record<ErrorCode, number> = {
  not_found: 404,
  validation_error: 400,
  unauthorized: 401,
  forbidden: 403,
  conflict: 409,
  internal_error: 500,
};

export class AIDosError extends Error {
  readonly code: ErrorCode;
  readonly httpStatus: number;
  /** The underlying cause, if any. Never serialized — see toProblemDetails. */
  override readonly cause?: unknown;

  constructor(code: ErrorCode, message: string, cause?: unknown) {
    super(message);
    this.name = "AIDosError";
    this.code = code;
    this.httpStatus = HTTP_STATUS_BY_CODE[code];
    this.cause = cause;
    // Restores the correct prototype chain when compiled down to ES5-
    // style target environments — a well-known TypeScript/Error gotcha
    // that silently breaks `instanceof` checks if omitted.
    Object.setPrototypeOf(this, AIDosError.prototype);
  }

  toProblemDetails(): { title: string; status: number; detail: string } {
    return {
      title: this.code,
      status: this.httpStatus,
      detail: this.message,
    };
  }
}

export class NotFoundError extends AIDosError {
  constructor(message: string, cause?: unknown) {
    super("not_found", message, cause);
    this.name = "NotFoundError";
    Object.setPrototypeOf(this, NotFoundError.prototype);
  }
}

export class ValidationError extends AIDosError {
  constructor(message: string, cause?: unknown) {
    super("validation_error", message, cause);
    this.name = "ValidationError";
    Object.setPrototypeOf(this, ValidationError.prototype);
  }
}

export class UnauthorizedError extends AIDosError {
  constructor(message: string, cause?: unknown) {
    super("unauthorized", message, cause);
    this.name = "UnauthorizedError";
    Object.setPrototypeOf(this, UnauthorizedError.prototype);
  }
}

export class ForbiddenError extends AIDosError {
  constructor(message: string, cause?: unknown) {
    super("forbidden", message, cause);
    this.name = "ForbiddenError";
    Object.setPrototypeOf(this, ForbiddenError.prototype);
  }
}

export class ConflictError extends AIDosError {
  constructor(message: string, cause?: unknown) {
    super("conflict", message, cause);
    this.name = "ConflictError";
    Object.setPrototypeOf(this, ConflictError.prototype);
  }
}

export class InternalError extends AIDosError {
  constructor(message: string, cause?: unknown) {
    super("internal_error", message, cause);
    this.name = "InternalError";
    Object.setPrototypeOf(this, InternalError.prototype);
  }
}
