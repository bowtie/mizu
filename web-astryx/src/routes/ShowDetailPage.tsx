import {
  memo,
  useEffect,
  useState,
  useCallback,
  useMemo,
  useRef,
  useDeferredValue,
  lazy,
  Suspense,
} from "react";
import { useParams, useNavigate } from "react-router-dom";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Section } from "@astryxdesign/core/Section";
import { Grid } from "@astryxdesign/core/Grid";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Badge } from "@astryxdesign/core/Badge";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { Icon } from "@astryxdesign/core/Icon";
const AlertDialog = lazy(() =>
  import("@astryxdesign/core/AlertDialog").then((m) => ({ default: m.AlertDialog })),
);
import { Card } from "@astryxdesign/core/Card";
import { ToggleButton } from "@astryxdesign/core/ToggleButton";
import { NumberInput } from "@astryxdesign/core/NumberInput";
import { InputGroup, InputGroupText } from "@astryxdesign/core/InputGroup";
import { Collapsible } from "@astryxdesign/core/Collapsible";
import { List, ListItem } from "@astryxdesign/core/List";
import { Selector } from "@astryxdesign/core/Selector";
import { Divider } from "@astryxdesign/core/Divider";
import { SegmentedControl, SegmentedControlItem } from "@astryxdesign/core/SegmentedControl";
import {
  Trash2,
  Save,
  Check,
  CircleAlert,
  Bot,
  Eye,
  EyeOff,
  BookOpen,
  Tv,
  Shuffle,
  SkipForward,
  Minus,
  Plus,
  RefreshCw,
  Image as ImageIcon,
  LoaderCircle,
} from "lucide-react";
import { useNotificationStore } from "../lib/store";
import {
  useShows,
  useArtworkProviders,
  useDeleteShow,
  useSonarrSeries,
  useSettings,
} from "../lib/queries";
import { VirtualizedEpisodeList } from "../components/VirtualizedEpisodeList";
import {
  getArtworkBannerUrl,
  getCachedShowDetail,
  clearCachedShowDetail,
  getEpisodes,
  getShow,
  refreshArtwork,
  setArtworkSource,
  selectArtwork,
  setEpisodeMonitor,
  updateShow,
  errorMessage,
} from "../lib/api";
import type { Episode, SelectOption, Show, ShowUpdate, SonarrSeries } from "../lib/types";
import { resolveSonarrImageUrl } from "../lib/sonarr-images";
import { TrackingRuleCard } from "../components/TrackingRuleCard";
import {
  ArtworkPickerDialog,
  type ArtworkPickerSource,
  type ArtworkSlot,
} from "../components/ArtworkPickerDialog";

const WATERMARK_AUTO = "auto";
const WATERMARK_MANUAL = "manual";

// Providers that expose many candidate images (the artwork picker).
// Sonarr/anilist/kitsu have a single best pick and no candidates endpoint.
const PICKER_SOURCES = new Set(["fanart", "tmdb"]);

const actionToTracked = (action: string | undefined, fallback: boolean = true): boolean =>
  (action || (fallback ? "monitor" : "unmonitor")) === "monitor";
const trackedToAction = (tracked: boolean): string => (tracked ? "monitor" : "unmonitor");

function isSpecialSeasonNumber(num: number | undefined): boolean {
  return num === -1 || num === 0;
}
function seasonLabel(num: number | undefined): string {
  return isSpecialSeasonNumber(num) ? "Specials" : `Season ${num}`;
}
function displayEpisodeNumber(ep: Episode): number {
  if (isSpecialSeasonNumber(ep.season_number)) return ep.sonarr_episode_number ?? 0;
  return ep.absolute_episode_number ?? ep.episode_num ?? ep.sonarr_episode_number ?? 0;
}
function episodeLabel(ep: Episode): string {
  return isSpecialSeasonNumber(ep.season_number)
    ? `Special ${displayEpisodeNumber(ep)}`
    : `Episode #${displayEpisodeNumber(ep)}`;
}
function episodeSeason(ep: Episode): number {
  const raw = ep.season_number;
  // Only treat valid positive integers as real seasons.
  // null/undefined/0/-1 all go to the "Specials" bucket (-1).
  if (typeof raw !== "number" || !Number.isInteger(raw) || raw <= 0) return -1;
  return raw;
}
function resolveSonarrEpisode(ep: Episode) {
  if (!ep.sonarr_episode_id) return null;
  return { id: ep.sonarr_episode_id, monitored: !!ep.monitored };
}

/** Combined badge info for an episode type — avoids two separate
 *  lookups (episodeTypeBadge + badgeVariantForType) per item. */
function episodeBadgeInfo(type: string | undefined | null): {
  label: string;
  variant: "neutral" | "blue" | "purple" | "yellow" | "red";
} {
  switch (type) {
    case "canon":
      return { label: "Manga", variant: "blue" };
    case "anime_canon":
      return { label: "Anime", variant: "purple" };
    case "mixed_canon":
    case "mixed":
      return { label: "Mixed", variant: "yellow" };
    case "filler":
      return { label: "Filler", variant: "red" };
    case "special":
      return { label: "Special", variant: "neutral" };
    default:
      return { label: "TBD", variant: "neutral" };
  }
}

/** Composite syncing key — avoids collision when sonarr_episode_id is 0/null. */
function epSyncKey(groupSeason: number, ep: Episode): string {
  return `${groupSeason}-${displayEpisodeNumber(ep)}-${ep.sonarr_episode_id ?? "x"}`;
}

interface SeasonGroup {
  season: number;
  episodes: Episode[];
  index: number;
}

interface EpisodeListSectionProps {
  episodesLoading: boolean;
  hasShowData: boolean;
  groupedBySeason: SeasonGroup[];
  epStats: { total: number; canon: number; anime: number; mixed: number; filler: number };
  activeTypeFilter: string;
  syncingEps: Record<string, boolean>;
  onFilterChange: (value: string) => void;
  onToggleMonitor: (groupSeason: number, ep: Episode) => void;
  isMobile: boolean;
}

/** Single episode row — memoized so that toggling one episode's monitor
 *  state doesn't re-render all 1200+ ListItems. Only re-renders when
 *  the episode data or its sync state changes. */
