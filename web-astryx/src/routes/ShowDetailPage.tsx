import { useEffect, useState, useCallback, useMemo, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Section } from "@astryxdesign/core/Section";
import { Grid } from "@astryxdesign/core/Grid";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Badge } from "@astryxdesign/core/Badge";
import { Spinner } from "@astryxdesign/core/Spinner";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { Icon } from "@astryxdesign/core/Icon";
import { AlertDialog } from "@astryxdesign/core/AlertDialog";
import { Tooltip } from "@astryxdesign/core/Tooltip";
import { Timestamp } from "@astryxdesign/core/Timestamp";
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
} from "lucide-react";
import { useAppStore } from "../lib/store";
import {
  deleteShow,
  getArtworkBannerUrl,
  getArtworkProviders,
  getArtworkPosterUrl,
  getEpisodes,
  getShow,
  refreshArtwork,
  setArtworkSource,
  setEpisodeMonitor,
  updateShow,
  errorMessage,
} from "../lib/api";
import type {
  ArtworkProvider,
  Episode,
  SelectOption,
  Show,
  ShowUpdate,
  SonarrSeries,
} from "../lib/types";
import { resolveSonarrImageUrl } from "../lib/sonarr-images";
import { episodeTypeBadge } from "../lib/fox";
import { TrackingRuleCard } from "../components/TrackingRuleCard";

const WATERMARK_AUTO = "auto";
const WATERMARK_MANUAL = "manual";

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
function episodeMonitored(ep: Episode): boolean {
  return !!resolveSonarrEpisode(ep)?.monitored;
}

function badgeVariantForType(
  type: string | undefined | null,
): "neutral" | "blue" | "purple" | "yellow" | "red" {
  switch (type) {
    case "canon":
      return "blue";
    case "anime_canon":
      return "purple";
    case "mixed_canon":
    case "mixed":
      return "yellow";
    case "filler":
      return "red";
    default:
      return "neutral";
  }
}

/** Composite syncing key — avoids collision when sonarr_episode_id is 0/null. */
function epSyncKey(groupSeason: number, ep: Episode): string {
  return `${groupSeason}-${displayEpisodeNumber(ep)}-${ep.sonarr_episode_id ?? "x"}`;
}

