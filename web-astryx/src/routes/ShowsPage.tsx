import { useEffect, useState, useCallback, useMemo, useReducer, useRef, lazy, Suspense } from "react";
import { useNavigate } from "react-router-dom";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { StackItem } from "@astryxdesign/core/Stack";
import { Section } from "@astryxdesign/core/Section";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Badge } from "@astryxdesign/core/Badge";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { Icon } from "@astryxdesign/core/Icon";
import { Dialog } from "@astryxdesign/core/Dialog";
import { TextInput } from "@astryxdesign/core/TextInput";
import { DropdownMenu } from "@astryxdesign/core/DropdownMenu";
import { ClickableCard } from "@astryxdesign/core/ClickableCard";
import { Grid } from "@astryxdesign/core/Grid";
import { Plus, Search, Waves, CircleAlert, RefreshCw, Check, ArrowDownUp } from "lucide-react";
import { useAppStore } from "../lib/store";
import { getShows, getSettings, getSonarrAddOptions, getSonarrSeries, getArtworkPosterUrl, getArtworkBannerUrl, getArtworkLogoUrl, refreshAllArtwork, getArtworkRefreshStatus, prefetchEpisodes, errorMessage } from "../lib/api";
import { prefetchDetailChunk, prefetchAddDialogChunk } from "../lib/chunkPrefetch";
import type { SelectOption, Settings, Show, SonarrAddOptions, SonarrSeries } from "../lib/types";
import { resolveSonarrImageUrl } from "../lib/sonarr-images";
import { syncStateBadge } from "../lib/fox";
import {
  addWizardReducer,
  INITIAL_WIZARD,
  actionToTracked,
  getWizardSteps,
  START_FROM_CURRENT,
} from "./addWizardHelpers";
import type { AddWizardState } from "./addWizardHelpers";

// Lazy-load the AddAnimeDialog component — it's 700+ lines of wizard UI
// that's only needed when the user opens the "Add anime" dialog. The
// reducer/helpers above are tiny and stay eager.
const AddAnimeDialog = lazy(() =>
  import("./AddAnimeDialog").then((m) => ({ default: m.AddAnimeDialog })),
);

const sortOptions: SelectOption[] = [
  { value: "title", label: "Title (A-Z)" },
  { value: "title_desc", label: "Title (Z-A)" },
  { value: "watermark", label: "Latest progress" },
];

