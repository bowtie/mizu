import { useEffect, useState, useCallback, useRef, useMemo } from "react";
import { useForm } from "react-hook-form";
import { valibotResolver } from "@hookform/resolvers/valibot";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Section } from "@astryxdesign/core/Section";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Badge } from "@astryxdesign/core/Badge";
import { Save, Check } from "lucide-react";
import { useNotificationStore } from "../lib/store";
import { useSettings, useArtworkProviders, useUpdateSettings } from "../lib/queries";
import { useQueryClient } from "@tanstack/react-query";
import { applySoundsEnabled } from "../lib/sounds";
import { ConnectionCard } from "../components/settings/ConnectionCard";
import { SyncCard } from "../components/settings/SyncCard";
import { NotificationsCard } from "../components/settings/NotificationsCard";
import { BackupCard } from "../components/settings/BackupCard";
import { InterfaceCard } from "../components/settings/InterfaceCard";
import { DefaultsSection } from "../components/settings/DefaultsSection";
import { SettingsSkeleton } from "../components/settings/SettingsSkeleton";
import { SecuritySettingsCard } from "../components/SecuritySettingsCard";
import {
  getSettings,
  refreshAflCache,
  getAflCacheRefreshStatus,
  type AflCacheRefreshStatus,
  refreshAllArtwork,
  testNtfyConnection,
  testSonarrConnection,
  exportConfig,
  importConfig,
  errorMessage,
} from "../lib/api";
import { queryKeys } from "../lib/queries";
import { SettingsFormSchema, type SettingsForm } from "../lib/schemas";
import type { Settings as SettingsType, SelectOption } from "../lib/types";

const actionToTracked = (action: string | undefined, fallback = true): boolean =>
  (action || (fallback ? "monitor" : "unmonitor")) === "monitor";
const trackedToAction = (tracked: boolean): string => (tracked ? "monitor" : "unmonitor");

const sleep = (ms: number): Promise<void> => new Promise((r) => setTimeout(r, ms));

const isNotifyOn = (value: unknown): value is "always" | "changes" | "failures" =>
  value === "always" || value === "changes" || value === "failures";

