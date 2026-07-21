/**
 * buildInfo exists to prove this package's build/lint/test pipeline
 * actually works end to end — it is infrastructure, not a dashboard
 * feature. The dashboard's actual views (health tiles, benchmark
 * trends) are roadmap epic 15's scope, not Project Foundation's.
 */
export interface BuildInfo {
  readonly packageName: string;
  readonly builtAt: string;
}

export function getBuildInfo(now: () => Date = () => new Date()): BuildInfo {
  return {
    packageName: "@ai-dos/dashboard",
    builtAt: now().toISOString(),
  };
}
