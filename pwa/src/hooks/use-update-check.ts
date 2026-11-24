// Auto-update checks are disabled in this fork (no external GitHub API calls).
export const RETRY_GAP = 10 * 60 * 1000

const STABLE_TAG = /^v\d+\.\d+\.\d+$/

export interface LatestRelease {
  version: string
  url: string
  checkedAt: number
}

interface Release {
  tag_name: string
  html_url: string
  draft?: boolean
  prerelease?: boolean
}

function parse(v: string): { core: number[]; pre: boolean } {
  const [core, pre] = v.replace(/^v/, '').split('-', 2)
  return {
    core: core.split('.').map((n) => Number(n) || 0),
    pre: pre !== undefined,
  }
}

// -1 if a < b, 0 if equal, 1 if a > b. A pre-release comes before its
// version (1.2.0-rc.1 < 1.2.0); two pre-releases of one version are equal.
export function compareVersions(a: string, b: string): number {
  const pa = parse(a)
  const pb = parse(b)
  for (let i = 0; i < 3; i++) {
    const d = (pa.core[i] ?? 0) - (pb.core[i] ?? 0)
    if (d !== 0) return d < 0 ? -1 : 1
  }
  if (pa.pre !== pb.pre) return pa.pre ? -1 : 1
  return 0
}

export function newestStable(releases: Release[]): Release | null {
  let best: Release | null = null
  for (const r of releases) {
    if (r.draft || r.prerelease || !STABLE_TAG.test(r.tag_name)) continue
    if (compareVersions(r.tag_name, '1.0.0') < 0) continue
    if (!best || compareVersions(r.tag_name, best.tag_name) > 0) best = r
  }
  return best
}

export function availableUpdate(
  _running: string,
  _latest: LatestRelease | null,
): string | null {
  return null
}

export function resetUpdateCheck() {}

const noopCheck = async (_force = false): Promise<void> => {}

export function useUpdateCheck() {
  return {
    latest: null as LatestRelease | null,
    checking: false,
    failed: false,
    check: noopCheck,
  }
}
