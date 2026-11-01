import type { AuthStatus } from "./api";

/** True when sign-in is handled outside Mizu (MIZU_AUTH=none|proxy). */
export function isAuthManaged(status: Pick<AuthStatus, "mode"> | undefined): boolean {
  return status?.mode === "none" || status?.mode === "proxy";
}
