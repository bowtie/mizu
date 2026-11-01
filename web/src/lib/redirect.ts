/** The in-app path to return to after signing in. Only same-origin paths
 *  ("/settings", "/shows/3?x=1") are accepted; anything else — absolute
 *  URLs, protocol-relative "//host", backslash tricks — falls back to "/". */
export function safeRedirectPath(from: string | null): string {
  if (!from || !from.startsWith("/") || from.startsWith("//") || from.includes("\\")) {
    return "/";
  }
  return from;
}