const EpisodeListItem = memo(function EpisodeListItem({
  ep,
  groupSeason,
  isSyncing,
  onToggleMonitor,
}: {
  ep: Episode;
  groupSeason: number;
  isSyncing: boolean;
  onToggleMonitor: (groupSeason: number, ep: Episode) => void;
}) {
  const badge = episodeBadgeInfo(ep.type);
  const monitored = !!ep.sonarr_episode_id && !!ep.monitored;
  const epNum = displayEpisodeNumber(ep);
  return (
    <ListItem
      key={`${groupSeason}-${epNum}`}
      label={ep.title || "—"}
      description={`Episode ${epNum}`}
      endContent={
        <HStack gap={4} align="center">
          {ep.air_date && (
            <Text type="supporting" size="sm" color="secondary">
              {new Date(ep.air_date).toLocaleDateString(undefined, {
                year: "numeric",
                month: "short",
                day: "numeric",
              })}
            </Text>
          )}
          <Badge variant={badge.variant} label={badge.label} />
          <ToggleButton
            label={monitored ? "Unmonitor episode" : "Monitor episode"}
            isIconOnly
            size="sm"
            icon={<Eye size={14} />}
            pressedIcon={<EyeOff size={14} />}
            isPressed={monitored}
            isDisabled={isSyncing || !ep.sonarr_episode_id}
            isLoading={isSyncing}
            onPressedChange={() => onToggleMonitor(groupSeason, ep)}
            aria-label={monitored ? "Unmonitor" : "Monitor"}
          />
        </HStack>
      }
    />
  );
});

/** Episode list section — memoized so tracking-rule toggles don't re-render
 *  the (potentially 1200+) episode list. Only re-renders when episode data,
 *  filter, or syncing state actually changes. */
const EpisodeListSection = memo(function EpisodeListSection({
  episodesLoading,
  hasShowData,
  groupedBySeason,
  epStats,
  activeTypeFilter,
  syncingEps,
  onFilterChange,
  onToggleMonitor,
  isMobile,
}: EpisodeListSectionProps) {
  // Short-circuit: if no episodes are syncing, skip the epSyncKey
  // computation for every item (83+ per expanded season). syncingEps
  // is only non-empty during a brief monitor toggle.
  const hasSyncing = Object.keys(syncingEps).length > 0;

  // Track which seasons have been expanded at least once — collapsed
  // seasons that were never opened skip rendering their (potentially
  // 1000+) episode ListItems entirely, saving DOM nodes.
  // Initialize with the default-open season (last one) so it renders
  // its episodes on the first paint — no extra useEffect round-trip.
  const [openedSeasons, setOpenedSeasons] = useState<Set<string>>(() => {
    if (groupedBySeason.length === 0) return new Set();
    const last = groupedBySeason[groupedBySeason.length - 1];
    return new Set([String(last.season)]);
  });

  // When groupedBySeason changes (filter switch), auto-open the new
  // last season and clean up stale seasons that no longer exist.
  useEffect(() => {
    if (groupedBySeason.length === 0) {
      setOpenedSeasons(new Set());
      return;
    }
    const validSeasons = new Set(groupedBySeason.map((g) => String(g.season)));
    const lastKey = String(groupedBySeason[groupedBySeason.length - 1].season);
    setOpenedSeasons((prev) => {
      const next = new Set<string>();
      for (const key of prev) {
        if (validSeasons.has(key)) next.add(key);
      }
      next.add(lastKey);
      return next;
    });
  }, [groupedBySeason]);

  const filterItems = (
    <>
      <SegmentedControlItem value="all" label={`All (${epStats.total})`} />
      <SegmentedControlItem
        value="canon"
        label={`Manga (${isMobile ? epStats.canon + epStats.anime : epStats.canon})`}
        icon={<BookOpen size={16} />}
      />
      <SegmentedControlItem
        value="anime_canon"
        label={`Anime (${epStats.anime})`}
        icon={<Tv size={16} />}
      />
      <SegmentedControlItem
        value="mixed_canon"
        label={`Mixed (${epStats.mixed})`}
        icon={<Shuffle size={16} />}
      />
      <SegmentedControlItem
        value="filler"
        label={`Filler (${epStats.filler})`}
        icon={<SkipForward size={16} />}
      />
    </>
  );

  return (
    <VStack gap={4}>
      <HStack gap={3} align="center">
        <Heading level={2}>Episode list</Heading>
        <div style={{ flex: 1 }} />
      </HStack>

      {/* Episode filters */}
      <SegmentedControl
        value={activeTypeFilter}
        onChange={onFilterChange}
        label="Episode type filter"
        layout="fill"
        size="lg"
      >
        {filterItems}
      </SegmentedControl>

      {/* Episodes by season — keep skeleton visible while useDeferredValue
          catches up to avoid a flash of "No episodes" empty state. */}
      {episodesLoading || (epStats.total > 0 && groupedBySeason.length === 0) ? (
        hasShowData ? (
          /* Minimal loader — show data is already rendered, just waiting
             for episodes to arrive from prefetch/network. */
          <EmptyState
            title="Loading episodes…"
            description="Fetching episode data from Sonarr."
            icon={<Icon icon={LoaderCircle} size="lg" color="secondary" className="spin" />}
          />
        ) : (
          <VStack gap={4}>
            {/* Skeleton filter bar */}
            <div style={{ display: "flex", gap: "var(--spacing-2)" }}>
              <div
                className="skeleton"
                style={{ width: 70, height: 36, borderRadius: "var(--radius-inner)" }}
              />
              <div
                className="skeleton"
                style={{ width: 80, height: 36, borderRadius: "var(--radius-inner)" }}
              />
              <div
                className="skeleton"
                style={{ width: 75, height: 36, borderRadius: "var(--radius-inner)" }}
              />
              <div
                className="skeleton"
                style={{ width: 70, height: 36, borderRadius: "var(--radius-inner)" }}
              />
              <div
                className="skeleton"
                style={{ width: 65, height: 36, borderRadius: "var(--radius-inner)" }}
              />
            </div>
            {/* Skeleton season cards — show 2, not 4, to reduce visual noise */}
            {Array.from({ length: 2 }).map((_, i) => (
              <div key={i} className="skeleton-season-card">
                <div className="skeleton" style={{ width: 20, height: 20, borderRadius: 4 }} />
                <div className="skeleton-text" style={{ width: 90 }} />
                <div className="skeleton" style={{ width: 80, height: 20, borderRadius: 10 }} />
                <div style={{ flex: 1 }} />
                <div className="skeleton" style={{ width: 24, height: 24, borderRadius: 4 }} />
              </div>
            ))}
          </VStack>
        )
      ) : groupedBySeason.length === 0 ? (
        <EmptyState
          title="No episodes"
          description="No episodes match the selected type."
          icon={<Icon icon={CircleAlert} size="lg" color="secondary" />}
          actions={
            <Button
              label="Show all"
              variant="secondary"
              onClick={() => onFilterChange("all")}
              data-cuelume-press
            />
          }
        />
      ) : (
        <VStack gap={4}>
          {groupedBySeason.map((group) => (
            <Card key={group.season} padding={4} variant="muted" className="season-card">
              <Collapsible
                trigger={
                  <>
                    <span>{seasonLabel(group.season)}</span>
                    <Badge variant="neutral" label={`${group.episodes.length} episodes`} />
                  </>
                }
                value={String(group.season)}
                defaultIsOpen={group.index === groupedBySeason.length - 1}
                onOpenChange={(isOpen) => {
                  if (isOpen) {
                    setOpenedSeasons((prev) => {
                      if (prev.has(String(group.season))) return prev;
                      const next = new Set(prev);
                      next.add(String(group.season));
                      return next;
                    });
                  }
                }}
              >
                {openedSeasons.has(String(group.season)) ||
                group.index === groupedBySeason.length - 1 ? (
                  group.episodes.length > 100 ? (
                    <VirtualizedEpisodeList
                      episodes={group.episodes}
                      groupSeason={group.season}
                      syncingEps={syncingEps}
                      hasSyncing={hasSyncing}
                      onToggleMonitor={onToggleMonitor}
                    />
                  ) : (
                    <div className="episode-list-wrap">
                      <List density="balanced" hasDividers>
                        {group.episodes.map((ep) => (
                          <EpisodeListItem
                            key={`${group.season}-${displayEpisodeNumber(ep)}-${ep.sonarr_episode_id}`}
                            ep={ep}
                            groupSeason={group.season}
                            isSyncing={hasSyncing && !!syncingEps[epSyncKey(group.season, ep)]}
                            onToggleMonitor={onToggleMonitor}
                          />
                        ))}
                      </List>
                    </div>
                  )
                ) : null}
              </Collapsible>
            </Card>
          ))}
        </VStack>
      )}
    </VStack>
  );
});

