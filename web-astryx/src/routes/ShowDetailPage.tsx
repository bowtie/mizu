import { useEffect, useState, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { VStack } from '@astryxdesign/core/VStack';
import { HStack } from '@astryxdesign/core/HStack';
import { Section } from '@astryxdesign/core/Section';
import { Grid } from '@astryxdesign/core/Grid';
import { Heading } from '@astryxdesign/core/Heading';
import { Text } from '@astryxdesign/core/Text';
import { Button } from '@astryxdesign/core/Button';
import { Badge } from '@astryxdesign/core/Badge';
import { Spinner } from '@astryxdesign/core/Spinner';
import { EmptyState } from '@astryxdesign/core/EmptyState';
import { Icon } from '@astryxdesign/core/Icon';
import { AlertDialog } from '@astryxdesign/core/AlertDialog';
import { Tooltip } from '@astryxdesign/core/Tooltip';
import { Switch } from '@astryxdesign/core/Switch';
import { Card } from '@astryxdesign/core/Card';
import { ToggleButton } from '@astryxdesign/core/ToggleButton';
import { NumberInput } from '@astryxdesign/core/NumberInput';
import { InputGroup, InputGroupText } from '@astryxdesign/core/InputGroup';
import { Collapsible } from '@astryxdesign/core/Collapsible';
import { List, ListItem } from '@astryxdesign/core/List';
import { SegmentedControl, SegmentedControlItem } from '@astryxdesign/core/SegmentedControl';
import { Trash2, Save, Check, CircleAlert, Bot, Eye, EyeOff, BookOpen, Tv, Shuffle, SkipForward, Minus, Plus } from 'lucide-react';
import { useAppStore } from '../lib/store';
import {
  deleteShow,
  getAniListArtwork,
  getEpisodes,
  getSettings,
  getShows,
  getSonarrSeries,
  setEpisodeMonitor,
  updateShow,
} from '../lib/api';
import type { Artwork, Episode, Settings, Show, ShowUpdate, SonarrSeries } from '../lib/types';
import { resolveSonarrImageUrl } from '../lib/sonarr-images';
import { episodeTypeBadge } from '../lib/fox';

const WATERMARK_AUTO = 'auto';
const WATERMARK_MANUAL = 'manual';

const actionToTracked = (action: string | undefined, fallback: boolean = true): boolean =>
  (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
const trackedToAction = (tracked: boolean): string => (tracked ? 'monitor' : 'unmonitor');

function isSpecialSeasonNumber(num: number | undefined): boolean {
  return num === -1 || num === 0;
}
function seasonLabel(num: number | undefined): string {
  return isSpecialSeasonNumber(num) ? 'Specials' : `Season ${num}`;
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
  const rawSeason = typeof raw === 'number' && Number.isInteger(raw) ? raw : -1;
  if (isSpecialSeasonNumber(rawSeason)) return -1;
  return rawSeason;
}
function resolveSonarrEpisode(ep: Episode) {
  if (!ep.sonarr_episode_id) return null;
  return { id: ep.sonarr_episode_id, monitored: !!ep.monitored };
}
function episodeMonitored(ep: Episode): boolean {
  return !!resolveSonarrEpisode(ep)?.monitored;
}

function badgeVariantForType(type: string | undefined | null): 'neutral' | 'blue' | 'purple' | 'yellow' | 'red' {
  switch (type) {
    case 'canon': return 'blue';
    case 'anime_canon': return 'purple';
    case 'mixed_canon':
    case 'mixed': return 'yellow';
    case 'filler': return 'red';
    default: return 'neutral';
  }
}

export function ShowDetailPage() {
  const { id: showId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { settings, setSettings, addNotification } = useAppStore();

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [savedFlash, setSavedFlash] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [show, setShow] = useState<Show | null>(null);
  const [series, setSeries] = useState<SonarrSeries | null>(null);
  const [artwork, setArtwork] = useState<Artwork | null>(null);
  const [episodes, setEpisodes] = useState<Episode[]>([]);
  const [trackFiller, setTrackFiller] = useState(false);
  const [trackMixed, setTrackMixed] = useState(true);
  const [trackAnimeCanon, setTrackAnimeCanon] = useState(true);
  const [watermarkAuto, setWatermarkAuto] = useState(true);
  const [markerValue, setMarkerValue] = useState(0);
  const [activeTypeFilter, setActiveTypeFilter] = useState<string>('all');
  const [isMobile, setIsMobile] = useState(false);
  const [syncingEps, setSyncingEps] = useState<Record<string, boolean>>({});

  const [pristineFiller, setPristineFiller] = useState(false);
  const [pristineMixed, setPristineMixed] = useState(true);
  const [pristineAnimeCanon, setPristineAnimeCanon] = useState(true);
  const [pristineWatermarkAuto, setPristineWatermarkAuto] = useState(true);
  const [pristineMarkerValue, setPristineMarkerValue] = useState(0);

  const rulesDirty = !!show && (
    trackFiller !== pristineFiller ||
    trackMixed !== pristineMixed ||
    trackAnimeCanon !== pristineAnimeCanon ||
    watermarkAuto !== pristineWatermarkAuto ||
    (!watermarkAuto && markerValue !== pristineMarkerValue)
  );

  const filteredEpisodes = episodes.filter((ep) => {
    if (activeTypeFilter === 'all') return true;
    if (activeTypeFilter === 'canon' && isMobile) {
      return ep.type === 'canon' || ep.type === 'anime_canon';
    }
    return ep.type === activeTypeFilter;
  });

  const groupedBySeason = (() => {
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
  })();

  const epStats = {
    total: episodes.length,
    canon: episodes.filter((e) => e.type === 'canon').length,
    anime: episodes.filter((e) => e.type === 'anime_canon').length,
    mixed: episodes.filter((e) => e.type === 'mixed_canon').length,
    filler: episodes.filter((e) => e.type === 'filler').length,
  };

  const clampMarkerValue = (next: number, source: Episode[] = episodes): number => {
    const normalized = Math.max(0, Number(next) || 0);
    if (!source?.length) return normalized;
    const upper = source.reduce((highest, ep) => Math.max(highest, displayEpisodeNumber(ep)), 0);
    return Math.min(normalized, upper);
  };

  const loadShow = useCallback(async () => {
    if (!showId) return;
    setLoading(true);
    try {
      const [showsResult, settingsResult, episodesResult] = await Promise.all([
        getShows(),
        settings ? Promise.resolve(settings) : getSettings(),
        getEpisodes(showId),
      ]) as [Show[], Settings, Episode[]];

      if (!settings) setSettings(settingsResult);

      const foundShow = (showsResult || []).find((item) => String(item.id) === String(showId));
      if (!foundShow) {
        addNotification('Anime not found', 'error');
        navigate('/shows');
        return;
      }
      setShow(foundShow);
      setEpisodes(episodesResult || []);

      // AniList artwork: use cached mapping URLs if available, else fetch from API
      if (settingsResult?.enable_anilist_artwork) {
        if (foundShow.anilist_banner_url || foundShow.anilist_poster_url) {
          setArtwork({
            banner_image: foundShow.anilist_banner_url || '',
            poster_image: foundShow.anilist_poster_url || '',
          });
        } else {
          getAniListArtwork(showId)
            .then((art) => {
              setArtwork(art?.banner_image || art?.poster_image ? art : null);
            })
            .catch(() => setArtwork(null));
        }
      } else {
        setArtwork(null);
      }

      setTrackFiller(actionToTracked(foundShow.filler_action, false));
      setTrackMixed(actionToTracked(foundShow.mixed_action, true));
      setTrackAnimeCanon(actionToTracked(foundShow.anime_canon_action, true));
      setWatermarkAuto((foundShow.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO);
      setMarkerValue(clampMarkerValue(foundShow.sync_watermark || 0, episodesResult || []));
      setPristineFiller(actionToTracked(foundShow.filler_action, false));
      setPristineMixed(actionToTracked(foundShow.mixed_action, true));
      setPristineAnimeCanon(actionToTracked(foundShow.anime_canon_action, true));
      setPristineWatermarkAuto((foundShow.watermark_mode || WATERMARK_AUTO) === WATERMARK_AUTO);
      setPristineMarkerValue(clampMarkerValue(foundShow.sync_watermark || 0, episodesResult || []));

      getSonarrSeries()
        .then((all: SonarrSeries[]) => {
          setSeries((all || []).find((s) => s.id === foundShow.sonarr_series_id) || null);
        })
        .catch(() => {});
    } catch (err: any) {
      addNotification('Failed to load anime: ' + err.message, 'error');
    }
    setLoading(false);
  }, [showId, settings, setSettings, addNotification, navigate]);

  useEffect(() => {
    loadShow();
  }, [loadShow]);

  useEffect(() => {
    const updateViewport = () => setIsMobile(window.innerWidth <= 600);
    updateViewport();
    window.addEventListener('resize', updateViewport);
    return () => window.removeEventListener('resize', updateViewport);
  }, []);

  useEffect(() => {
    if (isMobile && activeTypeFilter === 'anime_canon') {
      setActiveTypeFilter('canon');
    }
  }, [isMobile, activeTypeFilter]);

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
      addNotification(`Saved tracking rules for ${show?.sonarr_title || 'anime'}`, 'success');
      setSavedFlash(true);
      setTimeout(() => setSavedFlash(false), 1500);
      await loadShow();
    } catch (err: any) {
      addNotification('Failed to save anime: ' + err.message, 'error');
    }
    setSaving(false);
  };

  const handleDelete = async () => {
    if (!showId) return;
    setDeleting(true);
    try {
      await deleteShow(showId);
      addNotification(`Removed ${show?.sonarr_title || 'anime'} from Mizu`, 'success');
      setShowDeleteDialog(false);
      navigate('/shows');
    } catch (err: any) {
      addNotification('Failed to remove anime: ' + err.message, 'error');
    }
    setDeleting(false);
  };

  const toggleEpisodeMonitor = async (ep: Episode) => {
    const sonarrEp = resolveSonarrEpisode(ep);
    if (!sonarrEp?.id || !showId) {
      addNotification('No Sonarr episode found for ' + episodeLabel(ep), 'error');
      return;
    }
    setSyncingEps((prev) => ({ ...prev, [sonarrEp.id]: true }));
    try {
      const nextState = !sonarrEp.monitored;
      await setEpisodeMonitor(showId, sonarrEp.id, nextState);
      setEpisodes((prev) => prev.map((e) =>
        e.sonarr_episode_id === sonarrEp.id ? { ...e, monitored: nextState } : e
      ));
      addNotification(`${episodeLabel(ep)} ${nextState ? 'now monitored' : 'now unmonitored'}`, 'success');
    } catch (err: any) {
      addNotification('Failed to update episode: ' + err.message, 'error');
    }
    setSyncingEps((prev) => { const next = { ...prev }; delete next[sonarrEp.id]; return next; });
  };

  const backdropUrl = (): string => {
    // Prefer AniList banner when available
    if (artwork?.banner_image) return artwork.banner_image;
    if (artwork?.poster_image) return artwork.poster_image;
    // Fall back to Sonarr images
    if (!series?.images?.length) return '';
    const preferred = series.images.find((img) => img.coverType === 'fanart') || series.images.find((img) => img.coverType === 'banner') || series.images[0];
    if (!preferred) return '';
    return resolveSonarrImageUrl(preferred, settings?.sonarr_url || '');
  };

  if (loading) {
    return (
      <Section padding={4} maxWidth={900} style={{ marginInline: 'auto' }}>
        <VStack gap={3} hAlign="center" padding={6}>
          <Spinner size="lg" label="Loading anime" />
        </VStack>
      </Section>
    );
  }

  if (!show) return null;

  return (
    <>
    <div className="show-shell" style={{
      position: 'relative',
      width: 'calc(100% - clamp(24px, 8vw, 64px))',
      maxWidth: 'calc(900px - 64px)',
      marginInline: 'auto',
      marginTop: 'var(--spacing-4)',
      borderRadius: 'var(--radius-container)',
      overflow: 'visible',
      backgroundColor: 'transparent',
    }}>
    {/* Hero header with backdrop — matches content width */}
    <div className="show-hero" style={{
      position: 'relative',
      zIndex: 2,
      isolation: 'isolate',
      minHeight: 'clamp(160px, 30vw, 220px)',
      overflow: 'hidden',
      boxSizing: 'border-box',
      backgroundColor: 'var(--color-background-muted)',
      border: 'none',
      borderRadius: 'var(--radius-container)',
      width: '100%',
      margin: 0,
    }}>
          {backdropUrl() && (
            <>
              <img
                src={backdropUrl()}
                alt=""
                loading="lazy"
                decoding="async"
                style={{
                  position: 'absolute',
                  inset: 0,
                  width: '100%',
                  height: '100%',
                  objectFit: 'cover',
                  borderRadius: 'var(--radius-container)',
                  opacity: 0.65,
                }}
              />
              <div style={{
                position: 'absolute',
                inset: 0,
                borderRadius: 'var(--radius-container)',
                background: 'linear-gradient(to top, color-mix(in srgb, var(--color-background-body) 92%, transparent) 0%, color-mix(in srgb, var(--color-background-body) 45%, transparent) 58%, color-mix(in srgb, var(--color-background-body) 8%, transparent) 100%)',
              }} />
            </>
          )}
          <div style={{
            position: 'relative',
            zIndex: 1,
            minHeight: 'clamp(160px, 30vw, 220px)',
            boxSizing: 'border-box',
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'flex-end',
            padding: 'clamp(20px, 5vw, 32px) clamp(12px, 4vw, 32px)',
          }}>
            <VStack gap={4}>
              <VStack gap={1} style={{ minWidth: 0 }}>
                <Heading level={1} style={{ color: 'var(--color-text-primary)' }}>{show.sonarr_title}</Heading>
                <Text type="supporting" color="secondary" hasTabularNumbers display="block">
                  {episodes.length} episodes · from ep {show.sync_watermark > 0 ? show.sync_watermark : 0}
                </Text>
                {series?.overview && (
                  <Text type="supporting" color="secondary" display="block" style={{
                    marginTop: 8,
                    maxWidth: 640,
                    display: '-webkit-box',
                    WebkitLineClamp: 3,
                    WebkitBoxOrient: 'vertical',
                    overflow: 'hidden',
                  }}>
                    {series.overview}
                  </Text>
                )}
              </VStack>
            </VStack>
          </div>
        </div>

    <div className="controls-reveal">
    <div className="controls-reveal-inner">
    <div className="show-actions-bar" style={{
      position: 'relative',
      zIndex: 1,
      width: '100%',
      marginBottom: 0,
      padding: 'calc(var(--radius-container) + var(--spacing-2)) var(--spacing-3) var(--spacing-2)',
      display: 'flex',
      alignItems: 'flex-end',
      justifyContent: 'space-between',
      gap: 'var(--spacing-6)',
      flexWrap: 'nowrap',
      boxSizing: 'border-box',
      backgroundColor: 'var(--color-background-muted)',
      borderRadius: '0 0 var(--radius-container) var(--radius-container)',
    }}>
      <HStack gap={3} align="center" wrap="nowrap">
        <InputGroup label="Episode" isLabelHidden size="md" isDisabled={watermarkAuto} className="show-actions-bar-center">
          <InputGroupText style={{ padding: 0 }}>
            <Button
              label="Decrease episode"
              variant="ghost"
              size="md"
              isIconOnly
              icon={<Minus size={16} />}
              style={{ width: 36, height: 36, minWidth: 36, padding: 0 }}
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
            max={episodes.length > 0 ? episodes.reduce((h, ep) => Math.max(h, displayEpisodeNumber(ep)), 0) : null}
            step={1}
            isIntegerOnly
            isDisabled={watermarkAuto}
          />
          <InputGroupText style={{ padding: 0 }}>
            <Button
              label="Increase episode"
              variant="ghost"
              size="md"
              isIconOnly
              icon={<Plus size={16} />}
              style={{ width: 36, height: 36, minWidth: 36, padding: 0 }}
              isDisabled={watermarkAuto}
              onClick={() => setMarkerValue(clampMarkerValue(markerValue + 1))}
            />
          </InputGroupText>
        </InputGroup>
        <Tooltip content={watermarkAuto ? 'Using Sonarr progress' : 'Using manual episode'} placement="right">
          <Button
            label={watermarkAuto ? 'Auto' : 'Manual'}
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
          label={savedFlash ? 'Saved' : saving ? 'Saving...' : 'Save'}
          variant="primary"
          size="md"
          icon={savedFlash ? <Check size={14} /> : <Save size={14} />}
          isIconOnly={isMobile}
          onClick={handleSave}
          isDisabled={saving || !rulesDirty}
          isLoading={saving}
        />
      </HStack>
    </div>
    </div>
    </div>
    </div>

    {/* Page content — constrained width */}
    <Section padding={0} maxWidth={900} style={{ marginInline: 'auto' }}>
        <VStack gap={6} padding={4} style={{ paddingInline: 'clamp(12px, 4vw, 32px)' }}>
          {/* Tracking rules */}
          <VStack gap={4}>
            <HStack gap={3} align="center">
              <Heading level={2}>Tracking rules</Heading>
              <div style={{ flex: 1, height: 1, background: 'var(--color-border)' }} />
              {rulesDirty && <Badge variant="warning" label="Unsaved changes" />}
            </HStack>


            <Grid columns={{minWidth: 240}} gap={4} align="start">
              <Card padding={4} variant="green">
                <HStack hAlign="between" vAlign="center" wrap="nowrap">
                  <VStack gap={0} style={{ minWidth: 100 }}>
                    <Text type="body" weight="semibold" display="block">Canon</Text>
                    <Text type="supporting" color="secondary" display="block">Main plot episodes</Text>
                  </VStack>
                  <Switch
                    label="Track canon"
                    value={trackAnimeCanon}
                    onChange={setTrackAnimeCanon}
                    isLabelHidden
                    style={{ '--color-accent': 'var(--color-border-green)' } as React.CSSProperties}
                  />
                </HStack>
              </Card>

              <Card padding={4} variant="yellow">
                <HStack hAlign="between" vAlign="center" wrap="nowrap">
                  <VStack gap={0} style={{ minWidth: 100 }}>
                    <Text type="body" weight="semibold" display="block">Mixed</Text>
                    <Text type="supporting" color="secondary" display="block">Mix of canon and filler</Text>
                  </VStack>
                  <Switch
                    label="Track mixed"
                    value={trackMixed}
                    onChange={setTrackMixed}
                    isLabelHidden
                    style={{ '--color-accent': 'var(--color-border-yellow)' } as React.CSSProperties}
                  />
                </HStack>
              </Card>

              <Card padding={4} variant="red">
                <HStack hAlign="between" vAlign="center" wrap="nowrap">
                  <VStack gap={0} style={{ minWidth: 100 }}>
                    <Text type="body" weight="semibold" display="block">Filler</Text>
                    <Text type="supporting" color="secondary" display="block">Skippable, no plot impact</Text>
                  </VStack>
                  <Switch
                    label="Track filler"
                    value={trackFiller}
                    onChange={setTrackFiller}
                    isLabelHidden
                    style={{ '--color-accent': 'var(--color-border-red)' } as React.CSSProperties}
                  />
                </HStack>
              </Card>
            </Grid>
          </VStack>

          {/* Episode list */}
          <VStack gap={4}>
            <HStack gap={3} align="center">
              <Heading level={2}>Episode list</Heading>
              <div style={{ flex: 1, height: 1, background: 'var(--color-border)' }} />
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
              <SegmentedControlItem value="all" label="All" />
              <SegmentedControlItem
                value="canon"
                label="Canon"
                icon={<BookOpen size={16} />}
              />
              <SegmentedControlItem
                value="mixed_canon"
                label="Mixed"
                icon={<Shuffle size={16} />}
              />
              <SegmentedControlItem
                value="filler"
                label="Filler"
                icon={<SkipForward size={16} />}
              />
            </SegmentedControl>
          </div>

          {/* Episodes by season */}
          {filteredEpisodes.length === 0 ? (
            <EmptyState
              title="No episodes"
              description="No episodes match the selected type."
              icon={<Icon icon={CircleAlert} size="lg" color="secondary" />}
              actions={<Button label="Show all" variant="secondary" onClick={() => setActiveTypeFilter('all')} />}
            />
          ) : (
            <VStack gap={4}>
              {groupedBySeason.map((group) => (
                <Card key={group.season} padding={4} variant="muted">
                  <Collapsible
                    trigger={
                      <>
                        <span>{seasonLabel(group.season)}</span>
                        <span style={{ color: 'var(--color-text-secondary)', fontSize: 'var(--font-size-sm)' }}>
                          {group.episodes.length} episodes
                        </span>
                      </>
                    }
                    value={String(group.season)}
                    defaultIsOpen={group.index === groupedBySeason.length - 1}
                  >
                    <div style={{paddingBlockStart: 'var(--spacing-4)'}}>
                    <List density="balanced" hasDividers>
                      {group.episodes.map((ep) => {
                        const typeBadge = episodeTypeBadge(ep.type);
                        const epId = resolveSonarrEpisode(ep)?.id ?? 0;
                        const isSyncing = syncingEps[epId];
                        const monitored = episodeMonitored(ep);
                        return (
                          <ListItem
                            key={`${group.season}-${displayEpisodeNumber(ep)}`}
                            label={ep.title || '—'}
                            description={`Episode ${displayEpisodeNumber(ep)}`}
                            endContent={
                              <HStack gap={4} align="center">
                                <Badge variant={badgeVariantForType(ep.type)} label={typeBadge.label} />
                                <ToggleButton
                                  label={monitored ? 'Unmonitor episode' : 'Monitor episode'}
                                  isIconOnly
                                  size="sm"
                                  icon={<Eye size={14} />}
                                  pressedIcon={<EyeOff size={14} />}
                                  isPressed={monitored}
                                  isDisabled={isSyncing || !epId}
                                  isLoading={isSyncing}
                                  tooltip={monitored ? 'Unmonitor' : 'Monitor'}
                                  onPressedChange={() => toggleEpisodeMonitor(ep)}
                                />
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
