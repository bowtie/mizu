import { useEffect, useState, useCallback, useRef, useMemo } from "react";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Section } from "@astryxdesign/core/Section";
import { Grid } from "@astryxdesign/core/Grid";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Badge } from "@astryxdesign/core/Badge";
import { Spinner } from "@astryxdesign/core/Spinner";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Switch } from "@astryxdesign/core/Switch";
import { Selector } from "@astryxdesign/core/Selector";
import { Divider } from "@astryxdesign/core/Divider";
import { Save, Check, Plug, AlertTriangle, RefreshCw, Eye, EyeOff } from "lucide-react";
import { useAppStore } from "../lib/store";
import { SettingsRow } from "../components/SettingsRow";
import { TrackingRuleCard } from "../components/TrackingRuleCard";
import {
  getArtworkProviders,
  getSettings,
  getStoredSonarrApiKey,
  refreshAflCache,
  refreshAllArtwork,
  testNtfyConnection,
  testSonarrConnection,
  updateSettings,
  errorMessage,
} from "../lib/api";
import type { ArtworkProvider, Settings as SettingsType, SelectOption } from "../lib/types";

const actionToTracked = (action: string | undefined, fallback = true): boolean =>
  (action || (fallback ? "monitor" : "unmonitor")) === "monitor";
const trackedToAction = (tracked: boolean): string => (tracked ? "monitor" : "unmonitor");

const syncIntervalOptions: SelectOption[] = [
  { value: "1h", label: "1 hour" },
  { value: "3h", label: "3 hours" },
  { value: "6h", label: "6 hours" },
  { value: "12h", label: "12 hours" },
  { value: "24h", label: "24 hours" },
];

const sleep = (ms: number): Promise<void> => new Promise((r) => setTimeout(r, ms));

interface PristineSnapshot {
  sonarrUrl: string;
  apiKeyDirty: boolean;
  syncInterval: string;
  artworkDefaultSource: string;
  tmdbApiKeyDirty: boolean;
  fanartApiKeyDirty: boolean;
  trackFillerDefault: boolean;
  trackMixedDefault: boolean;
  trackAnimeCanonDefault: boolean;
  ntfyEnabled: boolean;
  ntfyServerUrl: string;
  ntfyTopic: string;
  ntfyTokenDirty: boolean;
}

