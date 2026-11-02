import { Controller, type Control } from "react-hook-form";
import { VStack } from "@astryxdesign/core/VStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Button } from "@astryxdesign/core/Button";
import { Spinner } from "@astryxdesign/core/Spinner";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Divider } from "@astryxdesign/core/Divider";
import { Card } from "@astryxdesign/core/Card";
import { Check, Plug, AlertTriangle } from "lucide-react";
import { SettingsRow } from "../SettingsRow";
import { SecretField } from "./SecretField";
import type { SettingsForm } from "../../lib/schemas";
import type { TestPhase } from "./testPhase";

interface ConnectionCardProps {
  control: Control<SettingsForm>;
  sonarrUrlError?: string;
  testPhase: TestPhase;
  testDisabled: boolean;
  hasSavedApiKey: boolean;
  apiKeyDirty: boolean;
  savedApiKeyMasked: string;
  clearTestResult: () => void;
  setApiKeyDirty: (dirty: boolean) => void;
  handleTestConnection: () => void;
}

export function ConnectionCard({
  control,
  sonarrUrlError,
  testPhase,
  testDisabled,
  hasSavedApiKey,
  apiKeyDirty,
  savedApiKeyMasked,
  clearTestResult,
  setApiKeyDirty,
  handleTestConnection,
}: ConnectionCardProps) {
  return (
    <Card variant="muted" padding={4}>
      <VStack gap={3}>
        <Heading level={2}>Connection</Heading>
        <Divider />

        <SettingsRow label="Sonarr URL" description="Your Sonarr address." error={sonarrUrlError}>
          <Controller
            control={control}
            name="sonarrUrl"
            render={({ field }) => (
              <TextInput
                label="Sonarr URL"
                value={field.value}
                onChange={(v) => {
                  field.onChange(v);
                  clearTestResult();
                }}
                placeholder="http://localhost:8989"
                isLabelHidden
                width={320}
              />
            )}
          />
        </SettingsRow>
        <Divider />

        <SettingsRow label="API key" description="In Sonarr under Settings, General.">
          <SecretField
            control={control}
            name="sonarrApiKey"
            label="API key"
            isSet={hasSavedApiKey}
            dirty={apiKeyDirty}
            savedMasked={savedApiKeyMasked}
            emptyPlaceholder="Your Sonarr API key"
            width={280}
            onDirtyChange={setApiKeyDirty}
            onEdit={clearTestResult}
          />
        </SettingsRow>
        <Divider />

        <SettingsRow label="Test connection" description="Check that Mizu can reach Sonarr.">
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
            variant={testPhase === "error" ? "destructive" : "secondary"}
            icon={
              testPhase === "connecting" ? (
                <Spinner size="sm" />
              ) : testPhase === "success" ? (
                <Check size={16} color="var(--color-on-success, #fff)" />
              ) : testPhase === "error" ? (
                <AlertTriangle size={16} />
              ) : (
                <Plug size={16} />
              )
            }
            onClick={handleTestConnection}
            isDisabled={testDisabled}
            className={testPhase !== "idle" ? "test-status-btn" : undefined}
            data-test-success={testPhase === "success" ? "true" : undefined}
            data-cuelume-press
          />
        </SettingsRow>
      </VStack>
    </Card>
  );
}
