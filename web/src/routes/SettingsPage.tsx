import { useEffect, useState, useCallback, useRef, useMemo } from "react";
import { useForm } from "react-hook-form";
import { valibotResolver } from "@hookform/resolvers/valibot";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Section } from "@astryxdesign/core/Section";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
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
import { SettingsSkeleton } from "../components/settings/SettingsSkeleton";
import {
  SecuritySettingsCard,
  MIN_PASSWORD_LENGTH,
  type SecurityIntent,
} from "../components/SecuritySettingsCard";
import {
  refreshAllArtwork,
  testNtfyConnection,
  testSonarrConnection,
  exportConfig,
  importConfig,
  errorMessage,
  setupPassword,
  removePassword,
} from "../lib/api";
import {
  queryKeys,
  settingsQuery as settingsQueryOptions,
  useAflCacheRefreshJob,
} from "../lib/queries";
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
  const loading = settingsQuery.isPending;
  const [saving, setSaving] = useState(false);
  const [savedFlash, setSavedFlash] = useState(false);
  const [securityIntent, setSecurityIntent] = useState<SecurityIntent>(null);
  const [securityPassword, setSecurityPassword] = useState("");
  const [securityConfirm, setSecurityConfirm] = useState("");
  const savedFlashRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [importingConfig, setImportingConfig] = useState(false);
  const importFileRef = useRef<HTMLInputElement | null>(null);
  const [testPhase, setTestPhase] = useState<"idle" | "connecting" | "success" | "error">("idle");

  // ── Stored secrets ──
  // The server only reports whether each key is set and a masked preview;
  // read those straight from the settings query. The *Dirty flags track
  // whether the user is replacing a key (presentation state, not form data).
  const savedApiKeyMasked = storeSettings?.sonarr_api_key_masked ?? "";
  const hasSavedApiKey = !!storeSettings?.sonarr_api_key_set;
  const [apiKeyDirty, setApiKeyDirty] = useState(false);

  const savedTmdbApiKeyMasked = storeSettings?.tmdb_api_key_masked ?? "";
  const tmdbApiKeySet = !!storeSettings?.tmdb_api_key_set;
  const [tmdbApiKeyDirty, setTmdbApiKeyDirty] = useState(false);

  const savedFanartApiKeyMasked = storeSettings?.fanart_api_key_masked ?? "";
  const fanartApiKeySet = !!storeSettings?.fanart_api_key_set;
  const [fanartApiKeyDirty, setFanartApiKeyDirty] = useState(false);

  const ntfyTokenSet = !!storeSettings?.ntfy_token_set;
  const [ntfyTokenDirty, setNtfyTokenDirty] = useState(false);
  const [ntfyTestPhase, setNtfyTestPhase] = useState<"idle" | "connecting" | "success" | "error">(
    "idle",
  );

  // AFL show-list cache (display-only)
  const cacheRefreshedAt = storeSettings?.afl_show_cache_refreshed_at ?? null;
  const cacheCount = storeSettings?.afl_show_cache_count ?? 0;

  const resetTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const ntfyResetTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

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
  const formChanged =
    !loading &&
    (formIsDirty || apiKeyDirty || tmdbApiKeyDirty || fanartApiKeyDirty || ntfyTokenDirty);
  const settingsDirty = formChanged || securityIntent !== null;

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

      // Leave any "Replace key" edits
      setApiKeyDirty(false);
      setTmdbApiKeyDirty(false);
      setFanartApiKeyDirty(false);
      setNtfyTokenDirty(false);
      setNtfyTestPhase("idle");
    },
    [reset],
  );

  /** Re-read settings from the server and load them into the form —
   *  after a save or import, when the form should show the stored values. */
  const reloadSettings = useCallback(async () => {
    try {
      applySettings(await queryClient.fetchQuery({ ...settingsQueryOptions, staleTime: 0 }));
    } catch (err: unknown) {
      addNotification("Failed to load settings: " + errorMessage(err), "error");
    }
  }, [applySettings, addNotification, queryClient]);

  // Load the stored settings into the form whenever they change on the
  // server — but only while the form is pristine, so a background refetch
  // (e.g. on window focus) never wipes in-progress edits.
  useEffect(() => {
    if (storeSettings && !settingsDirty) applySettings(storeSettings);
  }, [storeSettings, applySettings, settingsDirty]);

  const loadError = settingsQuery.error;
  useEffect(() => {
    if (loadError) addNotification("Failed to load settings: " + errorMessage(loadError), "error");
  }, [loadError, addNotification]);

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

  const resolveApiKeyForTest = (): string => {
    const current = getValues("sonarrApiKey");
    if (apiKeyDirty || !hasSavedApiKey) {
      return current;
    }
    // The saved key never reaches the browser; an empty key tells the
    // server to test with the stored one.
    return "";
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
      await reloadSettings();

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

  const submitSettings = handleSubmit(onSubmit);

  const resetSecurityIntent = () => {
    setSecurityIntent(null);
    setSecurityPassword("");
    setSecurityConfirm("");
  };

  const applySecurityIntent = async () => {
    if (securityIntent === "enable") {
      await setupPassword(securityPassword);
      queryClient.setQueryData(["authStatus"], {
        configured: true,
        authenticated: true,
      });
      addNotification("Password turned on", "success");
    } else if (securityIntent === "disable") {
      await removePassword();
      queryClient.setQueryData(["authStatus"], {
        configured: false,
        authenticated: false,
      });
      addNotification("Password turned off — the dashboard is now open", "success");
    }
    resetSecurityIntent();
  };

  const handleSave = async () => {
    if (securityIntent === "enable") {
      if (securityPassword.length < MIN_PASSWORD_LENGTH) {
        addNotification(`Password must be at least ${MIN_PASSWORD_LENGTH} characters`, "error");
        return;
      }
      if (securityPassword !== securityConfirm) {
        addNotification("Passwords do not match", "error");
        return;
      }
    }
    if (formChanged) {
      await submitSettings();
    }
    if (securityIntent) {
      setSaving(true);
      try {
        await applySecurityIntent();
        setSavedFlash(true);
        if (savedFlashRef.current) clearTimeout(savedFlashRef.current);
        savedFlashRef.current = setTimeout(() => setSavedFlash(false), 1500);
      } catch (err: unknown) {
        addNotification("Failed to update password: " + errorMessage(err), "error");
      }
      setSaving(false);
    }
  };

  const handleDiscard = () => {
    resetSecurityIntent();
    applySettings(storeSettings);
  };

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
  // rate-limits at 2s/show and outlives any HTTP request); the job hook
  // polls its status and reports here when it ends — including a refresh
  // that was already running when the page opened.
  const aflJob = useAflCacheRefreshJob((status) => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.settings });
    if (status.last_error) {
      addNotification("AFL cache refresh failed: " + status.last_error, "error");
    } else {
      addNotification("AFL show list refreshed", "success");
    }
  });
  const refreshingCache = aflJob.running;

  const handleRefreshCache = async () => {
    if (refreshingCache) return;
    try {
      await aflJob.start();
    } catch (err: unknown) {
      addNotification("Failed to refresh AFL cache: " + errorMessage(err), "error");
    }
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
      await reloadSettings();
      void queryClient.invalidateQueries({ queryKey: queryKeys.shows });
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
        </HStack>

        {/* Connection */}
        <ConnectionCard
          control={control}
          sonarrUrlError={errors.sonarrUrl?.message}
          testPhase={testPhase}
          testDisabled={!getValues("sonarrUrl") || !!errors.sonarrUrl}
          hasSavedApiKey={hasSavedApiKey}
          apiKeyDirty={apiKeyDirty}
          savedApiKeyMasked={savedApiKeyMasked}
          clearTestResult={clearTestResult}
          setApiKeyDirty={setApiKeyDirty}
          handleTestConnection={handleTestConnection}
        />

        {/* Sync */}
        <SyncCard
          control={control}
          artworkSourceOptions={artworkSourceOptions}
          tmdb={{
            isSet: tmdbApiKeySet,
            dirty: tmdbApiKeyDirty,
            savedMasked: savedTmdbApiKeyMasked,
          }}
          fanart={{
            isSet: fanartApiKeySet,
            dirty: fanartApiKeyDirty,
            savedMasked: savedFanartApiKeyMasked,
          }}
          setTmdbApiKeyDirty={setTmdbApiKeyDirty}
          setFanartApiKeyDirty={setFanartApiKeyDirty}
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

        {/* Interface */}
        <InterfaceCard control={control} />

        {/* Security */}
        <SecuritySettingsCard
          intent={securityIntent}
          onIntentChange={setSecurityIntent}
          password={securityPassword}
          confirm={securityConfirm}
          onPasswordChange={setSecurityPassword}
          onConfirmChange={setSecurityConfirm}
          isDisabled={saving}
        />

        {/* Backup */}
        <BackupCard
          importingConfig={importingConfig}
          importFileRef={importFileRef}
          handleExportConfig={handleExportConfig}
          handleImportConfigFile={handleImportConfigFile}
        />
      </VStack>

      {(settingsDirty || savedFlash) && (
        <HStack
          className="settings-save-bar"
          justify="between"
          align="center"
          gap={3}
          role="region"
          aria-label="Unsaved settings"
        >
          <Text type="supporting" color="secondary">
            {savedFlash ? "All changes saved" : "You have unsaved changes"}
          </Text>
          <HStack gap={2} align="center">
            <Button
              label="Discard"
              variant="ghost"
              onClick={handleDiscard}
              isDisabled={saving || savedFlash}
              data-cuelume-press
            />
            <Button
              label={savedFlash ? "Saved" : saving ? "Saving..." : "Save settings"}
              variant="primary"
              icon={
                savedFlash ? <Check size={14} color="var(--color-success)" /> : <Save size={16} />
              }
              onClick={handleSave}
              isDisabled={saving || savedFlash}
              isLoading={saving}
              className={savedFlash ? "save-flash" : undefined}
              data-cuelume-press
            />
          </HStack>
        </HStack>
      )}
    </Section>
  );
}