export function SettingsPage() {
  const addNotification = useNotificationStore((s) => s.addNotification);
  const queryClient = useQueryClient();
  const settingsQuery = useSettings();
  const storeSettings = settingsQuery.data ?? null;
  const { data: artworkProviders = [] } = useArtworkProviders();
  const updateSettingsMutation = useUpdateSettings();
  const [loading, setLoading] = useState(!storeSettings);
  const [saving, setSaving] = useState(false);
  const [savedFlash, setSavedFlash] = useState(false);
  const savedFlashRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [refreshingCache, setRefreshingCache] = useState(false);
  const [importingConfig, setImportingConfig] = useState(false);
  const importFileRef = useRef<HTMLInputElement | null>(null);
  const [testPhase, setTestPhase] = useState<"idle" | "connecting" | "success" | "error">("idle");

  // ── API key UI state (visibility, masked display, dirty tracking) ──
  // These stay as useState because they're presentation logic, not form data.
  const [savedApiKeyMasked, setSavedApiKeyMasked] = useState("");
  const [savedApiKeyFull, setSavedApiKeyFull] = useState("");
  const [hasSavedApiKey, setHasSavedApiKey] = useState(false);
  const [apiKeyVisible, setApiKeyVisible] = useState(false);
  const [apiKeyDirty, setApiKeyDirty] = useState(false);

  const [savedTmdbApiKeyMasked, setSavedTmdbApiKeyMasked] = useState("");
  const [savedTmdbApiKeyFull, setSavedTmdbApiKeyFull] = useState("");
  const [tmdbApiKeySet, setTmdbApiKeySet] = useState(false);
  const [tmdbApiKeyVisible, setTmdbApiKeyVisible] = useState(false);
  const [tmdbApiKeyDirty, setTmdbApiKeyDirty] = useState(false);

  const [savedFanartApiKeyMasked, setSavedFanartApiKeyMasked] = useState("");
  const [savedFanartApiKeyFull, setSavedFanartApiKeyFull] = useState("");
  const [fanartApiKeySet, setFanartApiKeySet] = useState(false);
  const [fanartApiKeyVisible, setFanartApiKeyVisible] = useState(false);
  const [fanartApiKeyDirty, setFanartApiKeyDirty] = useState(false);

  const [ntfyTokenSet, setNtfyTokenSet] = useState(false);
  const [ntfyTokenDirty, setNtfyTokenDirty] = useState(false);
  const [ntfyTestPhase, setNtfyTestPhase] = useState<"idle" | "connecting" | "success" | "error">(
    "idle",
  );

  // Cache info (display-only, not form fields)
  const [cacheRefreshedAt, setCacheRefreshedAt] = useState<string | null>(null);
  const [cacheCount, setCacheCount] = useState(0);

  const resetTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const ntfyResetTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Cancels the AFL refresh status poll when the component unmounts —
  // without it the poll loop kept hitting the status endpoint for up to
  // ~15 minutes after leaving the page.
  const aflPollCancelled = useRef(false);

  // ── React Hook Form ──
  const {
    control,
    handleSubmit,
    reset,
    formState: { isDirty: formIsDirty, errors },
    getValues,
  } = useForm<SettingsForm>({
    resolver: valibotResolver(SettingsFormSchema),
    defaultValues: {
      sonarrUrl: "",
      sonarrApiKey: "",
      syncInterval: "6h",
      artworkDefaultSource: "sonarr",
      tmdbApiKey: "",
      fanartApiKey: "",
      trackFillerDefault: false,
      trackMixedDefault: true,
      trackAnimeCanonDefault: true,
      ntfyEnabled: false,
      ntfyServerUrl: "",
      ntfyTopic: "",
      ntfyToken: "",
      notifyOn: "always",
      soundsEnabled: true,
    },
    mode: "onChange",
  });

  // Any API key dirty flag also makes the whole form dirty
  const settingsDirty =
    !loading &&
    (formIsDirty || apiKeyDirty || tmdbApiKeyDirty || fanartApiKeyDirty || ntfyTokenDirty);

  const scheduleReset = (delay = 1800) => {
    if (resetTimer.current) clearTimeout(resetTimer.current);
    resetTimer.current = setTimeout(() => {
      setTestPhase("idle");
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
    }
  };

  useEffect(() => {
    return () => {
      if (resetTimer.current) clearTimeout(resetTimer.current);
      if (ntfyResetTimer.current) clearTimeout(ntfyResetTimer.current);
      if (savedFlashRef.current) clearTimeout(savedFlashRef.current);
      aflPollCancelled.current = true;
    };
  }, []);

  const applySettings = useCallback(
    (data: SettingsType | null) => {
      if (!data || (data as { error?: string }).error) return;

      // Reset form fields. Key fields start empty — the stored (masked)
      // key shows in the placeholder. Prefilling the field with the
      // masked template meant any keystroke (even fixing a stray char)
      // saved something like "abcd...wxyz" as the real key.
      reset({
        sonarrUrl: data.sonarr_url || "",
        sonarrApiKey: "",
        syncInterval: data.sync_interval || "6h",
        artworkDefaultSource: data.artwork_default_source || "sonarr",
        tmdbApiKey: "",
        fanartApiKey: "",
        trackFillerDefault: actionToTracked(data.default_filler_action, false),
        trackMixedDefault: actionToTracked(data.default_mixed_canon_action, true),
        trackAnimeCanonDefault: actionToTracked(data.default_anime_canon_action, true),
        ntfyEnabled: !!data.ntfy_enabled,
        ntfyServerUrl: data.ntfy_server_url || "",
        ntfyTopic: data.ntfy_topic || "",
        ntfyToken: "",
        notifyOn: isNotifyOn(data.ntfy_notify_on) ? data.ntfy_notify_on : "always",
        soundsEnabled: data.ui_sounds_enabled !== false,
      });

      // Reset API key UI state
      setSavedApiKeyMasked(data.sonarr_api_key_masked || "");
      setSavedApiKeyFull(data.sonarr_api_key || "");
      setHasSavedApiKey(!!data.sonarr_api_key_set);
      setApiKeyVisible(false);
      setApiKeyDirty(false);

      setSavedTmdbApiKeyMasked(data.tmdb_api_key_masked || "");
      setSavedTmdbApiKeyFull(data.tmdb_api_key || "");
      setTmdbApiKeySet(!!data.tmdb_api_key_set);
      setTmdbApiKeyVisible(false);
      setTmdbApiKeyDirty(false);

      setSavedFanartApiKeyMasked(data.fanart_api_key_masked || "");
      setSavedFanartApiKeyFull(data.fanart_api_key || "");
      setFanartApiKeySet(!!data.fanart_api_key_set);
      setFanartApiKeyVisible(false);
      setFanartApiKeyDirty(false);

      setNtfyTokenSet(!!data.ntfy_token_set);
      setNtfyTokenDirty(false);
      setNtfyTestPhase("idle");

      setCacheRefreshedAt(data.afl_show_cache_refreshed_at || null);
      setCacheCount(data.afl_show_cache_count || 0);
    },
    [reset],
  );

  const loadSettings = useCallback(async () => {
    try {
      const data = await queryClient.fetchQuery({
        queryKey: ["settings"],
        queryFn: getSettings,
      });
      applySettings(data);
    } catch (err: unknown) {
      addNotification("Failed to load settings: " + errorMessage(err), "error");
    }
    setLoading(false);
  }, [applySettings, addNotification, queryClient]);

  // If the store already has settings (fetched on app mount), populate
  // immediately and skip the loading spinner. Still do a background
  // refresh to pick up any changes. A later refetch (staleTime +
  // refetchOnWindowFocus) must not wipe in-progress edits, so only
  // apply while the form is pristine.
  useEffect(() => {
    if (!storeSettings) return;
    if (!settingsDirty) {
      applySettings(storeSettings);
      setLoading(false);
    }
  }, [storeSettings, applySettings, settingsDirty]);

  useEffect(() => {
    loadSettings();
  }, [loadSettings]);

  // Warn user when navigating away with unsaved changes
  useEffect(() => {
    if (!settingsDirty) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [settingsDirty]);

  const artworkSourceOptions = useMemo<SelectOption[]>(() => {
    const opts: SelectOption[] = [{ value: "sonarr", label: "Sonarr (local)" }];
    for (const p of artworkProviders) {
      const suffix = p.needs_key && !p.available ? " (needs API key)" : "";
      opts.push({ value: p.id, label: `${p.label}${suffix}` });
    }
    return opts;
  }, [artworkProviders]);

  // ── API key helpers ──

  const toggleApiKeyVisibility = () => {
    if (apiKeyVisible) {
      setApiKeyVisible(false);
      if (!apiKeyDirty && hasSavedApiKey) {
        reset({ ...getValues(), sonarrApiKey: "" });
      }
      return;
    }
    if (!apiKeyDirty && hasSavedApiKey) {
      reset({ ...getValues(), sonarrApiKey: savedApiKeyFull });
    }
    setApiKeyVisible(true);
  };

  const resolveApiKeyForTest = (): string => {
    const current = getValues("sonarrApiKey");
    if (apiKeyDirty || apiKeyVisible || !hasSavedApiKey) {
      return current;
    }
    return savedApiKeyFull;
  };

  const toggleTmdbApiKeyVisibility = () => {
    if (tmdbApiKeyVisible) {
      setTmdbApiKeyVisible(false);
      if (!tmdbApiKeyDirty && tmdbApiKeySet) {
        reset({ ...getValues(), tmdbApiKey: "" });
      }
    } else {
      if (!tmdbApiKeyDirty && tmdbApiKeySet) {
        reset({ ...getValues(), tmdbApiKey: savedTmdbApiKeyFull });
      }
      setTmdbApiKeyVisible(true);
    }
  };

  const toggleFanartApiKeyVisibility = () => {
    if (fanartApiKeyVisible) {
      setFanartApiKeyVisible(false);
      if (!fanartApiKeyDirty && fanartApiKeySet) {
        reset({ ...getValues(), fanartApiKey: "" });
      }
    } else {
      if (!fanartApiKeyDirty && fanartApiKeySet) {
        reset({ ...getValues(), fanartApiKey: savedFanartApiKeyFull });
      }
      setFanartApiKeyVisible(true);
    }
  };

  // ── Save ──

  const onSubmit = async (formData: SettingsForm) => {
    setSaving(true);
    const pristineValues = {
      artworkDefaultSource: storeSettings?.artwork_default_source || "sonarr",
    };
    const sourceChanged = formData.artworkDefaultSource !== pristineValues.artworkDefaultSource;
    try {
      const payload: Partial<SettingsType> = {
        sonarr_url: formData.sonarrUrl,
        default_filler_action: trackedToAction(formData.trackFillerDefault),
        default_mixed_canon_action: trackedToAction(formData.trackMixedDefault),
        default_anime_canon_action: trackedToAction(formData.trackAnimeCanonDefault),
        sync_interval: formData.syncInterval,
        artwork_default_source: formData.artworkDefaultSource,
      };
      if (apiKeyDirty && formData.sonarrApiKey) {
        payload.sonarr_api_key = formData.sonarrApiKey;
      }
      if (tmdbApiKeyDirty && formData.tmdbApiKey) {
        payload.tmdb_api_key = formData.tmdbApiKey;
      }
      if (fanartApiKeyDirty && formData.fanartApiKey) {
        payload.fanart_api_key = formData.fanartApiKey;
      }
      payload.ntfy_enabled = formData.ntfyEnabled;
      payload.ntfy_server_url = formData.ntfyServerUrl;
      payload.ntfy_topic = formData.ntfyTopic;
      payload.ntfy_notify_on = formData.notifyOn;
      if (ntfyTokenDirty) {
        payload.ntfy_token = formData.ntfyToken;
      }
      payload.ui_sounds_enabled = formData.soundsEnabled;
      await updateSettingsMutation.mutateAsync(payload);
      addNotification("Settings saved", "success");
      applySoundsEnabled(formData.soundsEnabled);
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
      setSavedFlash(true);
      if (savedFlashRef.current) clearTimeout(savedFlashRef.current);
      savedFlashRef.current = setTimeout(() => setSavedFlash(false), 1500);
    } catch (err: unknown) {
      addNotification("Failed to save settings: " + errorMessage(err), "error");
    }
    setSaving(false);
  };

  const handleSave = handleSubmit(onSubmit);

  // ── Test connection ──

  const handleTestConnection = async () => {
    if (testPhase !== "idle") return;
    const sonarrUrl = getValues("sonarrUrl");
    if (!sonarrUrl) {
      addNotification("Enter a Sonarr URL first", "error");
      setTestPhase("error");
      return;
    }
    const urlValid = !sonarrUrl || /^https?:\/\/.+/.test(sonarrUrl);
    if (!urlValid) {
      addNotification("URL must start with http:// or https://", "error");
      setTestPhase("error");
      return;
    }
    clearReset();
    setTestPhase("connecting");
    const startTime = Date.now();
    const minDelay = 1100;
    try {
      const apiKey = resolveApiKeyForTest();
      const result = await testSonarrConnection(sonarrUrl, apiKey);
      const elapsed = Date.now() - startTime;
      if (elapsed < minDelay) await sleep(minDelay - elapsed);
      if (!result?.success) {
        throw new Error(result?.error || "Connection failed");
      }
      setTestPhase("success");
      addNotification("Sonarr connection successful", "success");
      scheduleReset(2000);
    } catch (err: unknown) {
      const elapsed = Date.now() - startTime;
      if (elapsed < minDelay) await sleep(minDelay - elapsed);
      addNotification("Connection test failed: " + errorMessage(err), "error");
      setTestPhase("error");
      scheduleReset(4000);
    }
  };

  // The refresh runs server-side in the background (a full scrape
  // rate-limits at 2s/show and outlives any HTTP request), so the UI
  // polls the status endpoint until the flag clears. Capped at ~15 min
  // to avoid polling forever if the server died mid-scrape.
  const AFL_REFRESH_POLL_MS = 2000;
  const AFL_REFRESH_MAX_POLLS = 450;

  const pollAflRefreshStatus = useCallback(async (): Promise<AflCacheRefreshStatus> => {
    for (let i = 0; i < AFL_REFRESH_MAX_POLLS; i++) {
      if (aflPollCancelled.current) return { refreshing: false };
      const status = await getAflCacheRefreshStatus();
      if (aflPollCancelled.current) return { refreshing: false };
      if (!status.refreshing) return status;
      await sleep(AFL_REFRESH_POLL_MS);
    }
    return { refreshing: false, last_error: "Refresh timed out — check the Activity page" };
  }, []);

  // Resume the spinner if a refresh is already in flight (e.g. the page
  // was reloaded mid-scrape).
  useEffect(() => {
    let cancelled = false;
    getAflCacheRefreshStatus()
      .then((status) => {
        if (!cancelled && status.refreshing) setRefreshingCache(true);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  const handleRefreshCache = async () => {
    if (refreshingCache) return;
    setRefreshingCache(true);
    aflPollCancelled.current = false;
    try {
      try {
        await refreshAflCache();
      } catch (err: unknown) {
        // A refresh triggered from another tab/device is fine — just watch it.
        if (!errorMessage(err).includes("already in progress")) throw err;
      }
      const status = await pollAflRefreshStatus();
      if (aflPollCancelled.current) return; // unmounted — skip state updates
      await loadSettings();
      if (status.last_error) {
        addNotification("AFL cache refresh failed: " + status.last_error, "error");
      } else {
        addNotification("AFL show list refreshed", "success");
      }
    } catch (err: unknown) {
      addNotification("Failed to refresh AFL cache: " + errorMessage(err), "error");
    }
    setRefreshingCache(false);
  };

  // ── Config backup ──

  const handleExportConfig = async () => {
    try {
      const blob = await exportConfig();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `mizu-config-${new Date().toISOString().slice(0, 10)}.json`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err: unknown) {
      addNotification("Failed to export config: " + errorMessage(err), "error");
    }
  };

  const handleImportConfigFile = async (file: File) => {
    setImportingConfig(true);
    try {
      const text = await file.text();
      let parsed: unknown;
      try {
        parsed = JSON.parse(text);
      } catch {
        addNotification("That file isn't valid JSON — pick a Mizu config export", "error");
        return;
      }
      const proceed = window.confirm(
        "Import this config? Shows with the same Sonarr series are replaced, " +
          "and imported settings overwrite the current ones.",
      );
      if (!proceed) return;
      const result = await importConfig(parsed);
      addNotification(
        `Imported ${result.mappings_imported} show${result.mappings_imported === 1 ? "" : "s"} ` +
          `and ${result.settings_imported} setting${result.settings_imported === 1 ? "" : "s"}`,
        "success",
      );
      await loadSettings();
      queryClient.invalidateQueries({ queryKey: queryKeys.shows });
      queryClient.invalidateQueries({ queryKey: queryKeys.bootstrap });
    } catch (err: unknown) {
      addNotification("Failed to import config: " + errorMessage(err), "error");
    } finally {
      setImportingConfig(false);
      if (importFileRef.current) importFileRef.current.value = "";
    }
  };

  const handleTestNtfy = async () => {
    if (ntfyTestPhase !== "idle") return;
    const formData = getValues();
    if (!formData.ntfyServerUrl) {
      addNotification("Enter a server URL first", "error");
      setNtfyTestPhase("error");
      return;
    }
    if (!formData.ntfyTopic) {
      addNotification("Enter a topic first", "error");
      setNtfyTestPhase("error");
      return;
    }
    setNtfyTestPhase("connecting");
    const startTime = Date.now();
    const minDelay = 1100;
    try {
      const result = await testNtfyConnection({
        server_url: formData.ntfyServerUrl,
        topic: formData.ntfyTopic,
        token: ntfyTokenDirty ? formData.ntfyToken : undefined,
      });
      const elapsed = Date.now() - startTime;
      if (elapsed < minDelay) await sleep(minDelay - elapsed);
      if (!result?.success) {
        throw new Error(result?.error || "Test failed");
      }
      setNtfyTestPhase("success");
      addNotification("ntfy notification sent successfully", "success");
      if (ntfyResetTimer.current) clearTimeout(ntfyResetTimer.current);
      ntfyResetTimer.current = setTimeout(() => setNtfyTestPhase("idle"), 2000);
    } catch (err: unknown) {
      const elapsed = Date.now() - startTime;
      if (elapsed < minDelay) await sleep(minDelay - elapsed);
      addNotification("Notification test failed: " + errorMessage(err), "error");
      setNtfyTestPhase("error");
      if (ntfyResetTimer.current) clearTimeout(ntfyResetTimer.current);
      ntfyResetTimer.current = setTimeout(() => {
        setNtfyTestPhase("idle");
      }, 4000);
    }
  };

  if (loading) {
    return <SettingsSkeleton />;
  }

  return (
    <Section padding={4} maxWidth={900} className="section-centered page-enter">
      <VStack gap={6}>
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
              label={savedFlash ? "Saved" : saving ? "Saving..." : "Save settings"}
              variant="primary"
              icon={
                savedFlash ? <Check size={14} color="var(--color-success)" /> : <Save size={16} />
              }
              onClick={handleSave}
              isDisabled={saving || !settingsDirty}
              isLoading={saving}
              className={savedFlash ? "save-flash" : undefined}
              data-cuelume-press
            />
          </HStack>
        </HStack>

        {/* Connection */}
        <ConnectionCard
          control={control}
          sonarrUrlError={errors.sonarrUrl?.message}
          testPhase={testPhase}
          testDisabled={!getValues("sonarrUrl") || !!errors.sonarrUrl}
          apiKeyVisible={apiKeyVisible}
          hasSavedApiKey={hasSavedApiKey}
          apiKeyDirty={apiKeyDirty}
          savedApiKeyMasked={savedApiKeyMasked}
          clearTestResult={clearTestResult}
          setApiKeyDirty={setApiKeyDirty}
          toggleApiKeyVisibility={toggleApiKeyVisibility}
          handleTestConnection={handleTestConnection}
        />

        {/* Sync */}
        <SyncCard
          control={control}
          artworkSourceOptions={artworkSourceOptions}
          tmdb={{
            visible: tmdbApiKeyVisible,
            isSet: tmdbApiKeySet,
            dirty: tmdbApiKeyDirty,
            savedMasked: savedTmdbApiKeyMasked,
          }}
          fanart={{
            visible: fanartApiKeyVisible,
            isSet: fanartApiKeySet,
            dirty: fanartApiKeyDirty,
            savedMasked: savedFanartApiKeyMasked,
          }}
          setTmdbApiKeyDirty={setTmdbApiKeyDirty}
          setFanartApiKeyDirty={setFanartApiKeyDirty}
          toggleTmdbApiKeyVisibility={toggleTmdbApiKeyVisibility}
          toggleFanartApiKeyVisibility={toggleFanartApiKeyVisibility}
          cacheCount={cacheCount}
          cacheRefreshedAt={cacheRefreshedAt}
          refreshingCache={refreshingCache}
          handleRefreshCache={handleRefreshCache}
        />

        {/* Notifications */}
        <NotificationsCard
          control={control}
          ntfyTokenSet={ntfyTokenSet}
          ntfyTokenDirty={ntfyTokenDirty}
          ntfyTestPhase={ntfyTestPhase}
          ntfyTestDisabled={!getValues("ntfyServerUrl") || !getValues("ntfyTopic")}
          setNtfyTokenDirty={setNtfyTokenDirty}
          handleTestNtfy={handleTestNtfy}
        />

        {/* Backup */}
        <BackupCard
          importingConfig={importingConfig}
          importFileRef={importFileRef}
          handleExportConfig={handleExportConfig}
          handleImportConfigFile={handleImportConfigFile}
        />

        {/* Interface */}
        <InterfaceCard control={control} />

        {/* Security */}
        <SecuritySettingsCard />

        {/* Defaults */}
        <DefaultsSection control={control} />
      </VStack>
    </Section>
  );
}
