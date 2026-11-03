/** Show page URLs use a slug of the Sonarr title ("/shows/cardcaptor-sakura")
 *  instead of Mizu's numeric id. Old "/shows/67" links still resolve and are
 *  replaced with the slug. */

interface Sluggable {
  id: number | string;
  sonarr_title: string;
}

/** Lowercase ASCII words joined by dashes; accents are dropped
 *  ("Pokémon" → "pokemon"). */
export function slugify(title: string): string {
  return title
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

/** Each show's slug. A title that slugs to nothing, or to the same slug as
 *  another show, gets its id appended so every slug stays unique. */
export function showSlugs(shows: readonly Sluggable[]): Map<string, string> {
  const counts = new Map<string, number>();
  for (const s of shows) {
    const base = slugify(s.sonarr_title);
    counts.set(base, (counts.get(base) ?? 0) + 1);
  }
  const slugs = new Map<string, string>();
  for (const s of shows) {
    const base = slugify(s.sonarr_title);
    slugs.set(String(s.id), base && counts.get(base) === 1 ? base : `${base || "show"}-${s.id}`);
  }
  return slugs;
}

/** The URL path of a show's page, falling back to its id when it isn't in
 *  the list yet (just added). */
export function showPath(id: number | string, shows: readonly Sluggable[] | undefined): string {
  const slug = shows ? showSlugs(shows).get(String(id)) : undefined;
  return `/shows/${slug ?? id}`;
}

/** Resolves the ":ref" part of a show URL to the show's id: a number is an id
 *  as is, anything else is looked up among the shows' slugs. Undefined while
 *  the list is unknown or when nothing matches. */
export function resolveShowRef(
  ref: string | undefined,
  shows: readonly Sluggable[] | undefined,
): string | undefined {
  if (!ref) return undefined;
  if (/^\d+$/.test(ref)) return ref;
  if (!shows) return undefined;
  for (const [id, slug] of showSlugs(shows)) {
    if (slug === ref) return id;
  }
  return undefined;
}
