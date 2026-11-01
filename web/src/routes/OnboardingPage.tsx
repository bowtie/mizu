import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Section } from "@astryxdesign/core/Section";
import { Card } from "@astryxdesign/core/Card";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Check, Plug, Waves } from "lucide-react";
import { useAuthStatus, useSettings, useUpdateSettings } from "../lib/queries";
import { errorMessage, refreshAflCache, setupPassword, testSonarrConnection } from "../lib/api";
import { skipOnboarding } from "../lib/onboarding";

const MIN_PASSWORD_LENGTH = 8;

type StepKey = "sonarr" | "password" | "done";

const STEPS: { key: StepKey; label: string }[] = [
  { key: "sonarr", label: "Connect Sonarr" },
  { key: "password", label: "Password" },
  { key: "done", label: "Finish" },
];

export function OnboardingPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: settings } = useSettings();
  const { data: authStatus } = useAuthStatus();
  const updateSettings = useUpdateSettings();

  const [step, setStep] = useState<StepKey>("sonarr");
  const [url, setUrl] = useState(settings?.sonarr_url || "http://localhost:8989");
  const [apiKey, setApiKey] = useState("");
  const [tested, setTested] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const stepIndex = STEPS.findIndex((s) => s.key === step);
  const passwordAlreadySet = authStatus?.configured === true;

  const editConnection = (setter: (v: string) => void) => (value: string) => {
    setter(value);
    setTested(false);
    setError(null);
  };

  const handleTest = async () => {
    setBusy(true);
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
      setBusy(false);
    }
  };

  const handleSaveConnection = async () => {
    setBusy(true);
    setError(null);
    try {
      await updateSettings.mutateAsync({
        sonarr_url: url.trim(),
        sonarr_api_key: apiKey.trim(),
      });
      setStep(passwordAlreadySet ? "done" : "password");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  const handleSetPassword = async () => {
    if (password.length < MIN_PASSWORD_LENGTH) {
      setError(`Password must be at least ${MIN_PASSWORD_LENGTH} characters`);
      return;
    }
    if (password !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await setupPassword(password);
      queryClient.setQueryData(["authStatus"], { configured: true, authenticated: true });
      setStep("done");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  const handleFinish = async () => {
    setBusy(true);
    // Warm the AFL title cache so the Add anime wizard has matches ready.
    // Best effort: a failure here must not block entering the app.
    await refreshAflCache().catch(() => undefined);
    navigate("/", { replace: true });
  };

  const handleSkip = () => {
    skipOnboarding();
    navigate("/", { replace: true });
  };

  return (
    <Section padding={4} maxWidth={520} className="section-centered page-enter">
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
                Mizu needs your Sonarr address and API key (Sonarr, Settings, General, Security).
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
              {error && (
                <Text role="alert" style={{ color: "var(--color-danger)" }}>
                  {error}
                </Text>
              )}
              <HStack gap={2} justify="between" wrap="wrap">
                <Button
                  label={tested ? "Connected" : "Test connection"}
                  variant="secondary"
                  icon={tested ? <Check size={16} /> : <Plug size={16} />}
                  onClick={handleTest}
                  isLoading={busy && !tested}
                  isDisabled={busy || !url.trim() || !apiKey.trim()}
                  data-cuelume-press
                />
                <Button
                  label="Continue"
                  variant="primary"
                  onClick={handleSaveConnection}
                  isDisabled={!tested || busy}
                  data-cuelume-press
                />
              </HStack>
            </VStack>
          )}

          {step === "password" && (
            <VStack gap={4}>
              <Text type="supporting" color="secondary">
                Optional: protect the dashboard with a password. Mizu stores your Sonarr key, so
                this is recommended if it is reachable from other devices.
              </Text>
              <TextInput
                label="Password"
                type="password"
                value={password}
                onChange={(v) => {
                  setPassword(v);
                  setError(null);
                }}
                placeholder={`At least ${MIN_PASSWORD_LENGTH} characters`}
              />
              <TextInput
                label="Confirm password"
                type="password"
                value={confirmPassword}
                onChange={(v) => {
                  setConfirmPassword(v);
                  setError(null);
                }}
              />
              {error && (
                <Text role="alert" style={{ color: "var(--color-danger)" }}>
                  {error}
                </Text>
              )}
              <HStack gap={2} justify="between" wrap="wrap">
                <Button
                  label="Skip for now"
                  variant="ghost"
                  onClick={() => {
                    setError(null);
                    setStep("done");
                  }}
                  isDisabled={busy}
                  data-cuelume-press
                />
                <Button
                  label="Set password"
                  variant="primary"
                  onClick={handleSetPassword}
                  isLoading={busy}
                  isDisabled={!password || !confirmPassword}
                  data-cuelume-press
                />
              </HStack>
            </VStack>
          )}

          {step === "done" && (
            <VStack gap={4} hAlign="center">
              <Heading level={2}>You're connected</Heading>
              <Text type="supporting" color="secondary" style={{ textAlign: "center" }}>
                Next, add your first anime from the Library. Mizu will match it against
                AnimeFillerList and apply your tracking rules on every sync.
              </Text>
              <Button
                label="Open library"
                variant="primary"
                onClick={handleFinish}
                isLoading={busy}
                data-cuelume-press
              />
            </VStack>
          )}
        </Card>

        {step !== "done" && (
          <HStack justify="center">
            <Button label="Skip setup" variant="ghost" size="sm" onClick={handleSkip} />
          </HStack>
        )}
      </VStack>
    </Section>
  );
}
