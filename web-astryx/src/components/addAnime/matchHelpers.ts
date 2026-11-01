import type { AFLMatch } from "../../lib/types";

export function isUnsafeMatch(match: AFLMatch | null): boolean {
  return !!match && match.confidence == null;
}
export function isCompactMatchSelection(match: AFLMatch | null): boolean {
  if (!match || isUnsafeMatch(match)) return false;
  return (
    match.method === "exact" || match.method === "normalized" || (match.confidence ?? 0) >= 0.95
  );
}
export function shouldAutoCreateMappingAfterSonarrAdd(match: AFLMatch | null): boolean {
  return (
    !!match && !isUnsafeMatch(match) && match.method === "exact" && (match.confidence ?? 0) >= 1
  );
}
export function matchTitle(match: AFLMatch | null | undefined): string {
  return match?.afl_show?.title || match?.title || "";
}
export function matchSlug(match: AFLMatch | null | undefined): string {
  return match?.afl_show?.slug || match?.slug || "";
}
