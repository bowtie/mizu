/** The in-app path to return to after signing in. Only same-origin paths
 *  ("/settings", "/shows/3?x=1") are accepted; anything else — absolute
 *  URLs, protocol-relative "//host", backslash tricks — falls back to "/". */
export function safeRedirectPath(from: string | null): string {
  if (!from || !from.startsWith("/") || from.startsWith("//") || from.includes("\\")) {
    return "/";
  }
  return from;
}

const RETURN_PATH_KEY = "mizu-return-path";

/** Remembers where to go after signing in, so /login needs no query string.
 *  Kept in sessionStorage; a blocked or private-mode storage just means the
 *  library is the landing page. */
export function rememberReturnPath(path: string): void {
  try {
    sessionStorage.setItem(RETURN_PATH_KEY, path);
  } catch {
    // Storage unavailable: fall back to the library.
  }
}

/** The remembered return path (validated by safeRedirectPath), cleared once read. */
export function takeReturnPath(): string {
  let path: string | null = null;
  try {
    path = sessionStorage.getItem(RETURN_PATH_KEY);
    sessionStorage.removeItem(RETURN_PATH_KEY);
  } catch {
    // Storage unavailable: fall back to the library.
  }
  return safeRedirectPath(path);
}
