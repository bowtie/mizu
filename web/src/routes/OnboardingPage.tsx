import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Section } from "@astryxdesign/core/Section";
import { Card } from "@astryxdesign/core/Card";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Selector } from "@astryxdesign/core/Selector";
import { Switch } from "@astryxdesign/core/Switch";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Divider } from "@astryxdesign/core/Divider";
import { Check, Loader2, Plug, Upload, Waves } from "lucide-react";
import { SettingsRow } from "../components/SettingsRow";
import { syncIntervalOptions } from "../components/settings/SyncCard";
import {
  queryKeys,
  settingsQuery,
  useAflCacheRefreshJob,
  useSettings,
  useUpdateSettings,
} from "../lib/queries";
import { errorMessage, importConfig, testSonarrConnection } from "../lib/api";
import { trackedToAction } from "./addWizardHelpers";

type StepKey = "sonarr" | "defaults";

const STEPS: { key: StepKey; label: string }[] = [
  { key: "sonarr", label: "Connect Sonarr" },
  { key: "defaults", label: "Sync and defaults" },
];

type AflState =
  | { phase: "idle" }
  | { phase: "fetching" }
  | { phase: "ready"; count: number }
  | { phase: "failed"; message: string };

const AFL_POLL_MS = 1000;
const AFL_COUNT_ATTEMPTS = 4;

const reachedOverNetwork = !["localhost", "127.0.0.1", "[::1]"].includes(window.location.hostname);

