import { useEffect, useState, useCallback, useRef, type CSSProperties } from 'react';
import { VStack } from '@astryxdesign/core/VStack';
import { HStack } from '@astryxdesign/core/HStack';
import { Section } from '@astryxdesign/core/Section';
import { Grid } from '@astryxdesign/core/Grid';
import { Heading } from '@astryxdesign/core/Heading';
import { Text } from '@astryxdesign/core/Text';
import { Button } from '@astryxdesign/core/Button';
import { Badge } from '@astryxdesign/core/Badge';
import { Spinner } from '@astryxdesign/core/Spinner';
import { TextInput } from '@astryxdesign/core/TextInput';
import { Switch } from '@astryxdesign/core/Switch';
import { Selector } from '@astryxdesign/core/Selector';
import { Divider } from '@astryxdesign/core/Divider';
import { Card } from '@astryxdesign/core/Card';
import { Save, Check, Plug, AlertTriangle, RefreshCw, Eye, EyeOff } from 'lucide-react';
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

const rowPadding: CSSProperties = {
  paddingBlock: 'var(--spacing-4)',
};

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
  const [, setTesting] = useState(false);
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

  const formatCacheTime = (value: string | null | undefined): string => {
    if (!value) return 'Never refreshed';
    return new Date(value).toLocaleString();
  };

  if (loading) {
    return (
      <Section padding={4} maxWidth={900} style={{ marginInline: 'auto' }}>
        <VStack gap={3} hAlign="center" padding={6}>
          <Spinner size="lg" label="Loading settings" />
        </VStack>
      </Section>
    );
  }

  return (
    <Section padding={4} maxWidth={900} style={{ marginInline: 'auto' }}>
      <VStack gap={8}>
        {/* Header */}
        <HStack justify="between" align="center" wrap="wrap" gap={4}>
          <VStack gap={1}>
            <Heading level={1}>Settings</Heading>
            <Text type="supporting" color="secondary">
              Configure Sonarr connection, sync behavior, and defaults.
            </Text>
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

        {/* Connection */}
        <VStack gap={0}>
          <Heading level={2}>Connection</Heading>
          <Divider />

          <HStack hAlign="between" vAlign="center" style={rowPadding} wrap="wrap">
            <VStack gap={0}>
              <Text type="body" weight="semibold" display="block">Sonarr URL</Text>
              <Text type="supporting" color="secondary" display="block">Base URL of your Sonarr instance.</Text>
              {sonarrUrl && !urlValid && (
                <Text type="supporting" display="block" style={{ color: 'var(--color-error)' }}>URL must start with http:// or https://</Text>
              )}
            </VStack>
            <TextInput
              label="Sonarr URL"
              value={sonarrUrl}
              onChange={handleUrlInput}
              placeholder="http://localhost:8989"
              isLabelHidden
              width={320}
            />
          </HStack>
          <Divider />

          <HStack hAlign="between" vAlign="center" style={rowPadding} wrap="wrap">
            <VStack gap={0}>
              <Text type="body" weight="semibold" display="block">API key</Text>
              <Text type="supporting" color="secondary" display="block">Found in Sonarr under Settings, General.</Text>
            </VStack>
            <HStack gap={2} align="center">
              <TextInput
                label="API key"
                type={apiKeyVisible ? 'text' : 'password'}
                value={sonarrApiKey}
                onChange={handleApiKeyInput}
                placeholder={hasSavedApiKey ? 'Saved API key' : 'Your Sonarr API key'}
                isLabelHidden
                width={280}
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

          <HStack hAlign="between" vAlign="center" style={rowPadding} wrap="wrap">
            <VStack gap={0}>
              <Text type="body" weight="semibold" display="block">Test connection</Text>
              <Text type="supporting" color="secondary" display="block">Check that Mizu can reach Sonarr with the URL and API key above.</Text>
              {testPhase === 'error' && testErrorMessage && (
                <Text type="supporting" display="block" style={{ color: 'var(--color-error)' }}>{testErrorMessage}</Text>
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
              isDisabled={testPhase !== 'idle' || !sonarrUrl || !urlValid}
              style={testPhase === 'success' ? {
                background: 'var(--color-success)',
                borderColor: 'var(--color-success)',
                color: 'var(--color-on-success)',
                opacity: 1,
                cursor: 'default',
              } : testPhase === 'error' ? {
                opacity: 1,
                cursor: 'default',
              } : testPhase === 'connecting' ? {
                opacity: 1,
                cursor: 'default',
              } : undefined}
            />
          </HStack>
        </VStack>

        {/* Sync */}
        <VStack gap={0}>
          <Heading level={2}>Sync</Heading>
          <Divider />

          <HStack hAlign="between" vAlign="center" style={rowPadding} wrap="wrap">
            <VStack gap={0}>
              <Text type="body" weight="semibold" display="block">Sync interval</Text>
              <Text type="supporting" color="secondary" display="block">How often Mizu checks for episode changes and updates Sonarr.</Text>
            </VStack>
            <Selector
              label="Sync interval"
              value={syncInterval}
              options={syncIntervalOptions}
              onChange={(v) => setSyncInterval(v as string)}
              isLabelHidden
              width={180}
            />
          </HStack>
          <Divider />

          <HStack hAlign="between" vAlign="center" style={rowPadding} wrap="wrap">
            <VStack gap={0}>
              <Text type="body" weight="semibold" display="block">AniList artwork</Text>
              <Text type="supporting" color="secondary" display="block">Use AniList banners and posters instead of Sonarr images only.</Text>
            </VStack>
            <Switch
              label="AniList artwork"
              value={enableAniListArtwork}
              onChange={setEnableAniListArtwork}
              isLabelHidden
            />
          </HStack>
          <Divider />

          <HStack hAlign="between" vAlign="center" style={rowPadding} wrap="wrap">
            <VStack gap={0}>
              <Text type="body" weight="semibold" display="block">AFL cache</Text>
              <Text type="supporting" color="secondary" hasTabularNumbers display="block">
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

        {/* Defaults */}
        <VStack gap={4}>
          <VStack gap={1}>
            <Heading level={2}>Defaults</Heading>
            <Text type="supporting" color="secondary" display="block">
              These tracking rules are applied automatically when you add a new anime mapping. You can override them per anime.
            </Text>
          </VStack>

          <Grid columns={3} gap={4} align="start">
            <Card padding={4} variant="green">
              <HStack hAlign="between" vAlign="center" wrap="nowrap">
                <VStack gap={0} style={{ minWidth: 120 }}>
                  <Text type="body" weight="semibold" display="block">Canon</Text>
                  <Text type="supporting" color="secondary" display="block">Main plot episodes</Text>
                </VStack>
                <Switch
                  label="Track canon"
                  value={trackAnimeCanonDefault}
                  onChange={setTrackAnimeCanonDefault}
                  isLabelHidden
                  style={{ '--color-accent': 'var(--color-border-green)', '--color-background-gray': 'var(--color-background-green)' } as React.CSSProperties}
                />
              </HStack>
            </Card>

            <Card padding={4} variant="yellow">
              <HStack hAlign="between" vAlign="center" wrap="nowrap">
                <VStack gap={0} style={{ minWidth: 120 }}>
                  <Text type="body" weight="semibold" display="block">Mixed</Text>
                  <Text type="supporting" color="secondary" display="block">Mix of canon and filler</Text>
                </VStack>
                <Switch
                  label="Track mixed"
                  value={trackMixedDefault}
                  onChange={setTrackMixedDefault}
                  isLabelHidden
                  style={{ '--color-accent': 'var(--color-border-yellow)', '--color-background-gray': 'var(--color-background-yellow)' } as React.CSSProperties}
                />
              </HStack>
            </Card>

            <Card padding={4} variant="red">
              <HStack hAlign="between" vAlign="center" wrap="nowrap">
                <VStack gap={0} style={{ minWidth: 120 }}>
                  <Text type="body" weight="semibold" display="block">Filler</Text>
                  <Text type="supporting" color="secondary" display="block">Skippable, no plot impact</Text>
                </VStack>
                <Switch
                  label="Track filler"
                  value={trackFillerDefault}
                  onChange={setTrackFillerDefault}
                  isLabelHidden
                  style={{ '--color-accent': 'var(--color-border-red)', '--color-background-gray': 'var(--color-background-red)' } as React.CSSProperties}
                />
              </HStack>
            </Card>
          </Grid>
        </VStack>
      </VStack>
    </Section>
  );
}

