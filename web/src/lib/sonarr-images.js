export function resolveSonarrImageUrl(image, sonarrBaseUrl = "") {
  if (!image) return "";
  if (image.remoteUrl) return image.remoteUrl;

  const candidate = image.url || "";
  if (!candidate) return "";
  if (/^https?:/i.test(candidate)) return candidate;
  if (!sonarrBaseUrl) return "";

  try {
    return new URL(candidate, sonarrBaseUrl).toString();
  } catch {
    return "";
  }
}
