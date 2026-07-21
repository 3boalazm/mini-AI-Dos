/**
 * Result<T, E> makes an expected failure path visible in a function's
 * type signature instead of hidden behind a throw a caller has to know
 * to catch. Reserved for expected failures (validation, not-found) —
 * unexpected/programmer errors should still throw and be caught at a
 * boundary, not be forced through this type everywhere.
 */
export type Result<T, E = Error> = Ok<T> | Err<E>;

export interface Ok<T> {
  readonly ok: true;
  readonly value: T;
}

export interface Err<E> {
  readonly ok: false;
  readonly error: E;
}

export function ok<T>(value: T): Ok<T> {
  return { ok: true, value };
}

export function err<E>(error: E): Err<E> {
  return { ok: false, error };
}

export function isOk<T, E>(result: Result<T, E>): result is Ok<T> {
  return result.ok;
}

export function isErr<T, E>(result: Result<T, E>): result is Err<E> {
  return !result.ok;
}

/**
 * unwrapOr returns the success value, or fallback if result is an
 * error — the common case of "give me the value or a safe default"
 * without a caller writing the same if/else every time.
 */
export function unwrapOr<T, E>(result: Result<T, E>, fallback: T): T {
  return isOk(result) ? result.value : fallback;
}

/**
 * map transforms the success value, passing an error through
 * unchanged — standard functional Result composition.
 */
export function map<T, U, E>(result: Result<T, E>, fn: (value: T) => U): Result<U, E> {
  return isOk(result) ? ok(fn(result.value)) : result;
}