export function ShowDetailPage() {
  const { id: showId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const addNotification = useNotificationStore((s) => s.addNotification);
  const showsQuery = useShows();
  const shows = showsQuery.data ?? [];
  const { data: artworkProviders = [] } = useArtworkProviders();
  const deleteShowMutation = useDeleteShow();
  const { data: sonarrSeries = [] } = useSonarrSeries();
  const { data: settings } = useSettings();

  // If the store already has this show (fetched on app mount), use it as
  // the initial state so the page renders instantly without waiting for
  // getShow to resolve. The effect below still fetches fresh data to pick
  // up any changes, but the user sees content immediately.
  const storeShow = useMemo(
    () => shows.find((s) => s.id === Number(showId)) ?? null,
    [shows, showId],
  );

  // Check if the prefetched detail data (fired in main.tsx on hard
  // refresh) has already resolved. If so, use it as initial state to
  // skip the skeleton entirely — the user sees content on first paint.
  const prefetchedDetail = showId ? getCachedShowDetail(showId) : null;
  const initialShow = storeShow ?? prefetchedDetail?.show ?? null;
  const initialEpisodes = prefetchedDetail?.episodes as Episode[] | undefined;

  const [loadingShow, setLoadingShow] = useState(!initialShow);
  const [saving, setSaving] = useState(false);
  const [savedFlash, setSavedFlash] = useState(false);
  const savedFlashRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [show, setShow] = useState<Show | null>(initialShow);
  const [artworkRefreshing, setArtworkRefreshing] = useState(false);
  const [artworkVersion, setArtworkVersion] = useState(0);
  const [heroImgError, setHeroImgError] = useState(false);
  const [heroFallbackFailed, setHeroFallbackFailed] = useState(false);
  const [episodes, setEpisodes] = useState<Episode[]>(initialEpisodes ?? []);
  const [episodesLoading, setEpisodesLoading] = useState(!initialEpisodes);
  const [pageSettled, setPageSettled] = useState(!!initialShow);
  const [epStats, setEpStats] = useState(() => {
    if (initialEpisodes) {
      let total = 0,
        canon = 0,
        anime = 0,
        mixed = 0,
        filler = 0,
        max = 0;
      for (const ep of initialEpisodes) {
        max = Math.max(max, displayEpisodeNumber(ep));
        total++;
        switch (ep.type) {
          case "canon":
            canon++;
            break;
          case "anime_canon":
            anime++;
            break;
          case "mixed_canon":
            mixed++;
            break;
          case "filler":
            filler++;
            break;
        }
      }
      return { total, canon, anime, mixed, filler };
    }
    return { total: 0, canon: 0, anime: 0, mixed: 0, filler: 0 };
  });
  const [maxEpisode, setMaxEpisode] = useState(() => {
    if (!initialEpisodes) return 0;
    let max = 0;
    for (const ep of initialEpisodes) max = Math.max(max, displayEpisodeNumber(ep));
    return max;
  });
  const [trackFiller, setTrackFiller] = useState(() =>
    actionToTracked(initialShow?.filler_action, false),
  );
  const [trackMixed, setTrackMixed] = useState(() =>
    actionToTracked(initialShow?.mixed_action, true),
  );
  const [trackAnimeCanon, setTrackAnimeCanon] = useState(() =>
    actionToTracked(initialShow?.anime_canon_action, true),
  );
  const [watermarkAuto, setWatermarkAuto] = useState(
    () => (initialShow?.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO,
  );
  const [markerValue, setMarkerValue] = useState(() =>
    Math.max(0, initialShow?.sync_watermark || 0),
  );
  const [activeTypeFilter, setActiveTypeFilter] = useState<string>("all");
  const [isMobile, setIsMobile] = useState(false);
  const [syncingEps, setSyncingEps] = useState<Record<string, boolean>>({});

  const [pristineFiller, setPristineFiller] = useState(() =>
    actionToTracked(initialShow?.filler_action, false),
  );
  const [pristineMixed, setPristineMixed] = useState(() =>
    actionToTracked(initialShow?.mixed_action, true),
  );
  const [pristineAnimeCanon, setPristineAnimeCanon] = useState(() =>
    actionToTracked(initialShow?.anime_canon_action, true),
  );
  const [pristineWatermarkAuto, setPristineWatermarkAuto] = useState(
    () => (initialShow?.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO,
  );
  const [pristineMarkerValue, setPristineMarkerValue] = useState(() =>
    Math.max(0, initialShow?.sync_watermark || 0),
  );

  const rulesDirty =
    !!show &&
    (trackFiller !== pristineFiller ||
      trackMixed !== pristineMixed ||
      trackAnimeCanon !== pristineAnimeCanon ||
      watermarkAuto !== pristineWatermarkAuto ||
      (!watermarkAuto && markerValue !== pristineMarkerValue));

  const filteredEpisodes = useMemo(() => {
    // Fast path: "all" filter returns the original array reference,
    // avoiding a 1209-element .filter() allocation on every render.
    if (activeTypeFilter === "all") return episodes;
    return episodes.filter((ep) => {
      if (activeTypeFilter === "canon" && isMobile) {
        return ep.type === "canon" || ep.type === "anime_canon";
      }
      return ep.type === activeTypeFilter;
    });
  }, [episodes, activeTypeFilter, isMobile]);

  // Defer filtered episode rendering on large lists so filter changes
  // don't block the SegmentedControl interaction. React renders the
  // urgent update (filter highlight) first, then commits the episode
  // list in a separate, lower-priority pass.
  const deferredFilteredEpisodes = useDeferredValue(filteredEpisodes);

  const groupedBySeason = useMemo(() => {
    const groups: Record<string, Episode[]> = {};
    for (const ep of deferredFilteredEpisodes) {
      const season = String(episodeSeason(ep));
      if (!groups[season]) groups[season] = [];
      groups[season].push(ep);
    }
    // Episodes come from the DB sorted by episode_num ASC. Within each
    // season, reverse to show newest first — avoids a full sort since
    // the input is already ordered.
    return Object.entries(groups)
      .map(([seasonKey, eps], index) => ({
        season: Number.parseInt(seasonKey, 10),
        episodes: eps.reverse(),
        index,
      }))
      .sort((a, b) => {
        if (a.season === -1) return 1;
        if (b.season === -1) return -1;
        return b.season - a.season;
      });
  }, [deferredFilteredEpisodes]);

  const clampMarkerValue = useCallback(
    (next: number): number => {
      const normalized = Math.max(0, Number(next) || 0);
      return Math.min(normalized, maxEpisode);
    },
    [maxEpisode],
  );

  /** Memoized backdrop URL — hits the artwork endpoint immediately
   *  using just the showId from the URL. No need to wait for getShow
   *  or store data — the backend serves the file if it exists.
   *  Falls back to Sonarr's banner/fanart URL when local artwork is
   *  missing (e.g. artwork not yet downloaded). */
  const backdropUrl = useMemo((): string => {
    const cacheBust = artworkVersion > 0 ? `?v=${artworkVersion}` : "";
    if (showId) {
      const localUrl = getArtworkBannerUrl(showId) + cacheBust;
      // If local artwork errored, fall back to Sonarr's image
      if (heroImgError && show) {
        const series = sonarrSeries.find((s: SonarrSeries) => s.id === show.sonarr_series_id);
        if (series?.images?.length) {
          const banner =
            series.images.find((img) => img.coverType === "fanart") ||
            series.images.find((img) => img.coverType === "banner") ||
            series.images[0];
          if (banner) {
            return resolveSonarrImageUrl(banner, settings?.sonarr_url || "");
          }
        }
      }
      return localUrl;
    }
    return "";
  }, [showId, artworkVersion, heroImgError, show, sonarrSeries, settings?.sonarr_url]);

  // Fetch show data + episodes. If the store already has show data,
  // we skip the getShow call entirely — the store data is fresh enough
  // (fetched on app mount). Episodes always need fetching since they're
  // not in the store. On hard refresh (no store data), we fire two
  // parallel requests: getShow (simple DB query, resolves fast) and
  // getEpisodes (may need Sonarr + AFL enrichment, resolves slower).
  // Show data is applied as soon as getShow resolves — the hero title,
  // tracking rules, and artwork card appear immediately. Episodes fill
  // in when getEpisodes resolves.
  // If the prefetched detail data was already resolved (fired in
  // main.tsx before React mounted), we skip all fetching.
  useEffect(() => {
    if (!showId) return;
    const hasStoreData = !!storeShow;
    const hasPrefetched = !!prefetchedDetail;

    // Clear stale state from a previous show so the old show's data
    // doesn't linger while the new show's data loads.
    setEpisodes([]);
    setEpStats({ total: 0, canon: 0, anime: 0, mixed: 0, filler: 0 });
    setMaxEpisode(0);

    // Cancellation flag — prevents stale promises from a previous showId
    // from overwriting state after navigation.
    let cancelled = false;
    const rafIds: number[] = [];
    const scheduleSettled = () => {
      const id = requestAnimationFrame(() => {
        if (!cancelled) setPageSettled(true);
      });
      rafIds.push(id);
    };

    // If we already have all data from prefetch, just settle — no fetch needed.
    if (hasPrefetched) {
      scheduleSettled();
      // Clear the cache so subsequent getShow/getEpisodes calls (e.g.
      // loadShowData after a save) fetch fresh data instead of returning
      // the stale prefetch.
      clearCachedShowDetail(showId);
      return () => {
        cancelled = true;
        rafIds.forEach(cancelAnimationFrame);
      };
    }

    // Note: clearCachedShowDetail is called AFTER getShow/getEpisodes
    // below, so they can still reuse the in-flight prefetch promise.
    // The cache is only needed for the initial mount — clearing it
    // after the fetches are fired ensures subsequent calls (e.g.
    // loadShowData after a save) fetch fresh data.

    setLoadingShow(!hasStoreData);
    setEpisodesLoading(true);
    setHeroImgError(false);
    setHeroFallbackFailed(false);

    // If the store already has show data, settle immediately — the
    // CSS controls-reveal keeps the bar hidden until hover anyway,
    // so we just need to enable the transition on next frame.
    if (hasStoreData) {
      scheduleSettled();
    } else {
      setPageSettled(false);
    }

    const applyShowData = (showRes: Show) => {
      if (cancelled) return;
      setShow(showRes);
      setTrackFiller(actionToTracked(showRes.filler_action, false));
      setTrackMixed(actionToTracked(showRes.mixed_action, true));
      setTrackAnimeCanon(actionToTracked(showRes.anime_canon_action, true));
      setWatermarkAuto((showRes.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO);
      setMarkerValue(Math.max(0, showRes.sync_watermark || 0));
      setPristineFiller(actionToTracked(showRes.filler_action, false));
      setPristineMixed(actionToTracked(showRes.mixed_action, true));
      setPristineAnimeCanon(actionToTracked(showRes.anime_canon_action, true));
      setPristineWatermarkAuto((showRes.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO);
      setPristineMarkerValue(Math.max(0, showRes.sync_watermark || 0));
      scheduleSettled();
    };

    const applyEpisodes = (eps: Episode[]) => {
      if (cancelled) return;
      setEpisodes(eps);
      let freshMax = 0;
      let total = 0,
        canon = 0,
        anime = 0,
        mixed = 0,
        filler = 0;
      for (const ep of eps) {
        freshMax = Math.max(freshMax, displayEpisodeNumber(ep));
        total++;
        switch (ep.type) {
          case "canon":
            canon++;
            break;
          case "anime_canon":
            anime++;
            break;
          case "mixed_canon":
            mixed++;
            break;
          case "filler":
            filler++;
            break;
        }
      }
      setEpStats({ total, canon, anime, mixed, filler });
      setMaxEpisode(freshMax);
      setMarkerValue((prev) => Math.min(prev, freshMax));
      setPristineMarkerValue((prev) => Math.min(prev, freshMax));
      setEpisodesLoading(false);
    };

    if (!hasStoreData) {
      // Hard refresh — fire show-only and episodes requests in parallel.
      // getShow is a simple DB query and resolves much faster than
      // getEpisodes (which may need to enrich from Sonarr + AFL). Show
      // data is applied as soon as it arrives so the hero title,
      // tracking rules, and artwork card appear immediately.
      // Episodes fill in when getEpisodes resolves — the episodes
      // endpoint has ETag support so repeat visits get 304 Not Modified.
      let showApplied = false;
      getShow(showId)
        .then((showRes) => {
          if (cancelled) return;
          showApplied = true;
          applyShowData(showRes);
        })
        .catch((err: unknown) => {
          if (!cancelled && !showApplied) {
            addNotification("Failed to load anime: " + errorMessage(err), "error");
          }
        })
        .finally(() => {
          if (!cancelled) setLoadingShow(false);
        });

      getEpisodes(showId)
        .then((episodesRes) => {
          if (cancelled) return;
          applyEpisodes((episodesRes as Episode[]) || []);
        })
        .catch((err: unknown) => {
          if (!cancelled) {
            addNotification("Failed to load episodes: " + errorMessage(err), "error");
            setEpisodesLoading(false);
          }
        });
    } else {
      // Store has show data — only fetch episodes.
      getEpisodes(showId)
        .then((episodesRes) => {
          if (cancelled) return;
          applyEpisodes((episodesRes as Episode[]) || []);
        })
        .catch((err: unknown) => {
          if (!cancelled) {
            addNotification("Failed to load episodes: " + errorMessage(err), "error");
            setEpisodesLoading(false);
          }
        });
    }

    // Clear the prefetch caches after getShow/getEpisodes have been fired.
    // This ensures subsequent calls (e.g. loadShowData after a save) fetch
    // fresh data instead of returning stale prefetch data.
    clearCachedShowDetail(showId);

    return () => {
      cancelled = true;
      rafIds.forEach(cancelAnimationFrame);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [showId]);

  // If the store didn't have show data on first render (e.g. hard refresh
  // where getShows() hasn't resolved yet), adopt it as soon as it arrives
  // so the page content appears without waiting for getShow to finish.
  useEffect(() => {
    if (storeShow && show?.id !== storeShow.id) {
      setShow(storeShow);
      setLoadingShow(false);
      setTrackFiller(actionToTracked(storeShow.filler_action, false));
      setTrackMixed(actionToTracked(storeShow.mixed_action, true));
      setTrackAnimeCanon(actionToTracked(storeShow.anime_canon_action, true));
      setWatermarkAuto((storeShow.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO);
      setMarkerValue(Math.max(0, storeShow.sync_watermark || 0));
      setPristineFiller(actionToTracked(storeShow.filler_action, false));
      setPristineMixed(actionToTracked(storeShow.mixed_action, true));
      setPristineAnimeCanon(actionToTracked(storeShow.anime_canon_action, true));
      setPristineWatermarkAuto((storeShow.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO);
      setPristineMarkerValue(Math.max(0, storeShow.sync_watermark || 0));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [storeShow]);

  // Lightweight reload functions for post-action refreshes (artwork, save, etc.)
  const loadShowData = useCallback(async (): Promise<Show | null> => {
    if (!showId) return null;
    try {
      const showRes = await getShow(showId);
      if (!showRes) return null;
      setShow(showRes);
      setTrackFiller(actionToTracked(showRes.filler_action, false));
      setTrackMixed(actionToTracked(showRes.mixed_action, true));
      setTrackAnimeCanon(actionToTracked(showRes.anime_canon_action, true));
      setWatermarkAuto((showRes.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO);
      setMarkerValue(Math.max(0, showRes.sync_watermark || 0));
      setPristineFiller(actionToTracked(showRes.filler_action, false));
      setPristineMixed(actionToTracked(showRes.mixed_action, true));
      setPristineAnimeCanon(actionToTracked(showRes.anime_canon_action, true));
      setPristineWatermarkAuto((showRes.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO);
      setPristineMarkerValue(Math.max(0, showRes.sync_watermark || 0));
      return showRes;
    } catch (err: unknown) {
      addNotification("Failed to load anime: " + errorMessage(err), "error");
      return null;
    }
  }, [showId, addNotification]);

  const loadEpisodeData = useCallback(
    async (currentShow: Show) => {
      if (!showId) return;
      setEpisodesLoading(true);
      try {
        const episodesRes = await getEpisodes(showId);
        const eps = (episodesRes as Episode[]) || [];
        setEpisodes(eps);
        // Recompute epStats in the same pass.
        let total = 0,
          canon = 0,
          anime = 0,
          mixed = 0,
          filler = 0;
        let freshMax = 0;
        for (const ep of eps) {
          freshMax = Math.max(freshMax, displayEpisodeNumber(ep));
          total++;
          switch (ep.type) {
            case "canon":
              canon++;
              break;
            case "anime_canon":
              anime++;
              break;
            case "mixed_canon":
              mixed++;
              break;
            case "filler":
              filler++;
              break;
          }
        }
        setEpStats({ total, canon, anime, mixed, filler });
        setMaxEpisode(freshMax);
        setMarkerValue(clampMarkerValue(currentShow.sync_watermark || 0));
        setPristineMarkerValue(clampMarkerValue(currentShow.sync_watermark || 0));
      } catch (err: unknown) {
        addNotification("Failed to load episodes: " + errorMessage(err), "error");
      }
      setEpisodesLoading(false);
    },
    [showId, addNotification, clampMarkerValue],
  );

  /** Build Selector options for the artwork source dropdown. */
  const artworkSourceOptions = useMemo<SelectOption[]>(() => {
    const opts: SelectOption[] = [
      { value: "", label: "Use global default" },
      { value: "sonarr", label: "Sonarr (local)" },
    ];
    for (const p of artworkProviders) {
      const suffix = p.needs_key && !p.available ? " (needs API key)" : "";
      opts.push({ value: p.id, label: `${p.label}${suffix}` });
    }
    return opts;
  }, [artworkProviders]);

  useEffect(() => {
    const updateViewport = () => setIsMobile(window.innerWidth <= 600);
    updateViewport();
    window.addEventListener("resize", updateViewport);
    return () => window.removeEventListener("resize", updateViewport);
  }, []);

  useEffect(() => {
    if (isMobile && activeTypeFilter === "anime_canon") {
      setActiveTypeFilter("canon");
    }
  }, [isMobile, activeTypeFilter]);

  // Clean up savedFlash timeout on unmount
  useEffect(() => {
    return () => {
      if (savedFlashRef.current) clearTimeout(savedFlashRef.current);
    };
  }, []);

  // Effective artwork source for the picker: the per-show override if set,
  // otherwise the global default. The picker only appears for sources that
  // expose candidate images.
  const resolvedArtworkSource =
    show?.artwork_source || settings?.artwork_default_source || "sonarr";
  const canPickArtwork = PICKER_SOURCES.has(resolvedArtworkSource);
  // Candidate-capable sources to tab between in the picker dialog.
  const pickerSources = useMemo<ArtworkPickerSource[]>(
    () =>
      artworkProviders
        .filter((p) => PICKER_SOURCES.has(p.id))
        .map((p) => ({ id: p.id, label: p.label })),
    [artworkProviders],
  );

  const handleArtworkPick = async (source: string, slot: ArtworkSlot, url: string) => {
    if (!showId) return;
    setArtworkRefreshing(true);
    try {
      // Partial pick: only the chosen slot is sent; the backend preserves
      // the other slots' existing files and paths.
      await selectArtwork(showId, {
        source,
        poster_url: slot === "poster" ? url : "",
        banner_url: slot === "banner" ? url : "",
        logo_url: slot === "logo" ? url : "",
      });
      const refreshed = await loadShowData();
      if (refreshed) {
        setHeroImgError(false);
        setHeroFallbackFailed(false);
        setArtworkVersion((v) => v + 1);
      }
      setPickerOpen(false);
      addNotification(`Artwork ${slot} updated`, "success");
    } catch (err: unknown) {
      addNotification("Failed to save artwork: " + errorMessage(err), "error");
    }
    setArtworkRefreshing(false);
  };

  const handleRefreshArtwork = async (source?: string) => {
    if (!showId) return;
    setArtworkRefreshing(true);
    try {
      const result = await refreshArtwork(showId, source);
      if (result.found) {
        addNotification(`Artwork fetched from ${result.source}`, "success");
        // Reload show to get updated artwork paths, then bust browser cache
        const refreshed = await loadShowData();
        if (refreshed) {
          setHeroImgError(false);
          setHeroFallbackFailed(false);
          setArtworkVersion((v) => v + 1);
        }
      } else {
        // Source is "sonarr" or provider returned nothing — reload to
        // pick up any cleared paths, and inform the user.
        await loadShowData();
        setHeroImgError(false);
        setHeroFallbackFailed(false);
        setArtworkVersion((v) => v + 1);
        addNotification(
          source === "sonarr"
            ? "Using Sonarr images (no download needed)"
            : "No artwork found from this source",
          "warning",
        );
      }
    } catch (err: unknown) {
      addNotification("Failed to fetch artwork: " + errorMessage(err), "error");
    }
    setArtworkRefreshing(false);
  };

  const handleArtworkSourceChange = async (source: string) => {
    if (!showId) return;
    setArtworkRefreshing(true);
    try {
      await setArtworkSource(showId, source);

      if (source === "") {
        await loadShowData();
        setHeroImgError(false);
        setHeroFallbackFailed(false);
        setArtworkVersion((v) => v + 1);
        addNotification("Using global default artwork source", "info");
      } else if (source === "sonarr") {
        // Reload show to reflect cleared local paths, then bust cache.
        await loadShowData();
        setHeroImgError(false);
        setHeroFallbackFailed(false);
        setArtworkVersion((v) => v + 1);
        addNotification("Switched to Sonarr images", "info");
      } else {
        // Fetch artwork from the selected provider, then reload once
        // to pick up the new paths — no need to reload before the fetch.
        const result = await refreshArtwork(showId, source);
        const refreshed = await loadShowData();
        if (refreshed) {
          setHeroImgError(false);
          setHeroFallbackFailed(false);
          setArtworkVersion((v) => v + 1);
        }
        if (result.found) {
          addNotification(`Switched to ${result.source} artwork`, "success");
        } else {
          addNotification(`No artwork found from ${source}`, "warning");
        }
      }
    } catch (err: unknown) {
      addNotification("Failed to set artwork source: " + errorMessage(err), "error");
    }
    setArtworkRefreshing(false);
  };

  const handleSave = async () => {
    if (!showId) return;
    setSaving(true);
    try {
      const payload: ShowUpdate = {
        filler_action: trackedToAction(trackFiller),
        mixed_action: trackedToAction(trackMixed),
        anime_canon_action: trackedToAction(trackAnimeCanon),
        watermark_mode: watermarkAuto ? WATERMARK_AUTO : WATERMARK_MANUAL,
      };
      if (!watermarkAuto) {
        payload.sync_watermark = clampMarkerValue(markerValue);
      }
      await updateShow(showId, payload);
      addNotification(`Saved tracking rules for ${show?.sonarr_title || "anime"}`, "success");
      setSavedFlash(true);
      if (savedFlashRef.current) clearTimeout(savedFlashRef.current);
      savedFlashRef.current = setTimeout(() => setSavedFlash(false), 1500);
      const refreshedShow = await loadShowData();
      if (refreshedShow) {
        await loadEpisodeData(refreshedShow);
      }
    } catch (err: unknown) {
      addNotification("Failed to save anime: " + errorMessage(err), "error");
    }
    setSaving(false);
  };

  const handleDelete = async () => {
    if (!showId) return;
    setDeleting(true);
    try {
      await deleteShowMutation.mutateAsync(showId);
      addNotification(`Removed ${show?.sonarr_title || "anime"} from Mizu`, "success");
      setShowDeleteDialog(false);
      navigate("/shows");
    } catch (err: unknown) {
      addNotification("Failed to remove anime: " + errorMessage(err), "error");
    }
    setDeleting(false);
  };

  const toggleEpisodeMonitor = useCallback(
    async (groupSeason: number, ep: Episode) => {
      const sonarrEp = resolveSonarrEpisode(ep);
      if (!sonarrEp?.id || !showId) {
        addNotification("No Sonarr episode found for " + episodeLabel(ep), "error");
        return;
      }
      const key = epSyncKey(groupSeason, ep);
      setSyncingEps((prev) => ({ ...prev, [key]: true }));
      try {
        const nextState = !sonarrEp.monitored;
        await setEpisodeMonitor(showId, sonarrEp.id, nextState);
        setEpisodes((prev) => {
          // Find the episode by sonarr_episode_id and update in-place
          // via splice — avoids mapping all 1200+ elements when only
          // one changed.
          const idx = prev.findIndex((e) => e.sonarr_episode_id === sonarrEp.id);
          if (idx === -1) return prev;
          const next = prev.slice();
          next[idx] = { ...prev[idx], monitored: nextState };
          return next;
        });
        addNotification(
          `${episodeLabel(ep)} ${nextState ? "now monitored" : "now unmonitored"}`,
          "success",
        );
      } catch (err: unknown) {
        addNotification("Failed to update episode: " + errorMessage(err), "error");
      }
      setSyncingEps((prev) => {
        const next = { ...prev };
        delete next[key];
        return next;
      });
    },
    [showId, addNotification],
  );

  return (
    <>
      <Section
        variant="transparent"
        padding={4}
        maxWidth={900}
        className="section-centered page-enter"
      >
        <VStack gap={6}>
          <div className={`show-shell${pageSettled ? " is-settled" : ""}`}>
            {/* Hero header with backdrop — renders immediately using showId
              from the URL. The image starts loading before getShow() resolves. */}
            <div className="show-hero">
              {backdropUrl && !heroFallbackFailed ? (
                <>
                  <img
                    key={`${showId}-${artworkVersion}-${heroImgError ? "fallback" : "local"}`}
                    src={backdropUrl}
                    alt=""
                    loading="eager"
                    decoding="async"
                    fetchPriority="high"
                    className="show-hero-img"
                    onError={() => {
                      if (heroImgError) {
                        setHeroFallbackFailed(true);
                      } else {
                        setHeroImgError(true);
                      }
                    }}
                  />
                </>
              ) : (
                <div className="show-hero-placeholder" />
              )}
              <div className="show-hero-content">
                <VStack gap={1} className="show-hero-titles" style={{ padding: 0, margin: 0 }}>
                  <HStack
                    gap={3}
                    align="center"
                    wrap="wrap"
                    style={{ padding: 0, margin: 0, alignItems: "center" }}
                  >
                    {show ? (
                      <Heading level={1} className="show-hero-title">
                        {show.sonarr_title}
                      </Heading>
                    ) : (
                      <div className="skeleton-text-lg" style={{ width: 220, height: "1.5em" }} />
                    )}
                  </HStack>
                  {show?.overview && (
                    <Text
                      type="supporting"
                      color="secondary"
                      display="block"
                      className="show-hero-overview"
                      style={{ margin: 0 }}
                    >
                      {show.overview}
                    </Text>
                  )}
                </VStack>
              </div>
            </div>

            {/* Actions bar — only render when show data is available */}
            {show && (
              <div className="controls-reveal">
                <div className="controls-reveal-inner">
                  <div className="show-actions-bar">
                    <HStack gap={3} align="center" wrap="nowrap">
                      <InputGroup
                        label="Episode"
                        isLabelHidden
                        size="md"
                        isDisabled={watermarkAuto}
                        className="show-actions-bar-center"
                      >
                        <InputGroupText className="episode-step-btn-wrap">
                          <Button
                            label="Decrease episode"
                            variant="ghost"
                            size="md"
                            isIconOnly
                            icon={<Minus size={16} />}
                            className="episode-step-btn"
                            isDisabled={watermarkAuto || markerValue <= 0}
                            onClick={() => setMarkerValue(clampMarkerValue(markerValue - 1))}
                            data-cuelume-press
                          />
                        </InputGroupText>
                        <NumberInput
                          label="Episode"
                          isLabelHidden
                          value={markerValue}
                          onChange={(value) => setMarkerValue(clampMarkerValue(value))}
                          min={0}
                          max={maxEpisode || null}
                          step={1}
                          isIntegerOnly
                          isDisabled={watermarkAuto}
                        />
                        <InputGroupText className="episode-step-btn-wrap">
                          <Button
                            label="Increase episode"
                            variant="ghost"
                            size="md"
                            isIconOnly
                            icon={<Plus size={16} />}
                            className="episode-step-btn"
                            isDisabled={watermarkAuto}
                            onClick={() => setMarkerValue(clampMarkerValue(markerValue + 1))}
                            data-cuelume-press
                          />
                        </InputGroupText>
                      </InputGroup>
                      <Button
                        label={watermarkAuto ? "Auto" : "Manual"}
                        variant="secondary"
                        size="md"
                        icon={<Bot size={16} />}
                        isIconOnly={isMobile}
                        onClick={() => setWatermarkAuto(!watermarkAuto)}
                        aria-label={
                          watermarkAuto ? "Using Sonarr progress" : "Using manual episode"
                        }
                        data-cuelume-toggle
                      />
                    </HStack>
                    <HStack gap={2} align="center" wrap="nowrap" className="show-actions-bar-end">
                      <Button
                        label="Remove"
                        variant="destructive"
                        size="md"
                        icon={<Trash2 size={16} />}
                        isIconOnly={isMobile}
                        onClick={() => setShowDeleteDialog(true)}
                        isDisabled={deleting}
                        data-cuelume-press
                      />
                      <Button
                        label={savedFlash ? "Saved" : saving ? "Saving..." : "Save"}
                        variant="primary"
                        size="md"
                        icon={
                          savedFlash ? (
                            <Check size={14} color="var(--color-success)" />
                          ) : (
                            <Save size={14} />
                          )
                        }
                        isIconOnly={isMobile}
                        onClick={handleSave}
                        isDisabled={saving || !rulesDirty}
                        isLoading={saving}
                        className={savedFlash ? "save-flash" : undefined}
                        data-cuelume-press
                      />
                    </HStack>
                  </div>
                </div>
              </div>
            )}
          </div>

          <Divider />

          {loadingShow || !show ? (
            <>
              {/* Skeleton: Artwork card */}
              <div className="skeleton-card" style={{ padding: "var(--spacing-4)" }}>
                <div style={{ display: "flex", flexDirection: "column", gap: "var(--spacing-3)" }}>
                  <div style={{ display: "flex", alignItems: "center", gap: "var(--spacing-3)" }}>
                    <div className="skeleton" style={{ width: 16, height: 16, borderRadius: 4 }} />
                    <div className="skeleton-text" style={{ width: 80 }} />
                    <div style={{ flex: 1 }} />
                    <div
                      className="skeleton"
                      style={{ width: 90, height: 28, borderRadius: "var(--radius-inner)" }}
                    />
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: "var(--spacing-3)" }}>
                    <div className="skeleton-text" style={{ width: 60 }} />
                    <div
                      className="skeleton"
                      style={{ width: 220, height: 32, borderRadius: "var(--radius-inner)" }}
                    />
                  </div>
                </div>
              </div>

              {/* Skeleton: Tracking rules */}
              <div style={{ display: "flex", flexDirection: "column", gap: "var(--spacing-4)" }}>
                <div style={{ display: "flex", alignItems: "center", gap: "var(--spacing-3)" }}>
                  <div className="skeleton-text-lg" style={{ width: 130 }} />
                  <div style={{ flex: 1 }} />
                </div>
                <div
                  style={{
                    display: "grid",
                    gridTemplateColumns: "repeat(auto-fill, minmax(240px, 1fr))",
                    gap: "var(--spacing-4)",
                  }}
                >
                  <div className="skeleton-rule-card">
                    <div className="skeleton-text" style={{ width: 60 }} />
                    <div className="skeleton-text-lg" style={{ width: 100 }} />
                    <div className="skeleton-text" style={{ width: "80%" }} />
                    <div style={{ flex: 1 }} />
                    <div className="skeleton" style={{ width: 44, height: 24, borderRadius: 12 }} />
                  </div>
                  <div className="skeleton-rule-card">
                    <div className="skeleton-text" style={{ width: 50 }} />
                    <div className="skeleton-text-lg" style={{ width: 90 }} />
                    <div className="skeleton-text" style={{ width: "70%" }} />
                    <div style={{ flex: 1 }} />
                    <div className="skeleton" style={{ width: 44, height: 24, borderRadius: 12 }} />
                  </div>
                  <div className="skeleton-rule-card">
                    <div className="skeleton-text" style={{ width: 45 }} />
                    <div className="skeleton-text-lg" style={{ width: 80 }} />
                    <div className="skeleton-text" style={{ width: "75%" }} />
                    <div style={{ flex: 1 }} />
                    <div className="skeleton" style={{ width: 44, height: 24, borderRadius: 12 }} />
                  </div>
                </div>
              </div>
            </>
          ) : (
            <>
              {/* Artwork controls */}
              <Card padding={4} variant="muted" className="artwork-card">
                <VStack gap={3}>
                  <HStack gap={3} align="center">
                    <ImageIcon size={16} color="var(--color-icon-secondary)" />
                    <Heading level={3}>Artwork</Heading>
                    <div style={{ flex: 1 }} />
                    {canPickArtwork && (
                      <Button
                        label="Choose"
                        variant="secondary"
                        size="sm"
                        icon={<ImageIcon size={14} />}
                        onClick={() => setPickerOpen(true)}
                        isDisabled={artworkRefreshing}
                        data-cuelume-press
                      />
                    )}
                    <Button
                      label={artworkRefreshing ? "Refreshing..." : "Refresh"}
                      variant="ghost"
                      size="sm"
                      icon={<RefreshCw size={14} className={artworkRefreshing ? "spin" : ""} />}
                      onClick={() => handleRefreshArtwork()}
                      isDisabled={artworkRefreshing}
                      data-cuelume-press
                    />
                  </HStack>
                  <HStack gap={3} align="center" wrap="wrap">
                    <Text type="supporting" color="secondary">
                      Source:
                    </Text>
                    <Selector
                      label="Artwork source"
                      value={show?.artwork_source || ""}
                      options={artworkSourceOptions}
                      onChange={(v) => handleArtworkSourceChange(v as string)}
                      isLabelHidden
                      width={220}
                    />
                    {show?.artwork_source && show.artwork_source !== "" && (
                      <Badge variant="info" label={`Using ${show.artwork_source}`} />
                    )}
                  </HStack>
                </VStack>
              </Card>

              {/* Tracking rules */}
              <VStack gap={4}>
                <HStack gap={3} align="center">
                  <Heading level={2}>Tracking rules</Heading>
                  <div style={{ flex: 1 }} />
                  {rulesDirty && <Badge variant="warning" label="Unsaved changes" />}
                </HStack>

                <Grid columns={{ minWidth: 240 }} gap={4} align="start">
                  <TrackingRuleCard
                    variant="green"
                    label="Canon"
                    description="Main plot episodes"
                    switchLabel="Track canon"
                    value={trackAnimeCanon}
                    onChange={setTrackAnimeCanon}
                  />
                  <TrackingRuleCard
                    variant="yellow"
                    label="Mixed"
                    description="Mix of canon and filler"
                    switchLabel="Track mixed"
                    value={trackMixed}
                    onChange={setTrackMixed}
                  />
                  <TrackingRuleCard
                    variant="red"
                    label="Filler"
                    description="Skippable, no plot impact"
                    switchLabel="Track filler"
                    value={trackFiller}
                    onChange={setTrackFiller}
                  />
                </Grid>
              </VStack>
            </>
          )}

          {/* Episode list — memoized so tracking-rule toggles don't re-render it.
            Rendered outside the show-gate so the skeleton appears immediately
            even before show data resolves (e.g. hard refresh). */}
          {showId && (
            <EpisodeListSection
              episodesLoading={episodesLoading}
              hasShowData={!!show}
              groupedBySeason={groupedBySeason}
              epStats={epStats}
              activeTypeFilter={activeTypeFilter}
              syncingEps={syncingEps}
              onFilterChange={setActiveTypeFilter}
              onToggleMonitor={toggleEpisodeMonitor}
              isMobile={isMobile}
            />
          )}
        </VStack>
      </Section>

      {show && showDeleteDialog && (
        <Suspense fallback={null}>
          <AlertDialog
            isOpen={showDeleteDialog}
            onOpenChange={setShowDeleteDialog}
            title="Remove anime?"
            description={`This will remove "${show.sonarr_title}" from Mizu. The series stays in Sonarr. You can re-add it anytime.`}
            actionLabel="Remove"
            isActionLoading={deleting}
            onAction={handleDelete}
          />
        </Suspense>
      )}

      {showId && (
        <ArtworkPickerDialog
          isOpen={pickerOpen}
          onOpenChange={setPickerOpen}
          showId={showId}
          sources={pickerSources}
          initialSource={resolvedArtworkSource}
          currentArtwork={{
            poster: show?.artwork_poster_url || undefined,
            banner: show?.artwork_banner_url || undefined,
            logo: show?.artwork_logo_url || undefined,
          }}
          onSelect={handleArtworkPick}
        />
      )}
    </>
  );
}
