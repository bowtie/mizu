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
import { Check, Loader2, Plug, ShieldCheck, Upload, Waves } from "lucide-react";
import { SettingsRow } from "../components/SettingsRow";
import { syncIntervalOptions } from "../components/settings/SyncCard";
import {
  queryKeys,
  settingsQuery,
  useAflCacheRefreshJob,
  useAuthStatus,
  useSettings,
  useUpdateSettings,
} from "../lib/queries";
import {
  errorMessage,
  importConfig,
  setupOpen,
  setupPassword,
  testSonarrConnection,
  type AuthStatus,
} from "../lib/api";
import { trackedToAction } from "./addWizardHelpers";

type StepKey = "sonarr" | "defaults" | "signin";

const ALL_STEPS: { key: StepKey; label: string }[] = [
  { key: "sonarr", label: "Connect Sonarr" },
  { key: "defaults", label: "Sync and defaults" },
  { key: "signin", label: "Sign-in" },
];

const MIN_PASSWORD_LENGTH = 8;

type AflState =
  | { phase: "idle" }
  | { phase: "fetching" }
  | { phase: "ready"; count: number }
  | { phase: "failed"; message: string };

const AFL_POLL_MS = 1000;
const AFL_COUNT_ATTEMPTS = 4;

// One-time setup: this page is only reachable until Sonarr is connected and
// a sign-in choice is made (see OnboardingGate in App.tsx); everything here
// can be changed later in Settings. On a fresh install the last step picks a
// password or skips it, and the server answers without sign-in until then.
// Settings are saved on finishing, never per step.
export function OnboardingPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: settings } = useSettings();
  const { data: authStatus } = useAuthStatus();
  const updateSettings = useUpdateSettings();

  const needsSignIn = authStatus?.configured === false;
  const steps = needsSignIn ? ALL_STEPS : ALL_STEPS.slice(0, 2);
  // Sonarr is already saved when a finish got as far as saving settings but
  // not the sign-in choice, or a backup was restored: only sign-in is left.
  const settingsSaved = useRef(settings?.sonarr_api_key_set === true);

  const [step, setStep] = useState<StepKey>(() =>
    needsSignIn && settingsSaved.current ? "signin" : "sonarr",
  );
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
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

  const stepIndex = steps.findIndex((s) => s.key === step);
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
    settingsSaved.current = false;
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

  // Saves Sonarr and the defaults (unless already saved), then the sign-in
  // choice, which ends first-run setup.
  const handleFinish = async (signIn?: { password: string } | "skip") => {
    setSaving(true);
    setError(null);
    try {
      if (!settingsSaved.current) {
        await updateSettings.mutateAsync({
          sonarr_url: url.trim(),
          sonarr_api_key: apiKey.trim(),
          sync_interval: syncInterval,
          default_anime_canon_action: trackedToAction(trackCanon),
          default_mixed_canon_action: trackedToAction(trackMixed),
          default_filler_action: trackedToAction(trackFiller),
        });
        settingsSaved.current = true;
      }
      if (signIn === "skip") {
        await setupOpen();
        queryClient.setQueryData<AuthStatus>(queryKeys.authStatus, {
          configured: true,
          authenticated: true,
          mode: "open",
        });
      } else if (signIn) {
        await setupPassword(signIn.password);
        queryClient.setQueryData<AuthStatus>(queryKeys.authStatus, {
          configured: true,
          authenticated: true,
          mode: "password",
        });
      }
      navigate("/", { replace: true });
    } catch (err) {
      setError(errorMessage(err));
      setSaving(false);
    }
  };

  const handlePasswordFinish = (e?: React.FormEvent) => {
    e?.preventDefault();
    if (busy) return;
    if (password.length < MIN_PASSWORD_LENGTH) {
      setError(`Password must be at least ${MIN_PASSWORD_LENGTH} characters`);
      return;
    }
    if (password !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }
    void handleFinish({ password });
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
        setError("That's not a Mizu export file.");
        return;
      }
      const result = await importConfig(parsed);
      const shows = `${result.mappings_imported} show${result.mappings_imported === 1 ? "" : "s"}`;
      const restored = await queryClient.fetchQuery({ ...settingsQuery, staleTime: 0 });
      await queryClient.invalidateQueries({ queryKey: queryKeys.shows });
      if (restored.sonarr_api_key_set) {
        settingsSaved.current = true;
        if (needsSignIn) setStep("signin");
        else navigate("/", { replace: true });
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
              {needsSignIn ? "Three" : "Two"} quick steps and you can start skipping filler.
            </Text>
          </VStack>

          <ol className="wizard-steps" aria-label="Setup progress">
            {steps.map((s, i) => {
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
                  The API key is in Sonarr under Settings, General.
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
                <SettingsRow label="Sync interval" description="How often Mizu syncs with Sonarr.">
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
                      Applied to new anime. You can change them per anime.
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
                        Fetching anime from AnimeFillerList…
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

                {errorLine}
                <HStack gap={2} justify="between" wrap="wrap">
                  <Button
                    label="Back"
                    variant="ghost"
                    onClick={() => setStep("sonarr")}
                    isDisabled={saving}
                    data-cuelume-press
                  />
                  {needsSignIn ? (
                    <Button
                      label="Continue"
                      variant="primary"
                      onClick={() => {
                        setError(null);
                        setStep("signin");
                      }}
                      isDisabled={busy}
                      data-cuelume-press
                    />
                  ) : (
                    <Button
                      label="Finish and open library"
                      variant="primary"
                      onClick={() => void handleFinish()}
                      isLoading={saving}
                      isDisabled={busy}
                      data-cuelume-press
                    />
                  )}
                </HStack>
              </VStack>
            )}

            {step === "signin" && (
              <VStack gap={4} as="form" onSubmit={handlePasswordFinish}>
                <Text type="supporting" color="secondary">
                  Set a password to protect the dashboard.
                </Text>
                <TextInput
                  label="Password"
                  type="password"
                  value={password}
                  onChange={setPassword}
                  placeholder="Choose a password"
                  hasAutoFocus
                  isDisabled={saving}
                />
                <TextInput
                  label="Confirm password"
                  type="password"
                  value={confirmPassword}
                  onChange={setConfirmPassword}
                  placeholder="Repeat your password"
                  isDisabled={saving}
                />
                {errorLine}
                <HStack gap={2} justify="between" wrap="wrap">
                  <Button
                    label="Back"
                    variant="ghost"
                    onClick={() => {
                      setError(null);
                      setStep("defaults");
                    }}
                    isDisabled={saving}
                    data-cuelume-press
                  />
                  <HStack gap={2} wrap="wrap">
                    <Button
                      label="Skip"
                      variant="secondary"
                      onClick={() => void handleFinish("skip")}
                      isDisabled={busy}
                      data-cuelume-press
                    />
                    <Button
                      type="submit"
                      label="Finish and open library"
                      variant="primary"
                      icon={<ShieldCheck size={16} />}
                      isLoading={saving}
                      isDisabled={busy || !password}
                      data-cuelume-press
                    />
                  </HStack>
                </HStack>
                <Text size="xsm" color="secondary">
                  Skip only behind a VPN or private network. You can add a password later in
                  Settings.
                </Text>
              </VStack>
            )}
          </Card>

          {step === "sonarr" && (
            <HStack justify="center">
              <Button
                label="Restore from a backup"
                variant="secondary"
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
