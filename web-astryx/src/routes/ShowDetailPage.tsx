import { useEffect, useState, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { VStack } from '@astryxdesign/core/VStack';
import { HStack } from '@astryxdesign/core/HStack';
import { Section } from '@astryxdesign/core/Section';
import { Heading } from '@astryxdesign/core/Heading';
import { Text } from '@astryxdesign/core/Text';
import { Button } from '@astryxdesign/core/Button';
import { Badge } from '@astryxdesign/core/Badge';
import { Spinner } from '@astryxdesign/core/Spinner';
import { EmptyState } from '@astryxdesign/core/EmptyState';
import { Dialog } from '@astryxdesign/core/Dialog';
import { Switch } from '@astryxdesign/core/Switch';
import { List, ListItem } from '@astryxdesign/core/List';
import { Divider } from '@astryxdesign/core/Divider';
import { Trash2, Save, Check, ChevronLeft } from 'lucide-react';
import { useAppStore } from '../lib/store';
import {
  deleteShow,
  getEpisodes,
  getSettings,
  getShows,
  getSonarrSeries,
  setEpisodeMonitor,
  updateShow,
} from '../lib/api';
import type { Episode, Settings, Show, ShowUpdate, SonarrSeries } from '../lib/types';
import { resolveSonarrImageUrl } from '../lib/sonarr-images';
import { episodeTypeBadge, monitorStateBadge } from '../lib/fox';

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

function badgeVariantForType(type: string | undefined | null): 'neutral' | 'info' | 'success' | 'warning' | 'error' | 'blue' | 'purple' | 'teal' {
  switch (type) {
    case 'canon': return 'info';
    case 'anime_canon': return 'purple';
    case 'mixed_canon':
    case 'mixed': return 'warning';
    case 'filler': return 'error';
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
  const [episodes, setEpisodes] = useState<Episode[]>([]);
  const [trackFiller, setTrackFiller] = useState(false);
  const [trackMixed, setTrackMixed] = useState(true);
  const [trackAnimeCanon, setTrackAnimeCanon] = useState(true);
  const [watermarkAuto, setWatermarkAuto] = useState(true);
  const [markerValue, setMarkerValue] = useState(0);
  const [activeTypeFilters, setActiveTypeFilters] = useState<string[]>([]);
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

  const filteredEpisodes = episodes.filter((ep) =>
    activeTypeFilters.length === 0 ? true : (ep.type !== undefined && activeTypeFilters.includes(ep.type))
  );

  const groupedBySeason = (() => {
    const groups: Record<string, Episode[]> = {};
    for (const ep of filteredEpisodes) {
      const season = String(episodeSeason(ep));
      if (!groups[season]) groups[season] = [];
      groups[season].push(ep);
    }
    return Object.entries(groups)
      .map(([seasonKey, eps]) => ({
        season: Number.parseInt(seasonKey, 10),
        episodes: eps.slice().sort((a, b) => displayEpisodeNumber(b) - displayEpisodeNumber(a)),
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
    mixed: episodes.filter((e) => e.type === 'mixed_canon').length,
    animeCanon: episodes.filter((e) => e.type === 'anime_canon').length,
    filler: episodes.filter((e) => e.type === 'filler').length,
  };

  const filterButtons = [
    { type: 'all', label: 'All', count: epStats.total },
    { type: 'canon', label: 'Manga', count: epStats.canon },
    { type: 'anime_canon', label: 'Anime', count: epStats.animeCanon },
    { type: 'mixed_canon', label: 'Mixed', count: epStats.mixed },
    { type: 'filler', label: 'Filler', count: epStats.filler },
  ];

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

  const toggleTypeFilter = (type: string) => {
    if (type === 'all') { setActiveTypeFilters([]); return; }
    setActiveTypeFilters((prev) =>
      prev.includes(type) ? prev.filter((v) => v !== type) : [...prev, type]
    );
  };

  const isTypeFilterActive = (type: string) =>
    type === 'all' ? activeTypeFilters.length === 0 : activeTypeFilters.includes(type);

  const backdropUrl = (): string => {
    if (!series?.images?.length) return '';
    const preferred = series.images.find((img) => img.coverType === 'fanart') || series.images.find((img) => img.coverType === 'banner') || series.images[0];
    if (!preferred) return '';
    return resolveSonarrImageUrl(preferred, settings?.sonarr_url || '');
  };

  if (loading) {
    return (
      <Section padding={6}>
        <VStack gap={3} hAlign="center" padding={10}>
          <Spinner size="lg" label="Loading anime" />
        </VStack>
      </Section>
    );
  }

  if (!show) return null;

  return (
    <Section padding={0}>
      <VStack gap={0}>
        {/* Backdrop header */}
        {backdropUrl() && (
          <div style={{
            position: 'relative',
            height: 200,
            backgroundImage: `url(${backdropUrl()})`,
            backgroundSize: 'cover',
            backgroundPosition: 'center',
          }}>
            <div style={{
              position: 'absolute',
              inset: 0,
              background: 'linear-gradient(to bottom, transparent, var(--color-background-body))',
            }} />
          </div>
        )}

        <VStack gap={6} padding={6}>
          {/* Header */}
          <HStack justify="between" align="start" wrap="wrap">
            <VStack gap={1}>
              <Button
                label="Back"
                variant="ghost"
                icon={<ChevronLeft size={16} />}
                onClick={() => navigate('/shows')}
              />
              <Heading level={1}>{show.sonarr_title}</Heading>
              <Text type="supporting" hasTabularNumbers>
                #{show.sync_watermark > 0 ? show.sync_watermark : 0} · {episodes.length} episodes
              </Text>
            </VStack>
            <HStack gap={2}>
              <Button
                label="Remove"
                variant="destructive"
                icon={<Trash2 size={14} />}
                onClick={() => setShowDeleteDialog(true)}
                isDisabled={deleting}
              />
              <Button
                label={savedFlash ? 'Saved' : 'Save'}
                variant="primary"
                icon={savedFlash ? <Check size={14} /> : <Save size={14} />}
                onClick={handleSave}
                isDisabled={saving || !rulesDirty}
                isLoading={saving}
              />
            </HStack>
          </HStack>

          {/* Tracking rules */}
          <VStack gap={4}>
            <Heading level={2}>Tracking rules</Heading>
            <Switch
              label="Track filler"
              description="Monitor filler episodes in Sonarr"
              value={trackFiller}
              onChange={setTrackFiller}
            />
            <Switch
              label="Track mixed canon"
              description="Monitor mixed canon/filler episodes"
              value={trackMixed}
              onChange={setTrackMixed}
            />
            <Switch
              label="Track anime canon"
              description="Monitor anime-original episodes"
              value={trackAnimeCanon}
              onChange={setTrackAnimeCanon}
            />
            <Divider />
            <HStack gap={3} align="center">
              <Switch
                label="Auto start from Sonarr progress"
                description="Use Sonarr progress as the start point"
                value={watermarkAuto}
                onChange={setWatermarkAuto}
              />
            </HStack>
            {!watermarkAuto && (
              <HStack gap={2} align="center">
                <Text type="label">Start after episode:</Text>
                <input
                  type="number"
                  min={0}
                  max={episodes.length > 0 ? episodes.reduce((h, ep) => Math.max(h, displayEpisodeNumber(ep)), 0) : undefined}
                  step={1}
                  value={markerValue}
                  onChange={(e) => setMarkerValue(clampMarkerValue(Number(e.target.value)))}
                  style={{
                    width: 80,
                    padding: 'var(--spacing-1) var(--spacing-2)',
                    borderRadius: 'var(--radius-2)',
                    border: '1px solid var(--color-border)',
                    backgroundColor: 'var(--color-background-surface)',
                    color: 'var(--color-text-primary)',
                  }}
                />
              </HStack>
            )}
          </VStack>

          <Divider />

          {/* Episode filters */}
          <HStack gap={2} wrap="wrap">
            {filterButtons.map((btn) => (
              <Button
                key={btn.type}
                label={`${btn.label} (${btn.count})`}
                variant={isTypeFilterActive(btn.type) ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => toggleTypeFilter(btn.type)}
              />
            ))}
          </HStack>

          {/* Episodes by season */}
          {filteredEpisodes.length === 0 ? (
            <EmptyState title="No episodes" description="No episodes match the current filters." />
          ) : (
            <VStack gap={6}>
              {groupedBySeason.map((group) => (
                <VStack key={group.season} gap={3}>
                  <Heading level={3}>{seasonLabel(group.season)}</Heading>
                  <List hasDividers>
                    {group.episodes.map((ep) => {
                      const typeBadge = episodeTypeBadge(ep.type);
                      const monBadge = monitorStateBadge(episodeMonitored(ep));
                      const epId = resolveSonarrEpisode(ep)?.id ?? 0;
                      const isSyncing = syncingEps[epId];
                      return (
                        <ListItem
                          key={`${group.season}-${displayEpisodeNumber(ep)}`}
                          label={episodeLabel(ep)}
                          description={ep.title || undefined}
                          onClick={() => toggleEpisodeMonitor(ep)}
                          endContent={
                            <HStack gap={2} align="center">
                              <Badge variant={badgeVariantForType(ep.type)} label={typeBadge.label} />
                              <Badge
                                variant={episodeMonitored(ep) ? 'success' : 'neutral'}
                                label={monBadge.label}
                              />
                              {isSyncing && <Spinner size="sm" />}
                            </HStack>
                          }
                        />
                      );
                    })}
                  </List>
                </VStack>
              ))}
            </VStack>
          )}
        </VStack>
      </VStack>

      {/* Delete dialog */}
      <Dialog
        isOpen={showDeleteDialog}
        onOpenChange={setShowDeleteDialog}
        purpose="required"
        width={400}
      >
        <VStack gap={4} padding={5}>
          <Heading level={2}>Remove {show.sonarr_title}?</Heading>
          <Text type="supporting">This removes the mapping from Mizu. The series stays in Sonarr.</Text>
          <HStack justify="end" gap={2}>
            <Button label="Cancel" variant="secondary" onClick={() => setShowDeleteDialog(false)} isDisabled={deleting} />
            <Button label="Remove" variant="destructive" icon={<Trash2 size={14} />} onClick={handleDelete} isLoading={deleting} />
          </HStack>
        </VStack>
      </Dialog>
    </Section>
  );
}
