import { useEffect, useState, useCallback, useRef } from 'react';
import { VStack } from '@astryxdesign/core/VStack';
import { HStack } from '@astryxdesign/core/HStack';
import { Section } from '@astryxdesign/core/Section';
import { Heading } from '@astryxdesign/core/Heading';
import { Text } from '@astryxdesign/core/Text';
import { Button } from '@astryxdesign/core/Button';
import { Badge } from '@astryxdesign/core/Badge';
import { Spinner } from '@astryxdesign/core/Spinner';
import { TextInput } from '@astryxdesign/core/TextInput';
import { Switch } from '@astryxdesign/core/Switch';
import { Divider } from '@astryxdesign/core/Divider';
import { Save, Check, Plug, AlertTriangle, RefreshCw, Eye, EyeOff, Minus, Plus } from 'lucide-react';
import { useAppStore } from '../lib/store';
import {
  getSettings,
  getStoredSonarrApiKey,
  refreshAflCache,
  testSonarrConnection,
  updateSettings,
} from '../lib/api';
import type { Settings as SettingsType, SelectOption } from '../lib/types';

const actionToTracked = (action: string | undefined, fallback = true): boolean =>
  (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
const trackedToAction = (tracked: boolean): string => (tracked ? 'monitor' : 'unmonitor');

const syncIntervalOptions: SelectOption[] = [
  { value: '1h', label: '1 hour' },
  { value: '3h', label: '3 hours' },
  { value: '6h', label: '6 hours' },
  { value: '12h', label: '12 hours' },
  { value: '24h', label: '24 hours' },
];

const sleep = (ms: number): Promise<void> => new Promise((r) => setTimeout(r, ms));

interface PristineSnapshot {
  sonarrUrl: string;
  apiKeyDirty: boolean;
  syncInterval: string;
  enableAniListArtwork: boolean;
  trackFillerDefault: boolean;
  trackMixedDefault: boolean;
  trackAnimeCanonDefault: boolean;
}

export function SettingsPage() {
  const { setSettings, addNotification } = useAppStore();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [refreshingCache, setRefreshingCache] = useState(false);
  const [revealingKey, setRevealingKey] = useState(false);
  const [testPhase, setTestPhase] = useState<'idle' | 'connecting' | 'success' | 'error'>('idle');
  const [testErrorMessage, setTestErrorMessage] = useState('');

  const [sonarrUrl, setSonarrUrl] = useState('');
  const [sonarrApiKey, setSonarrApiKey] = useState('');
  const [savedApiKeyMasked, setSavedApiKeyMasked] = useState('');
  const [hasSavedApiKey, setHasSavedApiKey] = useState(false);
  const [apiKeyVisible, setApiKeyVisible] = useState(false);
  const [apiKeyDirty, setApiKeyDirty] = useState(false);
  const [syncInterval, setSyncInterval] = useState('6h');
  const [enableAniListArtwork, setEnableAniListArtwork] = useState(false);
  const [trackFillerDefault, setTrackFillerDefault] = useState(false);
  const [trackMixedDefault, setTrackMixedDefault] = useState(true);
  const [trackAnimeCanonDefault, setTrackAnimeCanonDefault] = useState(true);
  const [cacheRefreshedAt, setCacheRefreshedAt] = useState<string | null>(null);
  const [cacheCount, setCacheCount] = useState(0);

  const [pristine, setPristine] = useState<PristineSnapshot>({
    sonarrUrl: '',
    apiKeyDirty: false,
    syncInterval: '6h',
    enableAniListArtwork: false,
    trackFillerDefault: false,
    trackMixedDefault: true,
    trackAnimeCanonDefault: true,
  });

  const resetTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const settingsDirty = !loading && (
    sonarrUrl !== pristine.sonarrUrl ||
    apiKeyDirty ||
    syncInterval !== pristine.syncInterval ||
    enableAniListArtwork !== pristine.enableAniListArtwork ||
    trackFillerDefault !== pristine.trackFillerDefault ||
    trackMixedDefault !== pristine.trackMixedDefault ||
    trackAnimeCanonDefault !== pristine.trackAnimeCanonDefault
  );

  const urlValid = !sonarrUrl || /^https?:\/\/.+/.test(sonarrUrl);
  const syncIntervalIndex = syncIntervalOptions.findIndex((o) => o.value === syncInterval);

  const scheduleReset = (delay = 1800) => {
    if (resetTimer.current) clearTimeout(resetTimer.current);
    resetTimer.current = setTimeout(() => {
      setTestPhase('idle');
      setTestErrorMessage('');
    }, delay);
  };

  const clearReset = () => {
    if (resetTimer.current) clearTimeout(resetTimer.current);
    resetTimer.current = null;
  };

  const clearTestResult = () => {
    clearReset();
    if (testPhase === 'success' || testPhase === 'error') {
      setTestPhase('idle');
      setTestErrorMessage('');
    }
  };

  useEffect(() => {
    return () => { if (resetTimer.current) clearTimeout(resetTimer.current); };
  }, []);

  const loadSettings = useCallback(async () => {
    setLoading(true);
    try {
      const data = await getSettings();
      if (data && !data.error) {
        setSonarrUrl(data.sonarr_url || '');
        setSavedApiKeyMasked(data.sonarr_api_key_masked || '');
        setHasSavedApiKey(!!data.sonarr_api_key_set);
        setSonarrApiKey(data.sonarr_api_key_masked || '');
        setApiKeyVisible(false);
        setApiKeyDirty(false);
        setSyncInterval(data.sync_interval || '6h');
        setEnableAniListArtwork(data.enable_anilist_artwork || false);
        setTrackFillerDefault(actionToTracked(data.default_filler_action, false));
        setTrackMixedDefault(actionToTracked(data.default_mixed_canon_action, true));
        setTrackAnimeCanonDefault(actionToTracked(data.default_anime_canon_action, true));
        setCacheRefreshedAt(data.afl_show_cache_refreshed_at || null);
        setCacheCount(data.afl_show_cache_count || 0);
        setSettings(data);
        setPristine({
          sonarrUrl: data.sonarr_url || '',
          apiKeyDirty: false,
          syncInterval: data.sync_interval || '6h',
          enableAniListArtwork: data.enable_anilist_artwork || false,
          trackFillerDefault: actionToTracked(data.default_filler_action, false),
          trackMixedDefault: actionToTracked(data.default_mixed_canon_action, true),
          trackAnimeCanonDefault: actionToTracked(data.default_anime_canon_action, true),
        });
      }
    } catch (err: any) {
      addNotification('Failed to load settings: ' + err.message, 'error');
    }
    setLoading(false);
  }, [setSettings, addNotification]);

  useEffect(() => {
    loadSettings();
  }, [loadSettings]);

  const handleApiKeyInput = (value: string) => {
    setApiKeyDirty(true);
    setSonarrApiKey(value);
    clearTestResult();
  };

  const handleUrlInput = (value: string) => {
    setSonarrUrl(value);
    clearTestResult();
  };

  const toggleApiKeyVisibility = async () => {
    if (apiKeyVisible) {
      setApiKeyVisible(false);
      if (!apiKeyDirty && hasSavedApiKey) {
        setSonarrApiKey(savedApiKeyMasked);
      }
      return;
    }
    if (!apiKeyDirty && hasSavedApiKey) {
      setRevealingKey(true);
      try {
        const data = await getStoredSonarrApiKey();
        setSonarrApiKey(data.sonarr_api_key || '');
      } catch (err: any) {
        addNotification('Failed to reveal saved API key: ' + err.message, 'error');
        setRevealingKey(false);
        return;
      }
      setRevealingKey(false);
    }
    setApiKeyVisible(true);
  };

  const resolveApiKeyForTest = async (): Promise<string> => {
    if (apiKeyDirty || apiKeyVisible || !hasSavedApiKey) {
      return sonarrApiKey;
    }
    const data = await getStoredSonarrApiKey();
    return data.sonarr_api_key || '';
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const payload: Partial<SettingsType> = {
        sonarr_url: sonarrUrl,
        default_filler_action: trackedToAction(trackFillerDefault),
        default_mixed_canon_action: trackedToAction(trackMixedDefault),
        default_anime_canon_action: trackedToAction(trackAnimeCanonDefault),
        sync_interval: syncInterval,
        enable_anilist_artwork: enableAniListArtwork,
      };
      if (apiKeyDirty || !hasSavedApiKey) {
        payload.sonarr_api_key = sonarrApiKey;
      }
      await updateSettings(payload);
      addNotification('Settings saved', 'success');
      await loadSettings();
    } catch (err: any) {
      addNotification('Failed to save settings: ' + err.message, 'error');
    }
    setSaving(false);
  };

  const handleTestConnection = async () => {
    if (!sonarrUrl) {
      setTestErrorMessage('Enter a Sonarr URL first');
      setTestPhase('error');
      return;
    }
    if (!urlValid) {
      setTestErrorMessage('URL must start with http:// or https://');
      setTestPhase('error');
      return;
    }
    setTesting(true);
    setTestErrorMessage('');
    clearReset();
    setTestPhase('connecting');
    const startTime = Date.now();
    const minDelay = 1100;
    try {
      const apiKey = await resolveApiKeyForTest();
      const result = await testSonarrConnection(sonarrUrl, apiKey);
      const elapsed = Date.now() - startTime;
      if (elapsed < minDelay) await sleep(minDelay - elapsed);
      if (!result?.success) {
        throw new Error(result?.error || 'Connection failed');
      }
      setTestPhase('success');
      scheduleReset(2000);
    } catch (err: any) {
      const elapsed = Date.now() - startTime;
      if (elapsed < minDelay) await sleep(minDelay - elapsed);
      setTestErrorMessage(err.message);
      setTestPhase('error');
      scheduleReset(4000);
    }
    setTesting(false);
  };

  const handleRefreshCache = async () => {
    setRefreshingCache(true);
    try {
      await refreshAflCache();
      addNotification('AFL show list refreshed', 'success');
      await loadSettings();
    } catch (err: any) {
      addNotification('Failed to refresh AFL cache: ' + err.message, 'error');
    }
    setRefreshingCache(false);
  };

  const stepSyncInterval = (dir: number) => {
    const next = Math.min(Math.max(syncIntervalIndex + dir, 0), syncIntervalOptions.length - 1);
    setSyncInterval(syncIntervalOptions[next].value);
  };

  const formatCacheTime = (value: string | null | undefined): string => {
    if (!value) return 'Never refreshed';
    return new Date(value).toLocaleString();
  };

  if (loading) {
    return (
      <Section padding={6}>
        <VStack gap={3} hAlign="center" padding={10}>
          <Spinner size="lg" label="Loading settings" />
        </VStack>
      </Section>
    );
  }

  return (
    <Section padding={6}>
      <VStack gap={6}>
        {/* Header */}
        <HStack justify="between" align="start" wrap="wrap">
          <VStack gap={1}>
            <Heading level={1}>Settings</Heading>
            <Text type="supporting">Configure Sonarr connection, sync behavior, and defaults.</Text>
          </VStack>
          <HStack gap={3} align="center">
            {settingsDirty && <Badge variant="warning" label="Unsaved changes" />}
            <Button
              label={saving ? 'Saving...' : 'Save settings'}
              variant="primary"
              icon={<Save size={16} />}
              onClick={handleSave}
              isDisabled={saving || !settingsDirty}
              isLoading={saving}
            />
          </HStack>
        </HStack>

        {/* Connection group */}
        <Section variant="muted" padding={5}>
          <VStack gap={0}>
            <Heading level={3}>Connection</Heading>
            <Divider />

            {/* Sonarr URL */}
            <HStack justify="between" align="center" wrap="wrap" paddingBlock={3}>
              <VStack gap={0}>
                <Text type="body" weight="medium">Sonarr URL</Text>
                <Text type="supporting">Base URL of your Sonarr instance.</Text>
                {sonarrUrl && !urlValid && <Text type="supporting" color="accent">URL must start with http:// or https://</Text>}
              </VStack>
              <TextInput
                label="Sonarr URL"
                value={sonarrUrl}
                onChange={handleUrlInput}
                placeholder="http://localhost:8989"
                isLabelHidden
                width={256}
              />
            </HStack>

            <Divider />

            {/* API key */}
            <HStack justify="between" align="center" wrap="wrap" paddingBlock={3}>
              <VStack gap={0}>
                <Text type="body" weight="medium">API key</Text>
                <Text type="supporting">Found in Sonarr under Settings, General.</Text>
              </VStack>
              <HStack gap={2} align="center">
                <TextInput
                  label="API key"
                  type={apiKeyVisible ? 'text' : 'password'}
                  value={sonarrApiKey}
                  onChange={handleApiKeyInput}
                  placeholder={hasSavedApiKey ? 'Saved API key' : 'Your Sonarr API key'}
                  isLabelHidden
                  width={256}
                />
                {(hasSavedApiKey || apiKeyDirty) && (
                  <Button
                    label={apiKeyVisible ? 'Hide' : 'Show'}
                    variant="ghost"
                    isIconOnly
                    icon={apiKeyVisible ? <EyeOff size={16} /> : revealingKey ? <Spinner size="sm" /> : <Eye size={16} />}
                    onClick={toggleApiKeyVisibility}
                    isDisabled={revealingKey}
                  />
                )}
              </HStack>
            </HStack>

            <Divider />

            {/* Test connection */}
            <HStack justify="between" align="center" wrap="wrap" paddingBlock={3}>
              <VStack gap={0}>
                <Text type="body" weight="medium">Test connection</Text>
                <Text type="supporting">Check that Mizu can reach Sonarr with the URL and API key above.</Text>
                {testPhase === 'error' && testErrorMessage && (
                  <HStack gap={2} align="start" style={{ color: 'var(--color-error)' }}>
                    <AlertTriangle size={14} />
                    <Text type="supporting">{testErrorMessage}</Text>
                  </HStack>
                )}
              </VStack>
              <Button
                label={
                  testPhase === 'connecting' ? 'Testing' :
                  testPhase === 'success' ? 'Connected' :
                  testPhase === 'error' ? 'Retry' : 'Test'
                }
                variant={
                  testPhase === 'success' ? 'primary' :
                  testPhase === 'error' ? 'destructive' : 'secondary'
                }
                icon={
                  testPhase === 'connecting' ? <Spinner size="sm" /> :
                  testPhase === 'success' ? <Check size={16} /> :
                  testPhase === 'error' ? <AlertTriangle size={16} /> : <Plug size={16} />
                }
                onClick={handleTestConnection}
                isDisabled={testing || testPhase === 'success' || !sonarrUrl || !urlValid}
                isLoading={testing}
              />
            </HStack>
          </VStack>
        </Section>

        {/* Sync group */}
        <Section variant="muted" padding={5}>
          <VStack gap={0}>
            <Heading level={3}>Sync</Heading>
            <Divider />

            {/* Sync interval */}
            <HStack justify="between" align="center" wrap="wrap" paddingBlock={3}>
              <VStack gap={0}>
                <Text type="body" weight="medium">Sync interval</Text>
                <Text type="supporting">How often Mizu checks for episode changes and updates Sonarr.</Text>
              </VStack>
              <HStack gap={0}>
                <Button
                  label="Decrease"
                  variant="secondary"
                  isIconOnly
                  icon={<Minus size={16} />}
                  onClick={() => stepSyncInterval(-1)}
                  isDisabled={syncIntervalIndex <= 0}
                />
                <Section variant="muted" padding={3} style={{ minWidth: 80, textAlign: 'center' }}>
                  <Text type="body" hasTabularNumbers>{syncIntervalOptions[syncIntervalIndex]?.label}</Text>
                </Section>
                <Button
                  label="Increase"
                  variant="secondary"
                  isIconOnly
                  icon={<Plus size={16} />}
                  onClick={() => stepSyncInterval(1)}
                  isDisabled={syncIntervalIndex >= syncIntervalOptions.length - 1}
                />
              </HStack>
            </HStack>

            <Divider />

            {/* AniList artwork */}
            <HStack justify="between" align="center" wrap="wrap" paddingBlock={3}>
              <VStack gap={0}>
                <Text type="body" weight="medium">AniList artwork</Text>
                <Text type="supporting">Use AniList banners and posters instead of Sonarr images only.</Text>
              </VStack>
              <Switch
                label="AniList artwork"
                value={enableAniListArtwork}
                onChange={setEnableAniListArtwork}
                isLabelHidden
              />
            </HStack>

            <Divider />

            {/* AFL cache */}
            <HStack justify="between" align="center" wrap="wrap" paddingBlock={3}>
              <VStack gap={0}>
                <Text type="body" weight="medium">AFL cache</Text>
                <Text type="supporting" hasTabularNumbers>
                  {cacheCount} shows cached, refreshed {formatCacheTime(cacheRefreshedAt)}.
                </Text>
              </VStack>
              <Button
                label={refreshingCache ? 'Refreshing...' : 'Refresh'}
                variant="secondary"
                icon={<RefreshCw size={16} />}
                onClick={handleRefreshCache}
                isLoading={refreshingCache}
              />
            </HStack>
          </VStack>
        </Section>

        {/* Defaults group */}
        <Section variant="muted" padding={5}>
          <VStack gap={0}>
            <Heading level={3}>Defaults</Heading>
            <Text type="supporting">These tracking rules are applied automatically when you add a new anime mapping. You can override them per anime.</Text>
            <Divider />

            <HStack justify="between" align="center" wrap="wrap" paddingBlock={3}>
              <VStack gap={0}>
                <Text type="body" weight="medium">Anime canon</Text>
                <Text type="supporting">Original anime episodes</Text>
              </VStack>
              <Switch label="Track anime canon" value={trackAnimeCanonDefault} onChange={setTrackAnimeCanonDefault} isLabelHidden />
            </HStack>

            <Divider />

            <HStack justify="between" align="center" wrap="wrap" paddingBlock={3}>
              <VStack gap={0}>
                <Text type="body" weight="medium">Mixed canon</Text>
                <Text type="supporting">Episodes that mix canon and filler content</Text>
              </VStack>
              <Switch label="Track mixed canon" value={trackMixedDefault} onChange={setTrackMixedDefault} isLabelHidden />
            </HStack>

            <Divider />

            <HStack justify="between" align="center" wrap="wrap" paddingBlock={3}>
              <VStack gap={0}>
                <Text type="body" weight="medium">Filler</Text>
                <Text type="supporting">Anime-original filler episodes</Text>
              </VStack>
              <Switch label="Track filler" value={trackFillerDefault} onChange={setTrackFillerDefault} isLabelHidden />
            </HStack>
          </VStack>
        </Section>
      </VStack>
    </Section>
  );
}