export function ShowsPage() {
  const navigate = useNavigate();
  const { shows, setShows, settings, setSettings, addNotification } = useAppStore();

  // ── Page-level state ──
  // If the store already has shows, don't show a loading spinner — render
  // immediately. SonarrSeries data (for poster/banner URLs) loads in the
  // background and populates images when ready.
  const [loading, setLoading] = useState(() => !shows || shows.length === 0);
  const [filter, setFilter] = useState("");
  const [sortBy, setSortBy] = useState("title");
  const [artworkRefreshing, setArtworkRefreshing] = useState(false);
  const artworkReloadTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [allSeries, setAllSeries] = useState<SonarrSeries[]>([]);
  const [addSeriesLibrary, setAddSeriesLibrary] = useState<SonarrSeries[]>([]);
  const [seriesById, setSeriesById] = useState<Record<string | number, SonarrSeries>>({});

  // ── Wizard state ──
  const [w, dispatch] = useReducer(addWizardReducer, INITIAL_WIZARD);

  // ── Derived values ──

  // Only the settings fields that affect artwork URL resolution — avoids
  // recomputing all show posters/banners when unrelated settings change.
  const artworkSettings = useMemo(
    () => ({
      sonarr_url: settings?.sonarr_url,
    }),
    [settings?.sonarr_url],
  );

  const mappedSeriesIds = useMemo(
    () => new Set((shows || []).map((s: Show) => s.sonarr_series_id)),
    [shows],
  );

  const mappedShows = useMemo(
    () =>
      (shows || []).map((show: Show) => {
        const series = seriesById[show.sonarr_series_id] || null;
        return {
          ...show,
          poster: posterUrl(show, series),
          banner: bannerUrl(show, series),
          logo: logoUrl(show),
        };
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [shows, seriesById, artworkSettings],
  );

  const filteredShows = useMemo(
    () =>
      mappedShows
        .filter((show: Show & { poster?: string }) => {
          if (!filter) return true;
          const q = filter.toLowerCase();
          return (
            (show.sonarr_title || "").toLowerCase().includes(q) ||
            (show.afl_title || show.afl_slug || "").toLowerCase().includes(q)
          );
        })
        .sort((a: Show, b: Show) => {
          if (sortBy === "title") return (a.sonarr_title || "").localeCompare(b.sonarr_title || "");
          if (sortBy === "title_desc")
            return (b.sonarr_title || "").localeCompare(a.sonarr_title || "");
          if (sortBy === "watermark") return (b.sync_watermark || 0) - (a.sync_watermark || 0);
          return 0;
        }),
    [mappedShows, filter, sortBy],
  );

  const filteredSeriesLibrary = useMemo(
    () =>
      addSeriesLibrary
        .filter((series: SonarrSeries) => {
          const q = w.seriesQuery.trim().toLowerCase();
          if (!q) return true;
          return (
            (series.title || "").toLowerCase().includes(q) || String(series.year || "").includes(q)
          );
        })
        .sort((a: SonarrSeries, b: SonarrSeries) => {
          const aMapped = mappedSeriesIds.has(a.id) ? 1 : 0;
          const bMapped = mappedSeriesIds.has(b.id) ? 1 : 0;
          if (aMapped !== bMapped) return aMapped - bMapped;
          return a.title.localeCompare(b.title);
        }),
    [addSeriesLibrary, w.seriesQuery, mappedSeriesIds],
  );

  const activeWizardSteps = getWizardSteps(w.addStep);
  const currentStepIndex = activeWizardSteps.findIndex((s) => s.key === w.addStep);

  function posterUrl(show: Show, series: SonarrSeries | null | undefined): string {
    // 1. Locally downloaded artwork — always prefer when files exist
    if (show.artwork_poster_path) {
      return getArtworkPosterUrl(show.id);
    }
    // 2. Sonarr images (fallback)
    return posterUrlForSeries(series);
  }

  function posterUrlForSeries(series: SonarrSeries | null | undefined): string {
    if (!series?.images?.length) return "";
    const preferred =
      series.images.find((img) => img.coverType === "poster") ||
      series.images.find((img) => img.coverType === "fanart") ||
      series.images[0];
    if (!preferred) return "";
    return resolveSonarrImageUrl(preferred, artworkSettings.sonarr_url || "");
  }

  function bannerUrl(show: Show, series: SonarrSeries | null | undefined): string {
    // 1. Locally downloaded artwork — always prefer when files exist
    if (show.artwork_banner_path) {
      return getArtworkBannerUrl(show.id);
    }
    // 2. Sonarr images (fallback)
    if (!series?.images?.length) return "";
    const preferred =
      series.images.find((img) => img.coverType === "fanart") ||
      series.images.find((img) => img.coverType === "banner") ||
      series.images[0];
    if (!preferred) return "";
    return resolveSonarrImageUrl(preferred, artworkSettings.sonarr_url || "");
  }

  function logoUrl(show: Show): string {
    if (!show.artwork_logo_path) return "";
    return getArtworkLogoUrl(show.id);
  }

  // ── Sonarr options (for the dialog) ──

  const preferredAnimeRootFolder = useMemo((): string => {
    const counts = new Map<string, number>();
    let bestPath = "";
    let bestCount = 0;
    for (const series of allSeries) {
      const path = (series?.rootFolderPath || "").trim();
      if (!path) continue;
      const next = (counts.get(path) || 0) + 1;
      counts.set(path, next);
      if (next > bestCount) {
        bestCount = next;
        bestPath = path;
      }
    }
    return bestPath || w.sonarrRootFolderOptions[0]?.value || "";
  }, [allSeries, w.sonarrRootFolderOptions]);

  const preferredAnimeQualityProfileId = useMemo((): string => {
    const counts = new Map<string, number>();
    let bestValue = "";
    let bestCount = 0;
    for (const series of allSeries) {
      const value = Number(series?.qualityProfileId || 0);
      if (!value) continue;
      const key = String(value);
      const next = (counts.get(key) || 0) + 1;
      counts.set(key, next);
      if (next > bestCount) {
        bestCount = next;
        bestValue = key;
      }
    }
    return bestValue || w.sonarrQualityProfileOptions[0]?.value || "";
  }, [allSeries, w.sonarrQualityProfileOptions]);

  // ── Data loading ──

  const showsRef = useRef(shows);
  showsRef.current = shows;

  const loadLanding = useCallback(async (forceRefresh: boolean = false) => {
    // If the store already has shows and we're not forcing a refresh,
    // skip the redundant getShows + getSettings round-trip — only
    // SonarrSeries (for poster/banner URLs) needs fetching since it's
    // not in the store.
    const currentShows = showsRef.current;
    if (!forceRefresh && currentShows && currentShows.length > 0) {
      setLoading(false);
      getSonarrSeries()
        .then((nextSeries) => {
          setAllSeries(nextSeries || []);
          setSeriesById(Object.fromEntries((nextSeries || []).map((s) => [s.id, s])));
        })
        .catch((err: unknown) => {
          addNotification("Failed to load Sonarr series: " + errorMessage(err), "error");
        });
      return;
    }

    // First load with no store data: the store is already fetching
    // bootstrap (shows + settings + providers) on mount. Don't make
    // redundant getShows() + getSettings() calls — just fetch
    // SonarrSeries in the background. The store will populate shows
    // when bootstrap resolves, and the shows-changed effect below
    // will clear the loading state.
    if (!forceRefresh) {
      getSonarrSeries()
        .then((nextSeries) => {
          setAllSeries(nextSeries || []);
          setSeriesById(Object.fromEntries((nextSeries || []).map((s) => [s.id, s])));
        })
        .catch((err: unknown) => {
          addNotification("Failed to load Sonarr series: " + errorMessage(err), "error");
        });
      return;
    }

    // Forced refresh — fetch shows + settings first, then SonarrSeries
    // in the background.
    const [showsRes, settingsRes] = await Promise.allSettled([
      getShows(),
      getSettings(),
    ]);
    if (showsRes.status === "fulfilled") {
      setShows(showsRes.value || []);
    } else {
      setShows([]);
      addNotification("Failed to load anime: " + errorMessage(showsRes.reason), "error");
    }
    if (settingsRes.status === "fulfilled") {
      setSettings(settingsRes.value ?? null);
    }
    setLoading(false);

    // Load Sonarr series in the background — needed for poster/banner
    // URLs on shows without local artwork. Don't block the page render.
    getSonarrSeries()
      .then((nextSeries) => {
        setAllSeries(nextSeries || []);
        setSeriesById(Object.fromEntries((nextSeries || []).map((s) => [s.id, s])));
      })
      .catch((err: unknown) => {
        addNotification("Failed to load Sonarr series: " + errorMessage(err), "error");
      });
  }, [setShows, setSettings, addNotification]);

  // When the store populates shows (from bootstrap), clear the loading
  // state. This handles the first-load case where ShowsPage mounts
  // before bootstrap resolves — the store will update shows, and this
  // effect reacts to it without making redundant API calls.
  useEffect(() => {
    if (shows && shows.length > 0) {
      setLoading(false);
    }
  }, [shows]);

  useEffect(() => {
    loadLanding();
  }, [loadLanding]);

  useEffect(() => {
    return () => {
      if (artworkReloadTimer.current) clearTimeout(artworkReloadTimer.current);
    };
  }, []);

  const handleRefreshAllArtwork = async () => {
    if (artworkReloadTimer.current) {
      clearTimeout(artworkReloadTimer.current);
      artworkReloadTimer.current = null;
    }
    setArtworkRefreshing(true);
    try {
      await refreshAllArtwork();
      addNotification("Artwork refresh triggered — waiting for completion...", "info", 6000);
      // Poll the backend until the refresh completes, then reload shows.
      const poll = async () => {
        try {
          const status = await getArtworkRefreshStatus();
          if (!status.refreshing) {
            artworkReloadTimer.current = null;
            await loadLanding(true);
            setArtworkRefreshing(false);
            addNotification("Artwork refresh complete", "success");
          } else {
            artworkReloadTimer.current = setTimeout(poll, 3000);
          }
        } catch {
          // If the status check fails, fall back to a single reload after a delay.
          artworkReloadTimer.current = setTimeout(() => {
            artworkReloadTimer.current = null;
            loadLanding(true).then(() => setArtworkRefreshing(false));
          }, 8000);
        }
      };
      artworkReloadTimer.current = setTimeout(poll, 3000);
    } catch (err: unknown) {
      addNotification("Artwork refresh failed: " + errorMessage(err), "error");
      setArtworkRefreshing(false);
    }
  };

  const refreshSettings = async (): Promise<Settings | null> => {
    try {
      const current = await getSettings();
      setSettings(current);
      return current;
    } catch (err: unknown) {
      addNotification("Failed to load settings: " + errorMessage(err), "error");
      return null;
    }
  };

  const resetAddRuleDefaults = (current: Settings | null = settings) => {
    dispatch({
      type: "SET",
      payload: {
        addTrackFiller: actionToTracked(current?.default_filler_action, false),
        addTrackMixed: actionToTracked(current?.default_mixed_canon_action, true),
        addTrackAnimeCanon: actionToTracked(current?.default_anime_canon_action, true),
        addStartMode: START_FROM_CURRENT,
      },
    });
  };

  const loadSonarrAddOptions = async () => {
    try {
      const data: SonarrAddOptions = await getSonarrAddOptions();
      const payload: Partial<AddWizardState> = {
        sonarrRootFolderOptions: (data.root_folders || []).map((f) => ({
          value: f.path,
          label: f.path,
        })),
        sonarrQualityProfileOptions: (data.quality_profiles || []).map((p) => ({
          value: String(p.id),
          label: p.name,
        })),
      };
      if (!w.fallbackQualityProfileId) {
        payload.fallbackQualityProfileId = preferredAnimeQualityProfileId;
      }
      dispatch({ type: "SET", payload });
    } catch (err: unknown) {
      addNotification("Failed to load Sonarr add options: " + errorMessage(err), "error");
    }
  };

  // ── Add modal flow ──

  const openAddModal = async () => {
    dispatch({ type: "OPEN_ADD_MODAL_INIT" });

    const [refreshedSettings] = await Promise.all([refreshSettings(), loadSonarrAddOptions()]);
    resetAddRuleDefaults(refreshedSettings);

    dispatch({ type: "SET", payload: { sonarrLoading: true } });
    try {
      const [fullSeries, modalSeries] = await Promise.all([
        allSeries.length ? Promise.resolve(allSeries) : getSonarrSeries(),
        getSonarrSeries(true),
      ]);
      setAllSeries(fullSeries || []);
      setAddSeriesLibrary(modalSeries || []);
      setSeriesById(Object.fromEntries((fullSeries || []).map((s) => [s.id, s])));
    } catch (err: unknown) {
      addNotification("Failed to load Sonarr series: " + errorMessage(err), "error");
    }
    dispatch({ type: "SET", payload: { sonarrLoading: false } });
  };

  const openShow = useCallback((showId: string | number) => {
    navigate(`/shows/${showId}`);
  }, [navigate]);

  const handleCardHover = useCallback((showId: string | number) => {
    prefetchDetailChunk();
    // Only prefetch episodes — the show data is already in the store
    // (from bootstrap), so prefetchShow() would be a redundant API call.
    prefetchEpisodes(showId);
  }, []);

  const openExistingMapping = (seriesId: string | number) => {
    const mapped = (shows || []).find((s: Show) => s.sonarr_series_id === seriesId);
    if (!mapped) return;
    dispatch({ type: "SET", payload: { showAddModal: false } });
    openShow(mapped.id);
  };

  // ── Render ──

  return (
    <Section padding={4} maxWidth={900} className="section-centered page-enter">
      <VStack gap={6}>
        {/* Row 1: title + supporting text left, add right */}
        <HStack justify="between" align="center" wrap="wrap" gap={4}>
          <VStack gap={1}>
            <Heading level={1}>Library</Heading>
            <Text type="supporting" color="secondary">
              {loading ? "Loading..." : `${(shows || []).length} tracked`}
            </Text>
          </VStack>
          <HStack gap={2} align="center">
            <Button
              label={artworkRefreshing ? "Refreshing..." : "Refresh artwork"}
              variant="ghost"
              size="md"
              icon={<RefreshCw size={16} className={artworkRefreshing ? "spin" : ""} />}
              onClick={handleRefreshAllArtwork}
              isDisabled={artworkRefreshing || loading}
              data-cuelume-press
            />
            <Button
              label="Add anime"
              variant="primary"
              icon={<Plus size={16} />}
              onClick={openAddModal}
              onMouseEnter={prefetchAddDialogChunk}
              data-cuelume-press
            />
          </HStack>
        </HStack>

        {/* Row 2: search (fill) + sort, independent */}
        <HStack gap={3} align="center" wrap="nowrap" width="100%">
          <StackItem size="fill">
            <TextInput
              label="Search anime"
              value={filter}
              onChange={setFilter}
              placeholder="Search anime..."
              isLabelHidden
              startIcon={Search}
              width="100%"
            />
          </StackItem>
          <StackItem size="static">
            <DropdownMenu
              button={{
                label: sortOptions.find((o) => o.value === sortBy)?.label ?? "Sort",
                size: "lg",
                icon: <ArrowDownUp size={16} />,
              }}
              items={sortOptions.map((o) => ({
                label: o.label,
                icon: o.value === sortBy ? <Check size={14} /> : undefined,
                onClick: () => setSortBy(o.value),
              }))}
            />
          </StackItem>
        </HStack>

        {/* Content */}
        {loading ? (
          <Grid columns={{ minWidth: 320 }} gap={4}>
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="mizu-card skeleton-card" style={{ overflow: "hidden" }}>
                <div className="skeleton" style={{ width: "100%", height: 160, borderRadius: 0 }} />
                <div style={{ padding: "var(--spacing-4)", display: "flex", flexDirection: "column", gap: "var(--spacing-2)" }}>
                  <div className="skeleton-text-lg" style={{ width: "70%" }} />
                  <div className="skeleton-text" style={{ width: "50%" }} />
                </div>
              </div>
            ))}
          </Grid>
        ) : filteredShows.length === 0 && (shows || []).length === 0 ? (
          <EmptyState
            title="No anime synced yet"
            description="Add your first Sonarr anime mapping to start monitoring filler decisions in Mizu."
            icon={<Icon icon={Waves} size="lg" color="secondary" />}
            actions={
              <Button
                label="Add anime"
                variant="primary"
                icon={<Plus size={16} />}
                onClick={openAddModal}
                onMouseEnter={prefetchAddDialogChunk}
                data-cuelume-press
              />
            }
          />
        ) : filteredShows.length === 0 ? (
          <EmptyState
            title="No matching anime"
            description="Try another title, slug, or keyword."
            icon={<Icon icon={CircleAlert} size="lg" color="secondary" />}
            actions={
              <Button label="Clear search" variant="secondary" onClick={() => setFilter("")} data-cuelume-press />
            }
          />
        ) : (
          <VStack gap={4}>
            <Grid columns={{ minWidth: 320 }} gap={4}>
              {filteredShows.map((show: Show & { poster?: string; banner?: string; logo?: string }) => {
                const badge = syncStateBadge(show.enabled);
                const img = show.banner || show.poster;
                const logo = show.logo;
                return (
                  <ClickableCard
                    key={String(show.id)}
                    label={show.sonarr_title}
                    onClick={() => openShow(show.id)}
                    padding={0}
                    className="mizu-card"
                    onMouseEnter={() => handleCardHover(show.id)}
                  >
                    <div className={`mizu-card-img-wrap${img ? "" : " is-empty"}`}>
                      {img ? (
                        <img
                          src={img}
                          alt=""
                          loading="lazy"
                          onLoad={(e) => {
                            e.currentTarget.classList.add("is-loaded");
                          }}
                          className="mizu-card-img"
                        />
                      ) : (
                        <div className="mizu-card-img-placeholder">
                          <Waves size={24} color="var(--color-icon-secondary)" />
                        </div>
                      )}
                      {img && (
                        <div className="mizu-card-overlay">
                          {logo ? (
                            <img
                              src={logo}
                              alt={show.sonarr_title}
                              className="mizu-card-logo"
                            />
                          ) : (
                            <span className="mizu-card-overlay-title">{show.sonarr_title}</span>
                          )}
                          <Badge
                            variant={show.enabled ? "success" : "neutral"}
                            label={badge.label}
                          />
                        </div>
                      )}
                    </div>
                    {!img && (
                      <Section variant="transparent" padding={3}>
                        <VStack gap={1}>
                          <HStack justify="between" align="center" wrap="nowrap">
                            <Heading
                              level={3}
                              className="mizu-card-title"
                            >
                              {show.sonarr_title}
                            </Heading>
                            <Badge
                              variant={show.enabled ? "success" : "neutral"}
                              label={badge.label}
                            />
                          </HStack>
                          <Text type="supporting" size="sm">
                            #{show.sync_watermark > 0 ? show.sync_watermark : 0}
                          </Text>
                        </VStack>
                      </Section>
                    )}
                  </ClickableCard>
                );
              })}
            </Grid>
          </VStack>
        )}
      </VStack>

      {/* Add anime dialog — extracted component */}
      <Dialog
        isOpen={w.showAddModal}
        onOpenChange={(v) => {
          dispatch({ type: "SET", payload: { showAddModal: v } });
        }}
        width={640}
        maxHeight="85vh"
        purpose="form"
      >
        <Suspense fallback={null}>
        <AddAnimeDialog
          w={w}
          dispatch={dispatch}
          allSeries={allSeries}
          setAllSeries={setAllSeries}
          setSeriesById={setSeriesById}
          mappedSeriesIds={mappedSeriesIds}
          filteredSeriesLibrary={filteredSeriesLibrary}
          activeWizardSteps={activeWizardSteps}
          currentStepIndex={currentStepIndex}
          addNotification={addNotification}
          loadLanding={loadLanding}
          openShow={openShow}
          openExistingMapping={openExistingMapping}
          preferredAnimeRootFolder={preferredAnimeRootFolder}
          preferredAnimeQualityProfileId={preferredAnimeQualityProfileId}
          posterUrl={posterUrlForSeries}
        />
        </Suspense>
      </Dialog>
    </Section>
  );
}