// One-time setup: this page is only reachable until Sonarr is connected
// (see OnboardingGate in App.tsx); everything here can be changed later in
// Settings. Nothing is saved until the last step, because saving the API key
// is what closes this page.
export function OnboardingPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: settings } = useSettings();
  const updateSettings = useUpdateSettings();

  const [step, setStep] = useState<StepKey>("sonarr");
  const [url, setUrl] = useState(settings?.sonarr_url || "http://localhost:8989");
  const [apiKey, setApiKey] = useState("");
  const [tested, setTested] = useState(false);
  const [syncInterval, setSyncInterval] = useState(settings?.sync_interval || "6h");
  const [trackCanon, setTrackCanon] = useState(
    settings?.default_anime_canon_action !== "unmonitor",
  );
  const [trackMixed, setTrackMixed] = useState(
    settings?.default_mixed_canon_action !== "unmonitor",
  );
  const [trackFiller, setTrackFiller] = useState(settings?.default_filler_action === "monitor");
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [afl, setAfl] = useState<AflState>({ phase: "idle" });
  const restoreFileRef = useRef<HTMLInputElement | null>(null);
  const aflStarted = useRef(false);
  const unmounted = useRef(false);

  const stepIndex = STEPS.findIndex((s) => s.key === step);
  const busy = testing || saving || restoring;

  useEffect(() => {
    unmounted.current = false;
    return () => {
      unmounted.current = true;
    };
  }, []);

  // Fetch the AnimeFillerList catalogue while the user picks their defaults,
  // so the Add anime wizard has matches ready on first use. Best effort: a
  // failure never blocks setup (the first sync retries it).
  const aflJob = useAflCacheRefreshJob(async (status) => {
    if (status.last_error) {
      setAfl({ phase: "failed", message: status.last_error });
      return;
    }
    // The count can trail the status flag by a moment; look a few times
    // before settling on "none".
    let count = 0;
    for (let attempt = 0; attempt < AFL_COUNT_ATTEMPTS && count === 0; attempt++) {
      if (attempt > 0) await new Promise((resolve) => setTimeout(resolve, AFL_POLL_MS));
      if (unmounted.current) return;
      const fresh = await queryClient.fetchQuery({ ...settingsQuery, staleTime: 0 });
      count = fresh.afl_show_cache_count || 0;
    }
    if (!unmounted.current) setAfl({ phase: "ready", count });
  });
  const startAfl = aflJob.start;

  useEffect(() => {
    if (step !== "defaults" || aflStarted.current) return;
    aflStarted.current = true;
    setAfl({ phase: "fetching" });
    startAfl().catch((err: unknown) => {
      if (!unmounted.current) setAfl({ phase: "failed", message: errorMessage(err) });
    });
  }, [step, startAfl]);

  const editConnection = (setter: (v: string) => void) => (value: string) => {
    setter(value);
    setTested(false);
    setError(null);
  };

  const handleTest = async () => {
    setTesting(true);
    setError(null);
    try {
      const result = await testSonarrConnection(url.trim(), apiKey.trim());
      if (result.success) {
        setTested(true);
      } else {
        setTested(false);
        setError(result.error || "Could not reach Sonarr with those details.");
      }
    } catch (err) {
      setTested(false);
      setError(errorMessage(err));
    } finally {
      setTesting(false);
    }
  };

  const handleFinish = async () => {
    setSaving(true);
    setError(null);
    try {
      // Saved last: it flips the "needs onboarding" flag that guards this page.
      await updateSettings.mutateAsync({
        sonarr_url: url.trim(),
        sonarr_api_key: apiKey.trim(),
        sync_interval: syncInterval,
        default_anime_canon_action: trackedToAction(trackCanon),
        default_mixed_canon_action: trackedToAction(trackMixed),
        default_filler_action: trackedToAction(trackFiller),
      });
      navigate("/", { replace: true });
    } catch (err) {
      setError(errorMessage(err));
      setSaving(false);
    }
  };

  // Restore a config exported from another Mizu: it carries the Sonarr
  // connection, tracking defaults and every show mapping, so a reinstall
  // skips straight past setup.
  const handleRestoreFile = async (file: File) => {
    setRestoring(true);
    setError(null);
    try {
      let parsed: unknown;
      try {
        parsed = JSON.parse(await file.text());
      } catch {
        setError("That file isn't valid JSON. Pick a Mizu config export.");
        return;
      }
      const result = await importConfig(parsed);
      const shows = `${result.mappings_imported} show${result.mappings_imported === 1 ? "" : "s"}`;
      const restored = await queryClient.fetchQuery({ ...settingsQuery, staleTime: 0 });
      await queryClient.invalidateQueries({ queryKey: queryKeys.shows });
      if (restored.sonarr_api_key_set) {
        navigate("/", { replace: true });
      } else {
        setUrl(restored.sonarr_url || url);
        setError(`Restored ${shows}, but the backup has no Sonarr API key. Enter it below.`);
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setRestoring(false);
      if (restoreFileRef.current) restoreFileRef.current.value = "";
    }
  };

  const errorLine = error && (
    <Text role="alert" style={{ color: "var(--color-danger)" }}>
      {error}
    </Text>
  );

  return (
    <div className="onboarding-screen">
      <Section padding={4} maxWidth={560} className="section-centered page-enter">
        <VStack gap={5}>
          <VStack gap={2} hAlign="center">
            <Waves size={32} aria-hidden="true" />
            <Heading level={1}>Welcome to Mizu</Heading>
            <Text type="supporting" color="secondary">
              Two quick steps and you can start skipping filler.
            </Text>
          </VStack>

          <ol className="wizard-steps" aria-label="Setup progress">
            {STEPS.map((s, i) => {
              const isCurrent = s.key === step;
              const isPast = stepIndex > i;
              return (
                <li
                  key={s.key}
                  className="wizard-step"
                  data-state={isCurrent ? "current" : isPast ? "past" : "future"}
                  aria-current={isCurrent ? "step" : undefined}
                >
                  <span className="wizard-step-dot">{isPast ? <Check size={12} /> : i + 1}</span>
                  <span className="wizard-step-label">{s.label}</span>
                </li>
              );
            })}
          </ol>

          <Card variant="muted" padding={4}>
            {step === "sonarr" && (
              <VStack gap={4}>
                <Text type="supporting" color="secondary">
                  Your Sonarr address and API key (Sonarr, Settings, General, Security).
                </Text>
                <TextInput
                  label="Sonarr URL"
                  value={url}
                  onChange={editConnection(setUrl)}
                  placeholder="http://localhost:8989"
                />
                <TextInput
                  label="API key"
                  type="password"
                  value={apiKey}
                  onChange={editConnection(setApiKey)}
                  placeholder="Your Sonarr API key"
                />
                {errorLine}
                <HStack gap={2} justify="between" wrap="wrap">
                  <Button
                    label={tested ? "Connected" : "Test connection"}
                    variant="secondary"
                    icon={tested ? <Check size={16} /> : <Plug size={16} />}
                    onClick={handleTest}
                    isLoading={testing}
                    isDisabled={busy || !url.trim() || !apiKey.trim()}
                    data-cuelume-press
                  />
                  <Button
                    label="Continue"
                    variant="primary"
                    onClick={() => {
                      setError(null);
                      setStep("defaults");
                    }}
                    isDisabled={!tested || busy}
                    data-cuelume-press
                  />
                </HStack>
              </VStack>
            )}

            {step === "defaults" && (
              <VStack gap={4}>
                <SettingsRow
                  label="Sync interval"
                  description="How often Mizu checks for episode changes."
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

                <VStack gap={3}>
                  <VStack gap={1}>
                    <Heading level={3}>Tracking defaults</Heading>
                    <Text type="supporting" color="secondary">
                      Applied when you add an anime. You can override them per anime.
                    </Text>
                  </VStack>
                  <SettingsRow label="Canon" description="Main plot episodes">
                    <Switch
                      label="Track canon"
                      value={trackCanon}
                      onChange={setTrackCanon}
                      isLabelHidden
                      data-cuelume-toggle
                    />
                  </SettingsRow>
                  <SettingsRow label="Mixed" description="Mix of canon and filler">
                    <Switch
                      label="Track mixed"
                      value={trackMixed}
                      onChange={setTrackMixed}
                      isLabelHidden
                      data-cuelume-toggle
                    />
                  </SettingsRow>
                  <SettingsRow label="Filler" description="Skippable, no plot impact">
                    <Switch
                      label="Track filler"
                      value={trackFiller}
                      onChange={setTrackFiller}
                      isLabelHidden
                      data-cuelume-toggle
                    />
                  </SettingsRow>
                </VStack>

                <Divider />

                <HStack gap={2} align="center" role="status" aria-live="polite">
                  {afl.phase === "fetching" && (
                    <>
                      <Loader2 size={16} className="spin" aria-hidden="true" />
                      <Text type="supporting" color="secondary">
                        Fetching all available anime from AnimeFillerList…
                      </Text>
                    </>
                  )}
                  {afl.phase === "ready" && (
                    <>
                      <Check size={16} aria-hidden="true" />
                      <Text type="supporting" color="secondary">
                        {afl.count} anime ready to match.
                      </Text>
                    </>
                  )}
                  {afl.phase === "failed" && (
                    <Text type="supporting" color="secondary">
                      Couldn't fetch the anime list ({afl.message}). It will retry on the first
                      sync.
                    </Text>
                  )}
                </HStack>

                {reachedOverNetwork && (
                  <Text type="supporting" color="secondary">
                    You're reaching Mizu over the network and it isn't password protected. Turn on a
                    password in Settings, Security after setup.
                  </Text>
                )}

                {errorLine}
                <HStack gap={2} justify="between" wrap="wrap">
                  <Button
                    label="Back"
                    variant="ghost"
                    onClick={() => setStep("sonarr")}
                    isDisabled={saving}
                    data-cuelume-press
                  />
                  <Button
                    label="Finish and open library"
                    variant="primary"
                    onClick={handleFinish}
                    isLoading={saving}
                    isDisabled={busy}
                    data-cuelume-press
                  />
                </HStack>
              </VStack>
            )}
          </Card>

          {step === "sonarr" && (
            <HStack justify="center">
              <Button
                label="Restore from a backup"
                variant="ghost"
                size="sm"
                icon={<Upload size={14} />}
                isLoading={restoring}
                isDisabled={busy}
                onClick={() => restoreFileRef.current?.click()}
                data-cuelume-press
              />
              <input
                ref={restoreFileRef}
                type="file"
                accept="application/json,.json"
                aria-label="Mizu config backup file"
                style={{ display: "none" }}
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) void handleRestoreFile(file);
                }}
              />
            </HStack>
          )}
        </VStack>
      </Section>
    </div>
  );
}
