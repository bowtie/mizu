export interface SonarrImage {
  url?: string;
  remoteUrl?: string;
}

export function resolveSonarrImageUrl(image: SonarrImage | null | undefined, sonarrBaseUrl: string = ''): string {
  if (!image) return '';
  if (image.remoteUrl) return image.remoteUrl;

  const candidate = image.url || '';
  if (!candidate) return '';
  if (/^https?:/i.test(candidate)) return candidate;
  if (!sonarrBaseUrl) return '';

  try {
    return new URL(candidate, sonarrBaseUrl).toString();
  } catch {
    return '';
  }
}