export function SettingsPage() {
  const { setSettings, addNotification } = useAppStore();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [refreshingCache, setRefreshingCache] = useState(false);
  const [revealingKey, setRevealingKey] = useState(false);
  const [testPhase, setTestPhase] = useState<"idle" | "connecting" | "success" | "error">("idle");
  const [testErrorMessage, setTestErrorMessage] = useState("");

  const [sonarrUrl, setSonarrUrl] = useState("");
  const [sonarrApiKey, setSonarrApiKey] = useState("");
  const [savedApiKeyMasked, setSavedApiKeyMasked] = useState("");
  const [hasSavedApiKey, setHasSavedApiKey] = useState(false);
  const [apiKeyVisible, setApiKeyVisible] = useState(false);
  const [apiKeyDirty, setApiKeyDirty] = useState(false);
  const [syncInterval, setSyncInterval] = useState("6h");
  const [artworkDefaultSource, setArtworkDefaultSource] = useState("sonarr");
  const [tmdbApiKey, setTmdbApiKey] = useState("");
  const [tmdbApiKeySet, setTmdbApiKeySet] = useState(false);
  const [tmdbApiKeyDirty, setTmdbApiKeyDirty] = useState(false);
  const [fanartApiKey, setFanartApiKey] = useState("");
  const [fanartApiKeySet, setFanartApiKeySet] = useState(false);
  const [fanartApiKeyDirty, setFanartApiKeyDirty] = useState(false);
  const [trackFillerDefault, setTrackFillerDefault] = useState(false);
  const [trackMixedDefault, setTrackMixedDefault] = useState(true);
  const [trackAnimeCanonDefault, setTrackAnimeCanonDefault] = useState(true);
  const [cacheRefreshedAt, setCacheRefreshedAt] = useState<string | null>(null);
  const [cacheCount, setCacheCount] = useState(0);

  const [ntfyEnabled, setNtfyEnabled] = useState(false);
  const [ntfyServerUrl, setNtfyServerUrl] = useState("");
  const [ntfyTopic, setNtfyTopic] = useState("");
  const [ntfyToken, setNtfyToken] = useState("");
  const [ntfyTokenSet, setNtfyTokenSet] = useState(false);
  const [ntfyTokenDirty, setNtfyTokenDirty] = useState(false);
  const [ntfyTestPhase, setNtfyTestPhase] = useState<"idle" | "connecting" | "success" | "error">("idle");
  const [ntfyTestErrorMessage, setNtfyTestErrorMessage] = useState("");
  const [artworkProviders, setArtworkProviders] = useState<ArtworkProvider[]>([]);

  const [pristine, setPristine] = useState<PristineSnapshot>({
    sonarrUrl: "",
    apiKeyDirty: false,
    syncInterval: "6h",
    artworkDefaultSource: "sonarr",
    tmdbApiKeyDirty: false,
    fanartApiKeyDirty: false,
    trackFillerDefault: false,
    trackMixedDefault: true,
    trackAnimeCanonDefault: true,
    ntfyEnabled: false,
    ntfyServerUrl: "",
    ntfyTopic: "",
    ntfyTokenDirty: false,
  });

  const resetTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const ntfyResetTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const settingsDirty =
    !loading &&
    (sonarrUrl !== pristine.sonarrUrl ||
      apiKeyDirty ||
      syncInterval !== pristine.syncInterval ||
      artworkDefaultSource !== pristine.artworkDefaultSource ||
      tmdbApiKeyDirty ||
      fanartApiKeyDirty ||
      trackFillerDefault !== pristine.trackFillerDefault ||
      trackMixedDefault !== pristine.trackMixedDefault ||
      trackAnimeCanonDefault !== pristine.trackAnimeCanonDefault ||
      ntfyEnabled !== pristine.ntfyEnabled ||
      ntfyServerUrl !== pristine.ntfyServerUrl ||
      ntfyTopic !== pristine.ntfyTopic ||
      ntfyTokenDirty);

  const urlValid = !sonarrUrl || /^https?:\/\/.+/.test(sonarrUrl);

  const scheduleReset = (delay = 1800) => {
    if (resetTimer.current) clearTimeout(resetTimer.current);
    resetTimer.current = setTimeout(() => {
      setTestPhase("idle");
      setTestErrorMessage("");
    }, delay);
  };

  const clearReset = () => {
    if (resetTimer.current) clearTimeout(resetTimer.current);
    resetTimer.current = null;
  };

  const clearTestResult = () => {
    clearReset();
    if (testPhase === "success" || testPhase === "error") {
      setTestPhase("idle");
      setTestErrorMessage("");
    }
  };

  useEffect(() => {
    return () => {
      if (resetTimer.current) clearTimeout(resetTimer.current);
      if (ntfyResetTimer.current) clearTimeout(ntfyResetTimer.current);
    };
  }, []);

  const loadSettings = useCallback(async () => {
    setLoading(true);
    try {
      const data = await getSettings();
      if (data && !data.error) {
        setSonarrUrl(data.sonarr_url || "");
        setSavedApiKeyMasked(data.sonarr_api_key_masked || "");
        setHasSavedApiKey(!!data.sonarr_api_key_set);
        setSonarrApiKey(data.sonarr_api_key_masked || "");
        setApiKeyVisible(false);
        setApiKeyDirty(false);
        setSyncInterval(data.sync_interval || "6h");
        setArtworkDefaultSource(data.artwork_default_source || "sonarr");
        setTmdbApiKeySet(!!data.tmdb_api_key_set);
        setTmdbApiKey("");
        setTmdbApiKeyDirty(false);
        setFanartApiKeySet(!!data.fanart_api_key_set);
        setFanartApiKey("");
        setFanartApiKeyDirty(false);
        setTrackFillerDefault(actionToTracked(data.default_filler_action, false));
        setTrackMixedDefault(actionToTracked(data.default_mixed_canon_action, true));
        setTrackAnimeCanonDefault(actionToTracked(data.default_anime_canon_action, true));
        setCacheRefreshedAt(data.afl_show_cache_refreshed_at || null);
        setCacheCount(data.afl_show_cache_count || 0);
        setNtfyEnabled(!!data.ntfy_enabled);
        setNtfyServerUrl(data.ntfy_server_url || "");
        setNtfyTopic(data.ntfy_topic || "");
        setNtfyTokenSet(!!data.ntfy_token_set);
        setNtfyToken("");
        setNtfyTokenDirty(false);
        setNtfyTestPhase("idle");
        setNtfyTestErrorMessage("");
        setSettings(data);
        setPristine({
          sonarrUrl: data.sonarr_url || "",
          apiKeyDirty: false,
          syncInterval: data.sync_interval || "6h",
          artworkDefaultSource: data.artwork_default_source || "sonarr",
          tmdbApiKeyDirty: false,
          fanartApiKeyDirty: false,
          trackFillerDefault: actionToTracked(data.default_filler_action, false),
          trackMixedDefault: actionToTracked(data.default_mixed_canon_action, true),
          trackAnimeCanonDefault: actionToTracked(data.default_anime_canon_action, true),
          ntfyEnabled: !!data.ntfy_enabled,
          ntfyServerUrl: data.ntfy_server_url || "",
          ntfyTopic: data.ntfy_topic || "",
          ntfyTokenDirty: false,
        });
      }
    } catch (err: unknown) {
      addNotification("Failed to load settings: " + errorMessage(err), "error");
    }
    setLoading(false);
  }, [setSettings, addNotification]);

  useEffect(() => {
    loadSettings();
  }, [loadSettings]);

  // Fetch artwork providers from the API so the dropdown reflects the
  // actual backend state (available providers, API key requirements).
  useEffect(() => {
    getArtworkProviders()
      .then((res) => setArtworkProviders(res.providers || []))
      .catch((err: unknown) => addNotification("Failed to load artwork providers: " + errorMessage(err), "error"));
  }, [addNotification]);

  const artworkSourceOptions = useMemo<SelectOption[]>(() => {
    const opts: SelectOption[] = [{ value: "sonarr", label: "Sonarr (local)" }];
    for (const p of artworkProviders) {
      const suffix = p.needs_key && !p.available ? " (needs API key)" : "";
      opts.push({ value: p.id, label: `${p.label}${suffix}` });
    }
    return opts;
  }, [artworkProviders]);

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
        setSonarrApiKey(data.sonarr_api_key || "");
      } catch (err: unknown) {
        addNotification("Failed to reveal saved API key: " + errorMessage(err), "error");
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
    return data.sonarr_api_key || "";
  };

  const handleSave = async () => {
    setSaving(true);
    const sourceChanged = artworkDefaultSource !== pristine.artworkDefaultSource;
    try {
      const payload: Partial<SettingsType> = {
        sonarr_url: sonarrUrl,
        default_filler_action: trackedToAction(trackFillerDefault),
        default_mixed_canon_action: trackedToAction(trackMixedDefault),
        default_anime_canon_action: trackedToAction(trackAnimeCanonDefault),
        sync_interval: syncInterval,
        artwork_default_source: artworkDefaultSource,
      };
      if (apiKeyDirty && sonarrApiKey) {
        payload.sonarr_api_key = sonarrApiKey;
      }
      if (tmdbApiKeyDirty && tmdbApiKey) {
        payload.tmdb_api_key = tmdbApiKey;
      }
      if (fanartApiKeyDirty && fanartApiKey) {
        payload.fanart_api_key = fanartApiKey;
      }
      payload.ntfy_enabled = ntfyEnabled;
      payload.ntfy_server_url = ntfyServerUrl;
      payload.ntfy_topic = ntfyTopic;
      if (ntfyTokenDirty) {
        payload.ntfy_token = ntfyToken;
      }
      await updateSettings(payload);
      addNotification("Settings saved", "success");
      await loadSettings();

      // If the global artwork source changed, trigger a background refresh
      // of all shows so their local artwork matches the new default.
      if (sourceChanged) {
        addNotification("Refreshing all artwork from new source...", "info", 6000);
        refreshAllArtwork()
          .then(() => {
            addNotification("Artwork refresh triggered — updates will appear shortly", "info");
          })
          .catch((err: unknown) => {
            addNotification("Background artwork refresh failed: " + errorMessage(err), "error");
          });
      }
    } catch (err: unknown) {
      addNotification("Failed to save settings: " + errorMessage(err), "error");
    }
    setSaving(false);
  };

  const handleTestConnection = async () => {
    if (testPhase !== "idle") return;
    if (!sonarrUrl) {
      setTestErrorMessage("Enter a Sonarr URL first");
      setTestPhase("error");
      return;
    }
    if (!urlValid) {
      setTestErrorMessage("URL must start with http:// or https://");
      setTestPhase("error");
      return;
    }
    setTestErrorMessage("");
    clearReset();
    setTestPhase("connecting");
    const startTime = Date.now();
    const minDelay = 1100;
    try {
      const apiKey = await resolveApiKeyForTest();
      const result = await testSonarrConnection(sonarrUrl, apiKey);
      const elapsed = Date.now() - startTime;
      if (elapsed < minDelay) await sleep(minDelay - elapsed);
      if (!result?.success) {
        throw new Error(result?.error || "Connection failed");
      }
      setTestPhase("success");
      scheduleReset(2000);
    } catch (err: unknown) {
      const elapsed = Date.now() - startTime;
      if (elapsed < minDelay) await sleep(minDelay - elapsed);
      setTestErrorMessage(errorMessage(err));
      setTestPhase("error");
      scheduleReset(4000);
    }
  };

  const handleRefreshCache = async () => {
    setRefreshingCache(true);
    try {
      await refreshAflCache();
      addNotification("AFL show list refreshed", "success");
      await loadSettings();
    } catch (err: unknown) {
      addNotification("Failed to refresh AFL cache: " + errorMessage(err), "error");
    }
    setRefreshingCache(false);
  };

  const handleTestNtfy = async () => {
    if (ntfyTestPhase !== "idle") return;
    if (!ntfyServerUrl) {
      setNtfyTestErrorMessage("Enter a server URL first");
      setNtfyTestPhase("error");
      return;
    }
    if (!ntfyTopic) {
      setNtfyTestErrorMessage("Enter a topic first");
      setNtfyTestPhase("error");
      return;
    }
    setNtfyTestPhase("connecting");
    setNtfyTestErrorMessage("");
    const startTime = Date.now();
    const minDelay = 1100;
    try {
      const result = await testNtfyConnection({
        server_url: ntfyServerUrl,
        topic: ntfyTopic,
        token: ntfyTokenDirty ? ntfyToken : undefined,
      });
      const elapsed = Date.now() - startTime;
      if (elapsed < minDelay) await sleep(minDelay - elapsed);
      if (!result?.success) {
        throw new Error(result?.error || "Test failed");
      }
      setNtfyTestPhase("success");
      if (ntfyResetTimer.current) clearTimeout(ntfyResetTimer.current);
      ntfyResetTimer.current = setTimeout(() => setNtfyTestPhase("idle"), 2000);
    } catch (err: unknown) {
      const elapsed = Date.now() - startTime;
      if (elapsed < minDelay) await sleep(minDelay - elapsed);
      setNtfyTestErrorMessage(errorMessage(err));
      setNtfyTestPhase("error");
      if (ntfyResetTimer.current) clearTimeout(ntfyResetTimer.current);
      ntfyResetTimer.current = setTimeout(() => {
        setNtfyTestPhase("idle");
        setNtfyTestErrorMessage("");
      }, 4000);
    }
  };

  const formatCacheTime = (value: string | null | undefined): string => {
    if (!value) return "Never refreshed";
    return new Date(value).toLocaleString();
  };

  if (loading) {
    return (
      <Section padding={4} maxWidth={900} className="section-centered">
        <VStack gap={3} hAlign="center" padding={6}>
          <Spinner size="lg" label="Loading settings" />
        </VStack>
      </Section>
    );
  }

  return (
    <Section padding={4} maxWidth={900} className="section-centered page-enter">
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
              label={saving ? "Saving..." : "Save settings"}
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
          <Heading level={2} className="settings-section-header">Connection</Heading>
          <Divider />

          <SettingsRow
            label="Sonarr URL"
            description="Base URL of your Sonarr instance."
            error={sonarrUrl && !urlValid ? "URL must start with http:// or https://" : undefined}
          >
            <TextInput
              label="Sonarr URL"
              value={sonarrUrl}
              onChange={handleUrlInput}
              placeholder="http://localhost:8989"
              isLabelHidden
              width={320}
            />
          </SettingsRow>
          <Divider />

          <SettingsRow
            label="API key"
            description="Found in Sonarr under Settings, General."
          >
            <HStack gap={2} align="center">
              <TextInput
                label="API key"
                type={apiKeyVisible ? "text" : "password"}
                value={sonarrApiKey}
                onChange={handleApiKeyInput}
                placeholder={hasSavedApiKey ? "Saved API key" : "Your Sonarr API key"}
                isLabelHidden
                width={280}
              />
              {(hasSavedApiKey || apiKeyDirty) && (
                <Button
                  label={apiKeyVisible ? "Hide" : "Show"}
                  variant="ghost"
                  isIconOnly
                  icon={
                    apiKeyVisible ? (
                      <EyeOff size={16} />
                    ) : revealingKey ? (
                      <Spinner size="sm" />
                    ) : (
                      <Eye size={16} />
                    )
                  }
                  onClick={toggleApiKeyVisibility}
                  isDisabled={revealingKey}
                />
              )}
            </HStack>
          </SettingsRow>
          <Divider />

          <SettingsRow
            label="Test connection"
            description="Check that Mizu can reach Sonarr with the URL and API key above."
            error={testPhase === "error" && testErrorMessage ? testErrorMessage : undefined}
          >
            <Button
              label={
                testPhase === "connecting"
                  ? "Testing"
                  : testPhase === "success"
                    ? "Connected"
                    : testPhase === "error"
                      ? "Retry"
                      : "Test"
              }
              variant={
                testPhase === "success"
                  ? "primary"
                  : testPhase === "error"
                    ? "destructive"
                    : "secondary"
              }
              icon={
                testPhase === "connecting" ? (
                  <Spinner size="sm" />
                ) : testPhase === "success" ? (
                  <Check size={16} />
                ) : testPhase === "error" ? (
                  <AlertTriangle size={16} />
                ) : (
                  <Plug size={16} />
                )
              }
              onClick={handleTestConnection}
              isDisabled={!sonarrUrl || !urlValid}
              className={testPhase !== "idle" ? "test-status-btn" : undefined}
            />
          </SettingsRow>
        </VStack>

        {/* Sync */}
        <VStack gap={0}>
          <Heading level={2} className="settings-section-header">Sync</Heading>
          <Divider />

          <SettingsRow
            label="Sync interval"
            description="How often Mizu checks for episode changes and updates Sonarr."
          >
            <Selector
              label="Sync interval"
              value={syncInterval}
              options={syncIntervalOptions}
              onChange={(v) => setSyncInterval(v as string)}
              isLabelHidden
              width={180}
            />
          </SettingsRow>
          <Divider />

          {/* Multi-source artwork system */}
          <SettingsRow
            label="Artwork source"
            description="Default provider for fetching show artwork. Images are downloaded and stored locally so browsing works offline."
          >
            <Selector
              label="Artwork source"
              value={artworkDefaultSource}
              options={artworkSourceOptions}
              onChange={(v) => setArtworkDefaultSource(v as string)}
              isLabelHidden
              width={220}
            />
          </SettingsRow>
          <Divider />

          <SettingsRow
            label="TMDB API key"
            description="Required for TMDB artwork. Get a free key at themoviedb.org."
          >
            <HStack gap={2} align="center">
              <TextInput
                label="TMDB API key"
                type="password"
                value={tmdbApiKey}
                onChange={(v) => {
                  setTmdbApiKey(v);
                  setTmdbApiKeyDirty(true);
                }}
                placeholder={tmdbApiKeySet ? "Saved API key" : "Enter TMDB API key"}
                isLabelHidden
                width={260}
              />
              {tmdbApiKeySet && !tmdbApiKeyDirty && (
                <Badge variant="success" label="Key set" />
              )}
            </HStack>
          </SettingsRow>
          <Divider />

          <SettingsRow
            label="Fanart.tv API key"
            description="Required for Fanart.tv artwork. Get a free key at fanart.tv."
          >
            <HStack gap={2} align="center">
              <TextInput
                label="Fanart.tv API key"
                type="password"
                value={fanartApiKey}
                onChange={(v) => {
                  setFanartApiKey(v);
                  setFanartApiKeyDirty(true);
                }}
                placeholder={fanartApiKeySet ? "Saved API key" : "Enter Fanart.tv API key"}
                isLabelHidden
                width={260}
              />
              {fanartApiKeySet && !fanartApiKeyDirty && (
                <Badge variant="success" label="Key set" />
              )}
            </HStack>
          </SettingsRow>
          <Divider />

          <SettingsRow
            label="AFL cache"
            description={`${cacheCount} shows cached, refreshed ${formatCacheTime(cacheRefreshedAt)}.`}
          >
            <Button
              label={refreshingCache ? "Refreshing..." : "Refresh"}
              variant="secondary"
              icon={<RefreshCw size={16} />}
              onClick={handleRefreshCache}
              isLoading={refreshingCache}
            />
          </SettingsRow>
        </VStack>

        {/* Notifications */}
        <VStack gap={0}>
          <Heading level={2} className="settings-section-header">Notifications</Heading>
          <Divider />

          <SettingsRow
            label="Enable notifications"
            description="Get a push notification after each sync via ntfy.sh."
          >
            <Switch
              label="Enable notifications"
              value={ntfyEnabled}
              onChange={setNtfyEnabled}
              isLabelHidden
            />
          </SettingsRow>
          <Divider />

          <SettingsRow
            label="Server URL"
            description="ntfy server URL (self-hosted or public ntfy.sh)."
          >
            <TextInput
              label="Server URL"
              value={ntfyServerUrl}
              onChange={setNtfyServerUrl}
              placeholder="https://ntfy.sh"
              isLabelHidden
              width={320}
            />
          </SettingsRow>
          <Divider />

          <SettingsRow
            label="Topic"
            description="ntfy topic name to publish to."
          >
            <TextInput
              label="Topic"
              value={ntfyTopic}
              onChange={setNtfyTopic}
              placeholder="my-anime-sync"
              isLabelHidden
              width={320}
            />
          </SettingsRow>
          <Divider />

          <SettingsRow
            label="Access token"
            description="Optional — only needed if your ntfy server requires authentication."
          >
            <HStack gap={2} align="center">
              <TextInput
                label="Access token"
                type="password"
                value={ntfyToken}
                onChange={(v) => {
                  setNtfyToken(v);
                  setNtfyTokenDirty(true);
                }}
                placeholder={ntfyTokenSet ? "Saved token" : "Enter access token"}
                isLabelHidden
                width={260}
              />
              {ntfyTokenSet && !ntfyTokenDirty && (
                <Badge variant="success" label="Token set" />
              )}
            </HStack>
          </SettingsRow>
          <Divider />

          <SettingsRow
            label="Test notification"
            description="Send a test notification to verify your configuration."
            error={ntfyTestPhase === "error" && ntfyTestErrorMessage ? ntfyTestErrorMessage : undefined}
          >
            <Button
              label={
                ntfyTestPhase === "connecting"
                  ? "Sending"
                  : ntfyTestPhase === "success"
                    ? "Sent"
                    : ntfyTestPhase === "error"
                      ? "Retry"
                      : "Test"
              }
              variant={
                ntfyTestPhase === "success"
                  ? "primary"
                  : ntfyTestPhase === "error"
                    ? "destructive"
                    : "secondary"
              }
              icon={
                ntfyTestPhase === "connecting" ? (
                  <Spinner size="sm" />
                ) : ntfyTestPhase === "success" ? (
                  <Check size={16} />
                ) : ntfyTestPhase === "error" ? (
                  <AlertTriangle size={16} />
                ) : (
                  <Plug size={16} />
                )
              }
              onClick={handleTestNtfy}
              isDisabled={!ntfyServerUrl || !ntfyTopic}
              className={ntfyTestPhase !== "idle" ? "test-status-btn" : undefined}
            />
          </SettingsRow>
        </VStack>

        {/* Defaults */}
        <VStack gap={4}>
          <VStack gap={1}>
            <Heading level={2} className="settings-section-header">Defaults</Heading>
            <Text type="supporting" color="secondary" display="block">
              These tracking rules are applied automatically when you add a new anime mapping. You
              can override them per anime.
            </Text>
          </VStack>

          <Grid columns={{ minWidth: 240, max: 3 }} gap={4} align="start">
            <TrackingRuleCard
              variant="green"
              label="Canon"
              description="Main plot episodes"
              switchLabel="Track canon"
              value={trackAnimeCanonDefault}
              onChange={setTrackAnimeCanonDefault}
            />
            <TrackingRuleCard
              variant="yellow"
              label="Mixed"
              description="Mix of canon and filler"
              switchLabel="Track mixed"
              value={trackMixedDefault}
              onChange={setTrackMixedDefault}
            />
            <TrackingRuleCard
              variant="red"
              label="Filler"
              description="Skippable, no plot impact"
              switchLabel="Track filler"
              value={trackFillerDefault}
              onChange={setTrackFillerDefault}
            />
          </Grid>
        </VStack>
      </VStack>
    </Section>
  );
}
