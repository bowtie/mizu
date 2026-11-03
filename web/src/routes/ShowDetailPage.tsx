import {
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
import { useForm } from "react-hook-form";
import { useQueryClient } from "@tanstack/react-query";
import { VStack } from "@astryxdesign/core/VStack";
import { Section } from "@astryxdesign/core/Section";
const AlertDialog = lazy(() =>
  import("@astryxdesign/core/AlertDialog").then((m) => ({ default: m.AlertDialog })),
);
import { Divider } from "@astryxdesign/core/Divider";
import { useNotificationStore } from "../lib/store";
import {
  invalidateShow,
  useArtworkProviders,
  useDeleteShow,
  useEpisodes,
  useSetEpisodeMonitor,
  useShow,
  useSonarrSeries,
  useSettings,
  useUpdateShow,
} from "../lib/queries";
import {
  getArtworkBannerUrl,
  refreshArtwork,
  setArtworkSource,
  selectArtwork,
  uploadCustomArtwork,
  removeCustomArtwork,
  errorMessage,
} from "../lib/api";
import type { Episode, SelectOption, Show, ShowUpdate, SonarrSeries } from "../lib/types";
import { resolveSonarrImageUrl } from "../lib/sonarr-images";
import {
  ArtworkPickerDialog,
  type ArtworkPickerSource,
  type ArtworkSlot,
} from "../components/ArtworkPickerDialog";

import {
  actionToTracked,
  trackedToAction,
  WATERMARK_AUTO,
  WATERMARK_MANUAL,
  PICKER_SOURCES,
  MAX_CUSTOM_IMAGE_BYTES,
  displayEpisodeNumber,
  episodeLabel,
  episodeSeason,
  epSyncKey,
  resolveSonarrEpisode,
} from "./showDetailHelpers";
import { EpisodeListSection } from "../components/EpisodeListSection";
import { ShowHero } from "../components/ShowHero";
import { ShowActionsBar } from "../components/ShowActionsBar";
import { useIsMobile, useIsTouch } from "../lib/useIsMobile";
import { FillerFilesDialog, useFillerFiles } from "../components/FillerFilesDialog";
import { ShowDetailSkeleton } from "../components/ShowDetailSkeleton";
import { TrackingRulesSection } from "../components/TrackingRulesSection";

const NO_EPISODES: Episode[] = [];

/** A tracking toggle saves this long after it changes, so a quick double
 *  flip is one save. */
const TRACKING_SAVE_DELAY_MS = 150;

/** The show's tracking rules as edited on this page. */
interface RulesForm {
  trackFiller: boolean;
  trackMixed: boolean;
  trackAnimeCanon: boolean;
  watermarkAuto: boolean;
  markerValue: number;
}

function rulesFromShow(show: Show, maxEpisode: number): RulesForm {
  const marker = Math.max(0, show.sync_watermark || 0);
  return {
    trackFiller: actionToTracked(show.filler_action, false),
    trackMixed: actionToTracked(show.mixed_action, true),
    trackAnimeCanon: actionToTracked(show.anime_canon_action, true),
    watermarkAuto: (show.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO,
    // Until episodes load the ceiling is unknown, so don't clamp yet.
    markerValue: maxEpisode > 0 ? Math.min(marker, maxEpisode) : marker,
  };
}

/** Per-type counts for the filter tabs, plus the highest episode number
 *  (the ceiling for the manual sync marker). */
function episodeStats(episodes: Episode[]) {
  const stats = { total: 0, canon: 0, anime: 0, mixed: 0, filler: 0 };
  let maxEpisode = 0;
  for (const ep of episodes) {
    maxEpisode = Math.max(maxEpisode, displayEpisodeNumber(ep));
    stats.total++;
    switch (ep.type) {
      case "canon":
        stats.canon++;
        break;
      case "anime_canon":
        stats.anime++;
        break;
      case "mixed_canon":
        stats.mixed++;
        break;
      case "filler":
        stats.filler++;
        break;
    }
  }
  return { stats, maxEpisode };
}

export function ShowDetailPage() {
  const { id: showId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const addNotification = useNotificationStore((s) => s.addNotification);

  const showQuery = useShow(showId);
  const episodesQuery = useEpisodes(showId);
  const { data: artworkProviders = [] } = useArtworkProviders();
  const { data: sonarrSeries = [] } = useSonarrSeries();
  const { data: settings } = useSettings();
  const updateShowMutation = useUpdateShow();
  const deleteShowMutation = useDeleteShow();
  const setEpisodeMonitorMutation = useSetEpisodeMonitor();

  const show = (showQuery.data as Show | undefined) ?? null;
  const episodes = (episodesQuery.data as Episode[] | undefined) ?? NO_EPISODES;
  const { stats: epStats, maxEpisode } = useMemo(() => episodeStats(episodes), [episodes]);

  const showError = showQuery.error;
  useEffect(() => {
    if (showError) addNotification("Couldn't load anime: " + errorMessage(showError), "error");
  }, [showError, addNotification]);

  // ── Tracking rules form ──
  // `values` re-seeds the form whenever the saved rules change on the
  // server; keepDirtyValues preserves edits the user hasn't saved yet.
  const savedRules = useMemo(
    () => (show ? rulesFromShow(show, maxEpisode) : undefined),
    [show, maxEpisode],
  );
  const form = useForm<RulesForm>({
    values: savedRules,
    resetOptions: { keepDirtyValues: true },
  });
  const { trackFiller, trackMixed, trackAnimeCanon, watermarkAuto, markerValue } = form.watch();
  const { dirtyFields } = form.formState;
  // Tracking rules save the moment they change; Auto/Manual and the
  // episode marker wait for the toolbar's Save. The marker only matters in
  // manual mode, so an edited marker doesn't count once back on auto.
  const trackingDirty =
    !!show && !!(dirtyFields.trackFiller || dirtyFields.trackMixed || dirtyFields.trackAnimeCanon);
  const watermarkDirty =
    !!show && !!(dirtyFields.watermarkAuto || (!watermarkAuto && dirtyFields.markerValue));
  const setRule = <K extends keyof RulesForm>(name: K, value: RulesForm[K]) =>
    form.setValue(name, value as never, { shouldDirty: true });

  const clampMarkerValue = useCallback(
    (next: number): number => Math.min(Math.max(0, Number(next) || 0), maxEpisode),
    [maxEpisode],
  );

  // ── View state ──
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [fillerFilesOpen, setFillerFilesOpen] = useState(false);
  const fillerFilesQuery = useFillerFiles(showId);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [artworkRefreshing, setArtworkRefreshing] = useState(false);
  const [artworkVersion, setArtworkVersion] = useState(0);
  const [heroImgError, setHeroImgError] = useState(false);
  const [heroFallbackFailed, setHeroFallbackFailed] = useState(false);
  const [activeTypeFilter, setActiveTypeFilter] = useState<string>("all");
  const isMobile = useIsMobile();
  const isTouch = useIsTouch();
  // Touch screens can't hover the hero to reveal the actions bar; a tap
  // toggles it.
  const [controlsOpen, setControlsOpen] = useState(false);
  const [syncingEps, setSyncingEps] = useState<Record<string, boolean>>({});
  // The actions bar animates in only after the first frame with show data,
  // so it doesn't slide in on every navigation.
  const [settledShowId, setSettledShowId] = useState<string | null>(null);
  const pageSettled = !!showId && settledShowId === showId;

  const hasShow = !!show;
  useEffect(() => {
    if (!hasShow || !showId) return;
    const frame = requestAnimationFrame(() => setSettledShowId(showId));
    return () => cancelAnimationFrame(frame);
  }, [hasShow, showId]);

  // A different show starts with a clean hero.
  useEffect(() => {
    setHeroImgError(false);
    setHeroFallbackFailed(false);
  }, [showId]);

  useEffect(() => {
    if (isMobile && activeTypeFilter === "anime_canon") {
      setActiveTypeFilter("canon");
    }
  }, [isMobile, activeTypeFilter]);

  // ── Episode list ──
  const filteredEpisodes = useMemo(() => {
    // Fast path: "all" returns the original array, avoiding a large .filter().
    if (activeTypeFilter === "all") return episodes;
    return episodes.filter((ep) => {
      if (activeTypeFilter === "canon" && isMobile) {
        return ep.type === "canon" || ep.type === "anime_canon";
      }
      return ep.type === activeTypeFilter;
    });
  }, [episodes, activeTypeFilter, isMobile]);

  // Defer the list on large shows so filter changes don't block the
  // SegmentedControl interaction.
  const deferredFilteredEpisodes = useDeferredValue(filteredEpisodes);

  const groupedBySeason = useMemo(() => {
    const groups: Record<string, Episode[]> = {};
    for (const ep of deferredFilteredEpisodes) {
      const season = String(episodeSeason(ep));
      if (!groups[season]) groups[season] = [];
      groups[season].push(ep);
    }
    // Episodes arrive sorted by episode_num ASC; reverse within each season
    // to show newest first. (Groups hold copies, so the cached list is
    // never mutated.)
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

  // ── Artwork ──

  /** Hero backdrop: the locally stored banner, falling back to Sonarr's
   *  fanart/banner when the local file is missing. */
  const backdropUrl = useMemo((): string => {
    if (!showId) return "";
    if (heroImgError && show) {
      const series = sonarrSeries.find((s: SonarrSeries) => s.id === show.sonarr_series_id);
      if (series?.images?.length) {
        const banner =
          series.images.find((img) => img.coverType === "fanart") ||
          series.images.find((img) => img.coverType === "banner") ||
          series.images[0];
        if (banner) return resolveSonarrImageUrl(banner, settings?.sonarr_url || "");
      }
    }
    // Versioned by the stored banner, plus a counter that busts the cache
    // after an artwork change made on this page.
    const stored = show?.artwork_banner_url || show?.artwork_banner_path || "";
    return getArtworkBannerUrl(showId, `${stored}#${artworkVersion}`);
  }, [showId, artworkVersion, heroImgError, show, sonarrSeries, settings?.sonarr_url]);

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

  // Effective source for the picker: the per-show override, else the global
  // default. The picker only appears for sources with candidate images.
  const resolvedArtworkSource =
    show?.artwork_source || settings?.artwork_default_source || "sonarr";
  const pickerSources = useMemo<ArtworkPickerSource[]>(
    () =>
      artworkProviders
        .filter((p) => PICKER_SOURCES.has(p.id) && p.available)
        .map((p) => ({ id: p.id, label: p.label })),
    [artworkProviders],
  );

  /** Re-read the show after an artwork change and bust the image cache. */
  const reloadArtwork = async () => {
    if (!showId) return;
    await invalidateShow(queryClient, showId);
    setHeroImgError(false);
    setHeroFallbackFailed(false);
    setArtworkVersion((v) => v + 1);
  };

  const withArtworkBusy = async (action: () => Promise<void>, failure: string) => {
    setArtworkRefreshing(true);
    try {
      await action();
    } catch (err: unknown) {
      addNotification(`${failure}: ${errorMessage(err)}`, "error");
    }
    setArtworkRefreshing(false);
  };

  const handleArtworkPick = (source: string, slot: ArtworkSlot, url: string) =>
    withArtworkBusy(async () => {
      if (!showId) return;
      // Partial pick: only the chosen slot is sent; the backend keeps the
      // other slots' existing files.
      await selectArtwork(showId, {
        source,
        poster_url: slot === "poster" ? url : "",
        banner_url: slot === "banner" ? url : "",
        logo_url: slot === "logo" ? url : "",
      });
      await reloadArtwork();
      addNotification(`Artwork ${slot} updated`, "success");
    }, "Couldn't save artwork");

  const handleUploadCustom = (slot: ArtworkSlot, file: File) =>
    withArtworkBusy(async () => {
      if (!showId) return;
      if (file.size > MAX_CUSTOM_IMAGE_BYTES) {
        addNotification(`That image is over ${MAX_CUSTOM_IMAGE_BYTES >> 20} MB`, "error");
        return;
      }
      await uploadCustomArtwork(showId, slot, file);
      await reloadArtwork();
      addNotification(`Your ${slot} is set`, "success");
    }, "Couldn't upload image");

  const handleRemoveCustom = (slot: ArtworkSlot) =>
    withArtworkBusy(async () => {
      if (!showId) return;
      await removeCustomArtwork(showId, slot);
      // Let the show's automatic source fill the slot again.
      await refreshArtwork(showId);
      await reloadArtwork();
      addNotification(`Removed your ${slot}`, "success");
    }, "Couldn't remove image");

  const handleRefreshArtwork = (source?: string) =>
    withArtworkBusy(async () => {
      if (!showId) return;
      const result = await refreshArtwork(showId, source);
      await reloadArtwork();
      if (result.found) {
        addNotification(`Artwork fetched from ${result.source}`, "success");
      } else {
        addNotification(
          source === "sonarr"
            ? "Using Sonarr images (no download needed)"
            : "No artwork found from this source",
          "warning",
        );
      }
    }, "Couldn't fetch artwork");

  const handleArtworkSourceChange = (source: string) =>
    withArtworkBusy(async () => {
      if (!showId) return;
      await setArtworkSource(showId, source);
      if (source === "" || source === "sonarr") {
        await reloadArtwork();
        addNotification(
          source === "" ? "Using global default artwork source" : "Switched to Sonarr images",
          "info",
        );
        return;
      }
      const result = await refreshArtwork(showId, source);
      await reloadArtwork();
      if (result.found) {
        addNotification(`Switched to ${result.source} artwork`, "success");
      } else {
        addNotification(`No artwork found from ${source}`, "warning");
      }
    }, "Couldn't set artwork source");

  // ── Save / delete ──

  // Saves the given fields, then takes the saved record as the baseline.
  // Anything the save didn't cover stays unsaved rather than reset away.
  const saveFields = async (keys: (keyof RulesForm)[], payload: ShowUpdate) => {
    if (!showId) return false;
    const sent = form.getValues();
    try {
      const saved = await updateShowMutation.mutateAsync({ id: showId, data: payload });
      const latest = form.getValues();
      // Changes this save didn't cover: other fields still unsaved, and
      // these fields changed again while it was in flight.
      const pending = (Object.keys(latest) as (keyof RulesForm)[]).filter((key) =>
        keys.includes(key) ? latest[key] !== sent[key] : form.getFieldState(key).isDirty,
      );
      form.reset(rulesFromShow(saved, maxEpisode), { keepDirtyValues: false });
      for (const key of pending) setRule(key, latest[key]);
      addNotification(`Saved ${show?.sonarr_title || "anime"}`, "success");
      return true;
    } catch (err: unknown) {
      addNotification("Couldn't save anime: " + errorMessage(err), "error");
      return false;
    }
  };

  const saveTracking = () => {
    const rules = form.getValues();
    return saveFields(["trackFiller", "trackMixed", "trackAnimeCanon"], {
      filler_action: trackedToAction(rules.trackFiller),
      mixed_action: trackedToAction(rules.trackMixed),
      anime_canon_action: trackedToAction(rules.trackAnimeCanon),
    });
  };

  const saveWatermark = () => {
    const rules = form.getValues();
    const payload: ShowUpdate = {
      watermark_mode: rules.watermarkAuto ? WATERMARK_AUTO : WATERMARK_MANUAL,
    };
    if (!rules.watermarkAuto) payload.sync_watermark = clampMarkerValue(rules.markerValue);
    return saveFields(["watermarkAuto", "markerValue"], payload);
  };

  // A tracking toggle saves at once (after any save already running).
  const saveTrackingRef = useRef(saveTracking);
  saveTrackingRef.current = saveTracking;
  const saving = updateShowMutation.isPending;
  useEffect(() => {
    if (!trackingDirty || saving) return;
    const timer = setTimeout(() => void saveTrackingRef.current(), TRACKING_SAVE_DELAY_MS);
    return () => clearTimeout(timer);
  }, [trackingDirty, saving, trackFiller, trackMixed, trackAnimeCanon]);
  // Leaving the page right after a toggle still sends it.
  const trackingDirtyRef = useRef(trackingDirty);
  trackingDirtyRef.current = trackingDirty;
  useEffect(
    () => () => {
      if (trackingDirtyRef.current) void saveTrackingRef.current();
    },
    [],
  );

  const handleMuteChange = async (muted: boolean) => {
    if (!showId) return;
    try {
      await updateShowMutation.mutateAsync({ id: showId, data: { notifications_muted: muted } });
      addNotification(
        muted
          ? `Muted notifications for ${show?.sonarr_title || "anime"}`
          : `Unmuted notifications for ${show?.sonarr_title || "anime"}`,
        "success",
      );
    } catch (err: unknown) {
      addNotification("Couldn't update notifications: " + errorMessage(err), "error");
    }
  };

  const handleDelete = async () => {
    if (!showId) return;
    try {
      await deleteShowMutation.mutateAsync(showId);
      addNotification(`Removed ${show?.sonarr_title || "anime"} from Mizu`, "success");
      setShowDeleteDialog(false);
      navigate("/shows");
    } catch (err: unknown) {
      addNotification("Couldn't remove anime: " + errorMessage(err), "error");
    }
  };

  const { mutateAsync: setEpisodeMonitor } = setEpisodeMonitorMutation;
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
        await setEpisodeMonitor({ showId, epId: sonarrEp.id, monitored: nextState });
        addNotification(
          `${episodeLabel(ep)} ${nextState ? "now monitored" : "now unmonitored"}`,
          "success",
        );
      } catch (err: unknown) {
        addNotification("Couldn't update episode: " + errorMessage(err), "error");
      }
      setSyncingEps((prev) => {
        const next = { ...prev };
        delete next[key];
        return next;
      });
    },
    [showId, addNotification, setEpisodeMonitor],
  );

  const retryEpisodes = useCallback(() => void episodesQuery.refetch(), [episodesQuery]);

  return (
    <>
      <Section
        variant="transparent"
        padding={4}
        maxWidth={900}
        className="section-centered page-enter"
      >
        <VStack gap={6}>
          <div
            className={`show-shell${pageSettled ? " is-settled" : ""}${controlsOpen ? " controls-open" : ""}`}
          >
            <ShowHero
              show={show}
              backdropUrl={heroFallbackFailed ? "" : backdropUrl}
              imageKey={`${showId}-${artworkVersion}-${heroImgError ? "fallback" : "local"}`}
              controlsToggle={
                isTouch
                  ? {
                      isOpen: controlsOpen,
                      onToggle: () => setControlsOpen((open) => !open),
                    }
                  : undefined
              }
              onImageError={() => {
                if (heroImgError) {
                  setHeroFallbackFailed(true);
                } else {
                  setHeroImgError(true);
                }
              }}
            />

            {show && (
              <ShowActionsBar
                watermarkAuto={watermarkAuto}
                markerValue={markerValue}
                maxEpisode={maxEpisode}
                isMobile={isMobile}
                deleting={deleteShowMutation.isPending}
                onMarkerChange={(value) => setRule("markerValue", clampMarkerValue(value))}
                onToggleAuto={() => setRule("watermarkAuto", !watermarkAuto)}
                notificationsMuted={!!show.notifications_muted}
                notificationsSaving={updateShowMutation.isPending}
                onToggleMuted={() => void handleMuteChange(!show.notifications_muted)}
                onEditArtwork={() => setPickerOpen(true)}
                hasFillerFiles={(fillerFilesQuery.data?.files.length ?? 0) > 0}
                onFillerFiles={() => setFillerFilesOpen(true)}
                onRemove={() => setShowDeleteDialog(true)}
                watermarkDirty={watermarkDirty}
                savingWatermark={saving}
                onSaveWatermark={() => void saveWatermark()}
              />
            )}
          </div>

          <Divider />

          {!show ? (
            <ShowDetailSkeleton />
          ) : (
            <>
              <TrackingRulesSection
                trackAnimeCanon={trackAnimeCanon}
                trackMixed={trackMixed}
                trackFiller={trackFiller}
                onAnimeCanonChange={(value) => setRule("trackAnimeCanon", value)}
                onMixedChange={(value) => setRule("trackMixed", value)}
                onFillerChange={(value) => setRule("trackFiller", value)}
              />
            </>
          )}

          {/* Rendered outside the show gate so the episode skeleton appears
            immediately, even before the show itself has loaded. */}
          {showId && (
            <EpisodeListSection
              episodesLoading={episodesQuery.isPending}
              episodesError={episodesQuery.error ? errorMessage(episodesQuery.error) : null}
              onRetryEpisodes={retryEpisodes}
              hasShowData={!!show}
              groupedBySeason={groupedBySeason}
              listStale={deferredFilteredEpisodes !== filteredEpisodes}
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
            description={`Removes "${show.sonarr_title}" from Mizu. It stays in Sonarr, and you can add it back anytime.`}
            actionLabel="Remove"
            isActionLoading={deleteShowMutation.isPending}
            onAction={() => void handleDelete()}
          />
        </Suspense>
      )}

      {showId && (
        <FillerFilesDialog
          isOpen={fillerFilesOpen}
          onOpenChange={setFillerFilesOpen}
          showId={showId}
          isMobile={isMobile}
        />
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
          onSelect={(source, slot, url) => void handleArtworkPick(source, slot, url)}
          artworkSource={show?.artwork_source || ""}
          artworkSourceOptions={artworkSourceOptions}
          refreshing={artworkRefreshing}
          onArtworkSourceChange={(source) => void handleArtworkSourceChange(source)}
          customSlots={show?.artwork_custom_slots ?? []}
          onUploadCustom={(slot, file) => void handleUploadCustom(slot, file)}
          onRemoveCustom={(slot) => void handleRemoveCustom(slot)}
          onRefresh={() => void handleRefreshArtwork()}
        />
      )}
    </>
  );
}
