/**
 * Cursor-based pagination — matches the convention specs/api/*.md
 * already establishes (cursor, not offset, since offset pagination is
 * unstable under concurrent writes at this system's scale).
 */
export interface PaginationParams {
  readonly cursor?: string;
  readonly limit?: number;
}

export interface PaginatedResult<T> {
  readonly items: readonly T[];
  readonly nextCursor: string | null;
  readonly hasMore: boolean;
}

export const DEFAULT_PAGE_LIMIT = 20;
export const MAX_PAGE_LIMIT = 100;

/**
 * clampLimit enforces the documented pagination bounds (default 20,
 * max 100) in one place, so every service applies the same bound
 * instead of each hand-rolling its own clamp — and a caller who never
 * validated their own input still can't request an unbounded page.
 */
export function clampLimit(requested: number | undefined): number {
  if (requested === undefined || requested <= 0) {
    return DEFAULT_PAGE_LIMIT;
  }
  return Math.min(requested, MAX_PAGE_LIMIT);
}
