import { Controller, type Control } from "react-hook-form";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Button } from "@astryxdesign/core/Button";
import { Spinner } from "@astryxdesign/core/Spinner";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Divider } from "@astryxdesign/core/Divider";
import { Card } from "@astryxdesign/core/Card";
import { Check, Plug, AlertTriangle, Eye, EyeOff } from "lucide-react";
import { SettingsRow } from "../SettingsRow";
import type { SettingsForm } from "../../lib/schemas";
import type { TestPhase } from "./testPhase";

interface ConnectionCardProps {
  control: Control<SettingsForm>;
  sonarrUrlError?: string;
  testPhase: TestPhase;
  testDisabled: boolean;
  apiKeyVisible: boolean;
  hasSavedApiKey: boolean;
  apiKeyDirty: boolean;
  savedApiKeyMasked: string;
  clearTestResult: () => void;
  setApiKeyDirty: (dirty: boolean) => void;
  toggleApiKeyVisibility: () => void;
  handleTestConnection: () => void;
}

export function ConnectionCard({
  control,
  sonarrUrlError,
  testPhase,
  testDisabled,
  apiKeyVisible,
  hasSavedApiKey,
  apiKeyDirty,
  savedApiKeyMasked,
  clearTestResult,
  setApiKeyDirty,
  toggleApiKeyVisibility,
  handleTestConnection,
}: ConnectionCardProps) {
  return (
    <Card variant="muted" padding={4}>
      <VStack gap={3}>
        <Heading level={2}>Connection</Heading>
        <Divider />

        <SettingsRow
          label="Sonarr URL"
          description="Base URL of your Sonarr instance."
          error={sonarrUrlError}
        >
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

        <SettingsRow label="API key" description="Found in Sonarr under Settings, General.">
          <HStack gap={2} align="center">
            <Controller
              control={control}
              name="sonarrApiKey"
              render={({ field }) => (
                <TextInput
                  label="API key"
                  type={apiKeyVisible ? "text" : "password"}
                  value={field.value}
                  onChange={(v) => {
                    field.onChange(v);
                    setApiKeyDirty(true);
                    clearTestResult();
                  }}
                  placeholder={
                    hasSavedApiKey ? savedApiKeyMasked || "Saved API key" : "Your Sonarr API key"
                  }
                  isLabelHidden
                  width={280}
                />
              )}
            />
            {(hasSavedApiKey || apiKeyDirty) && (
              <Button
                label={apiKeyVisible ? "Hide" : "Show"}
                variant="ghost"
                isIconOnly
                icon={apiKeyVisible ? <EyeOff size={16} /> : <Eye size={16} />}
                onClick={toggleApiKeyVisibility}
                data-cuelume-toggle="whisper"
              />
            )}
          </HStack>
        </SettingsRow>
        <Divider />

        <SettingsRow
          label="Test connection"
          description="Check that Mizu can reach Sonarr with the URL and API key above."
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