export function ShowDetailPage() {
  const { id: showId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { shows, settings, setSettings, addNotification, setShows } = useAppStore();

  const [loadingShow, setLoadingShow] = useState(true);
  const [saving, setSaving] = useState(false);
  const [savedFlash, setSavedFlash] = useState(false);
  const savedFlashRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [show, setShow] = useState<Show | null>(null);
  const [series, setSeries] = useState<SonarrSeries | null>(null);
  const [artworkProviders, setArtworkProviders] = useState<ArtworkProvider[]>([]);
  const [artworkRefreshing, setArtworkRefreshing] = useState(false);
  const [artworkVersion, setArtworkVersion] = useState(0);
  const [episodes, setEpisodes] = useState<Episode[]>([]);
  const [episodesLoading, setEpisodesLoading] = useState(true);
  const [pageSettled, setPageSettled] = useState(false);
  const [trackFiller, setTrackFiller] = useState(false);
  const [trackMixed, setTrackMixed] = useState(true);
  const [trackAnimeCanon, setTrackAnimeCanon] = useState(true);
  const [watermarkAuto, setWatermarkAuto] = useState(true);
  const [markerValue, setMarkerValue] = useState(0);
  const [activeTypeFilter, setActiveTypeFilter] = useState<string>("all");
  const [isMobile, setIsMobile] = useState(false);
  const [syncingEps, setSyncingEps] = useState<Record<string, boolean>>({});

  const [pristineFiller, setPristineFiller] = useState(false);
  const [pristineMixed, setPristineMixed] = useState(true);
  const [pristineAnimeCanon, setPristineAnimeCanon] = useState(true);
  const [pristineWatermarkAuto, setPristineWatermarkAuto] = useState(true);
  const [pristineMarkerValue, setPristineMarkerValue] = useState(0);

  const rulesDirty =
    !!show &&
    (trackFiller !== pristineFiller ||
      trackMixed !== pristineMixed ||
      trackAnimeCanon !== pristineAnimeCanon ||
      watermarkAuto !== pristineWatermarkAuto ||
      (!watermarkAuto && markerValue !== pristineMarkerValue));

  const filteredEpisodes = useMemo(
    () =>
      episodes.filter((ep) => {
        if (activeTypeFilter === "all") return true;
        if (activeTypeFilter === "canon" && isMobile) {
          return ep.type === "canon" || ep.type === "anime_canon";
        }
        return ep.type === activeTypeFilter;
      }),
    [episodes, activeTypeFilter, isMobile],
  );

  const groupedBySeason = useMemo(() => {
    const groups: Record<string, Episode[]> = {};
    for (const ep of filteredEpisodes) {
      const season = String(episodeSeason(ep));
      if (!groups[season]) groups[season] = [];
      groups[season].push(ep);
    }
    return Object.entries(groups)
      .map(([seasonKey, eps], index) => ({
        season: Number.parseInt(seasonKey, 10),
        episodes: eps.slice().sort((a, b) => displayEpisodeNumber(b) - displayEpisodeNumber(a)),
        index,
      }))
      .sort((a, b) => {
        if (a.season === -1) return 1;
        if (b.season === -1) return -1;
        return b.season - a.season;
      });
  }, [filteredEpisodes]);

  const epStats = useMemo(() => {
    let total = 0,
      canon = 0,
      anime = 0,
      mixed = 0,
      filler = 0;
    for (const e of episodes) {
      total++;
      switch (e.type) {
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
  }, [episodes]);

  /** Highest episode number across all loaded episodes — computed once, reused everywhere. */
  const maxEpisode = useMemo(() => {
    if (!episodes.length) return 0;
    return episodes.reduce((highest, ep) => Math.max(highest, displayEpisodeNumber(ep)), 0);
  }, [episodes]);

  const clampMarkerValue = useCallback(
    (next: number): number => {
      const normalized = Math.max(0, Number(next) || 0);
      return Math.min(normalized, maxEpisode);
    },
    [maxEpisode],
  );

  /** Memoized backdrop URL — locally downloaded artwork first, then Sonarr images.
   *  Local artwork is always preferred when files exist (they were downloaded
   *  from the configured provider). Sonarr images are only a fallback. */
  const backdropUrl = useMemo((): string => {
    const cacheBust = artworkVersion > 0 ? `?v=${artworkVersion}` : "";
    // 1. Locally downloaded artwork — always prefer when files exist
    if (show?.artwork_banner_path) return getArtworkBannerUrl(showId!) + cacheBust;
    if (show?.artwork_poster_path) return getArtworkPosterUrl(showId!) + cacheBust;
    // 2. Sonarr images (fallback when no local artwork)
    if (!series?.images?.length) return "";
    const preferred =
      series.images.find((img) => img.coverType === "fanart") ||
      series.images.find((img) => img.coverType === "banner") ||
      series.images[0];
    if (!preferred) return "";
    return resolveSonarrImageUrl(preferred, settings?.sonarr_url || "");
  }, [show, showId, series, settings?.sonarr_url, artworkVersion]);

  // Fetch show data + episodes in parallel — they're independent and
  // both keyed off showId from the URL. Settings is no longer needed
  // here since we removed the Sonarr series fallback fetch.
  useEffect(() => {
    if (!showId) return;
    setLoadingShow(true);
    setEpisodesLoading(true);
    setPageSettled(false);

    Promise.all([getShow(showId), getEpisodes(showId)] as const)
      .then(([showRes, episodesRes]) => {
        if (!showRes) {
          addNotification("Anime not found", "error");
          navigate("/shows");
          return;
        }

        setShow(showRes);
        setSeries(null);
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

        setEpisodes((episodesRes as Episode[]) || []);
        setMarkerValue(clampMarkerValue(showRes.sync_watermark || 0));
        setPristineMarkerValue(clampMarkerValue(showRes.sync_watermark || 0));
      })
      .catch((err: unknown) => {
        addNotification("Failed to load anime: " + errorMessage(err), "error");
      })
      .finally(() => {
        setLoadingShow(false);
        setEpisodesLoading(false);
        // Defer enabling the hover-reveal collapse until React has committed
        // the episode list to the DOM and the browser has painted it.
        requestAnimationFrame(() => {
          setTimeout(() => setPageSettled(true), 80);
        });
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [showId]);

  // Lightweight reload functions for post-action refreshes (artwork, save, etc.)
  const loadShowData = useCallback(async (): Promise<Show | null> => {
    if (!showId) return null;
    try {
      const showRes = await getShow(showId);
      if (!showRes) return null;
      setShow(showRes);
      setSeries(null);
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
        setEpisodes((episodesRes as Episode[]) || []);
        setMarkerValue(clampMarkerValue(currentShow.sync_watermark || 0));
        setPristineMarkerValue(clampMarkerValue(currentShow.sync_watermark || 0));
      } catch (err: unknown) {
        addNotification("Failed to load episodes: " + errorMessage(err), "error");
      }
      setEpisodesLoading(false);
    },
    [showId, addNotification, clampMarkerValue],
  );

  // Load artwork providers list once
  useEffect(() => {
    getArtworkProviders()
      .then((res) => setArtworkProviders(res.providers || []))
      .catch((err: unknown) => addNotification("Failed to load artwork providers: " + errorMessage(err), "error"));
  }, [addNotification]);

  /** Build Selector options for the artwork source dropdown. */
  const artworkSourceOptions = useMemo<SelectOption[]>(() => {
    const opts: SelectOption[] = [{ value: "", label: "Use global default" }, { value: "sonarr", label: "Sonarr (local)" }];
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
          setArtworkVersion((v) => v + 1);
        }
      } else {
        // Source is "sonarr" or provider returned nothing — reload to
        // pick up any cleared paths, and inform the user.
        await loadShowData();
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

      if (source === "" ) {
        await loadShowData();
        setArtworkVersion((v) => v + 1);
        addNotification("Using global default artwork source", "info");
      } else if (source === "sonarr") {
        // Reload show to reflect cleared local paths, then bust cache.
        await loadShowData();
        setArtworkVersion((v) => v + 1);
        addNotification("Switched to Sonarr images", "info");
      } else {
        // Fetch artwork from the selected provider, then reload once
        // to pick up the new paths — no need to reload before the fetch.
        const result = await refreshArtwork(showId, source);
        const refreshed = await loadShowData();
        if (refreshed) {
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
      await deleteShow(showId);
      setShows(shows.filter((s) => s.id !== Number(showId)));
      addNotification(`Removed ${show?.sonarr_title || "anime"} from Mizu`, "success");
      setShowDeleteDialog(false);
      navigate("/shows");
    } catch (err: unknown) {
      addNotification("Failed to remove anime: " + errorMessage(err), "error");
    }
    setDeleting(false);
  };

  const toggleEpisodeMonitor = async (groupSeason: number, ep: Episode) => {
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
      setEpisodes((prev) =>
        prev.map((e) => (e.sonarr_episode_id === sonarrEp.id ? { ...e, monitored: nextState } : e)),
      );
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
  };

  if (loadingShow) {
    return (
      <Section padding={4} maxWidth={900} className="section-centered">
        <VStack gap={3} hAlign="center" padding={6}>
          <Spinner size="lg" label="Loading anime" />
        </VStack>
      </Section>
    );
  }

  if (!show) return null;

  return (
    <>
      <Section
        variant="transparent"
        padding={0}
        maxWidth={900}
        className="section-centered show-hero-section"
      >
        <div className={`show-shell${pageSettled ? " is-settled" : ""}`}>
          {/* Hero header with backdrop */}
          <div className="show-hero">
            {backdropUrl ? (
              <>
                <img
                  src={backdropUrl}
                  alt=""
                  loading="eager"
                  decoding="async"
                  fetchPriority="high"
                  className="show-hero-img"
                  onLoad={(e) => e.currentTarget.classList.add("is-loaded")}
                />
                <div className="show-hero-gradient" />
              </>
            ) : (
              <div className="show-hero-placeholder" />
            )}
            <div className="show-hero-content">
              <VStack gap={4}>
                <VStack gap={1} className="show-hero-titles">
                  <HStack gap={3} align="center" wrap="wrap">
                    <Heading level={1}>
                      {show.sonarr_title}
                    </Heading>
                    {!episodesLoading && (
                      <Badge variant="gray" label={`${episodes.length} episodes`} />
                    )}
                  </HStack>
                  {show.overview && (
                    <Text
                      type="supporting"
                      color="secondary"
                      display="block"
                      className="show-hero-overview"
                    >
                      {show.overview}
                    </Text>
                  )}
                </VStack>
              </VStack>
            </div>
          </div>

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
                      />
                    </InputGroupText>
                  </InputGroup>
                  <Tooltip
                    content={watermarkAuto ? "Using Sonarr progress" : "Using manual episode"}
                    placement="end"
                  >
                    <Button
                      label={watermarkAuto ? "Auto" : "Manual"}
                      variant="secondary"
                      size="md"
                      icon={<Bot size={16} />}
                      isIconOnly={isMobile}
                      onClick={() => setWatermarkAuto(!watermarkAuto)}
                    />
                  </Tooltip>
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
                  />
                  <Button
                    label={savedFlash ? "Saved" : saving ? "Saving..." : "Save"}
                    variant="primary"
                    size="md"
                    icon={savedFlash ? <Check size={14} /> : <Save size={14} />}
                    isIconOnly={isMobile}
                    onClick={handleSave}
                    isDisabled={saving || !rulesDirty}
                    isLoading={saving}
                    className={savedFlash ? "save-flash" : undefined}
                  />
                </HStack>
              </div>
            </div>
          </div>
        </div>
      </Section>

      {/* Page content — constrained width */}
      <Section variant="transparent" padding={0} maxWidth={900} className="section-centered page-enter">
        <VStack gap={6} className="show-content-padded">
          {/* Artwork controls */}
          <Card padding={4} variant="muted" className="artwork-card">
            <VStack gap={3}>
              <HStack gap={3} align="center">
                <ImageIcon size={16} color="var(--color-icon-secondary)" />
                <Heading level={3} className="settings-section-header">Artwork</Heading>
                <Divider xstyle={{ flex: 1 }} />
                <Button
                  label={artworkRefreshing ? "Refreshing..." : "Refresh"}
                  variant="ghost"
                  size="sm"
                  icon={<RefreshCw size={14} className={artworkRefreshing ? "spin" : ""} />}
                  onClick={() => handleRefreshArtwork()}
                  isDisabled={artworkRefreshing}
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
              <Heading level={2} className="settings-section-header">Tracking rules</Heading>
              <Divider xstyle={{ flex: 1 }} />
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

          {/* Episode list */}
          <VStack gap={4}>
            <HStack gap={3} align="center">
              <Heading level={2} className="settings-section-header">Episode list</Heading>
              <Divider xstyle={{ flex: 1 }} />
            </HStack>

            {/* Episode filters */}
            <div className="episode-filters-desktop">
              <SegmentedControl
                value={activeTypeFilter}
                onChange={setActiveTypeFilter}
                label="Episode type filter"
                layout="fill"
                size="lg"
              >
                <SegmentedControlItem value="all" label={`All (${epStats.total})`} />
                <SegmentedControlItem
                  value="canon"
                  label={`Manga (${epStats.canon})`}
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
              </SegmentedControl>
            </div>
            <div className="episode-filters-mobile">
              <SegmentedControl
                value={activeTypeFilter}
                onChange={setActiveTypeFilter}
                label="Episode type filter"
                layout="fill"
                size="lg"
              >
                <SegmentedControlItem value="all" label={`All (${epStats.total})`} />
                <SegmentedControlItem
                  value="canon"
                  label={`Manga (${epStats.canon})`}
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
              </SegmentedControl>
            </div>

            {/* Episodes by season */}
            {episodesLoading ? (
              <VStack gap={3} hAlign="center" padding={6}>
                <Spinner size="md" label="Loading episodes" />
              </VStack>
            ) : filteredEpisodes.length === 0 ? (
              <EmptyState
                title="No episodes"
                description="No episodes match the selected type."
                icon={<Icon icon={CircleAlert} size="lg" color="secondary" />}
                actions={
                  <Button
                    label="Show all"
                    variant="secondary"
                    onClick={() => setActiveTypeFilter("all")}
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
                          <span className="season-episode-count">
                            {group.episodes.length} episodes
                          </span>
                        </>
                      }
                      value={String(group.season)}
                      defaultIsOpen={group.index === groupedBySeason.length - 1}
                    >
                      <div className="episode-list-wrap">
                        <List density="balanced" hasDividers>
                          {group.episodes.map((ep) => {
                            const typeBadge = episodeTypeBadge(ep.type);
                            const monitored = episodeMonitored(ep);
                            const key = epSyncKey(group.season, ep);
                            const isSyncing = syncingEps[key];
                            return (
                              <ListItem
                                key={`${group.season}-${displayEpisodeNumber(ep)}`}
                                label={ep.title || "—"}
                                description={`Episode ${displayEpisodeNumber(ep)}`}
                                endContent={
                                  <HStack gap={4} align="center">
                                    {ep.air_date && (
                                      <Timestamp value={ep.air_date} format="date" />
                                    )}
                                    <Badge
                                      variant={badgeVariantForType(ep.type)}
                                      label={typeBadge.label}
                                    />
                                    <Tooltip content={monitored ? "Unmonitor" : "Monitor"} placement="start">
                                      <ToggleButton
                                        label={monitored ? "Unmonitor episode" : "Monitor episode"}
                                        isIconOnly
                                        size="sm"
                                        icon={<Eye size={14} />}
                                        pressedIcon={<EyeOff size={14} />}
                                        isPressed={monitored}
                                        isDisabled={isSyncing || !ep.sonarr_episode_id}
                                        isLoading={isSyncing}
                                        onPressedChange={() => toggleEpisodeMonitor(group.season, ep)}
                                      />
                                    </Tooltip>
                                  </HStack>
                                }
                              />
                            );
                          })}
                        </List>
                      </div>
                    </Collapsible>
                  </Card>
                ))}
              </VStack>
            )}
          </VStack>
        </VStack>
      </Section>

      <AlertDialog
        isOpen={showDeleteDialog}
        onOpenChange={setShowDeleteDialog}
        title="Remove anime?"
        description={`This will remove "${show.sonarr_title}" from Mizu. The series stays in Sonarr. You can re-add it anytime.`}
        actionLabel="Remove"
        isActionLoading={deleting}
        onAction={handleDelete}
      />
    </>
  );
}
