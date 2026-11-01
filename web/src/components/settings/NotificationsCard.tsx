import { Controller, type Control } from "react-hook-form";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Button } from "@astryxdesign/core/Button";
import { Badge } from "@astryxdesign/core/Badge";
import { Spinner } from "@astryxdesign/core/Spinner";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Switch } from "@astryxdesign/core/Switch";
import { Selector } from "@astryxdesign/core/Selector";
import { Divider } from "@astryxdesign/core/Divider";
import { Card } from "@astryxdesign/core/Card";
import { Check, Plug, AlertTriangle } from "lucide-react";
import { SettingsRow } from "../SettingsRow";
import type { SettingsForm } from "../../lib/schemas";
import type { SelectOption } from "../../lib/types";
import type { TestPhase } from "./testPhase";

const notifyOnOptions: SelectOption[] = [
  { value: "always", label: "Every sync" },
  { value: "changes", label: "Changes or failures" },
  { value: "failures", label: "Failures only" },
];

interface NotificationsCardProps {
  control: Control<SettingsForm>;
  ntfyTokenSet: boolean;
  ntfyTokenDirty: boolean;
  ntfyTestPhase: TestPhase;
  ntfyTestDisabled: boolean;
  setNtfyTokenDirty: (dirty: boolean) => void;
  handleTestNtfy: () => void;
}

export function NotificationsCard({
  control,
  ntfyTokenSet,
  ntfyTokenDirty,
  ntfyTestPhase,
  ntfyTestDisabled,
  setNtfyTokenDirty,
  handleTestNtfy,
}: NotificationsCardProps) {
  return (
    <Card variant="muted" padding={4}>
      <VStack gap={3}>
        <Heading level={2}>Notifications</Heading>
        <Divider />

        <SettingsRow
          label="Enable notifications"
          description="Get a push notification after each sync via ntfy.sh."
        >
          <Controller
            control={control}
            name="ntfyEnabled"
            render={({ field }) => (
              <Switch
                label="Enable notifications"
                value={field.value}
                onChange={field.onChange}
                isLabelHidden
                data-cuelume-toggle
              />
            )}
          />
        </SettingsRow>
        <Divider />

        <SettingsRow label="Notify on" description="Which sync results should send a notification.">
          <Controller
            control={control}
            name="notifyOn"
            render={({ field }) => (
              <Selector
                label="Notify on"
                value={field.value}
                options={notifyOnOptions}
                onChange={(v) => field.onChange(v as "always" | "changes" | "failures")}
                isLabelHidden
                width={220}
              />
            )}
          />
        </SettingsRow>
        <Divider />

        <SettingsRow
          label="Server URL"
          description="ntfy server URL (self-hosted or public ntfy.sh)."
        >
          <Controller
            control={control}
            name="ntfyServerUrl"
            render={({ field }) => (
              <TextInput
                label="Server URL"
                value={field.value}
                onChange={field.onChange}
                placeholder="https://ntfy.sh"
                isLabelHidden
                width={320}
              />
            )}
          />
        </SettingsRow>
        <Divider />

        <SettingsRow label="Topic" description="ntfy topic name to publish to.">
          <Controller
            control={control}
            name="ntfyTopic"
            render={({ field }) => (
              <TextInput
                label="Topic"
                value={field.value}
                onChange={field.onChange}
                placeholder="my-anime-sync"
                isLabelHidden
                width={320}
              />
            )}
          />
        </SettingsRow>
        <Divider />

        <SettingsRow
          label="Access token"
          description="Optional — only needed if your ntfy server requires authentication."
        >
          <HStack gap={2} align="center">
            <Controller
              control={control}
              name="ntfyToken"
              render={({ field }) => (
                <TextInput
                  label="Access token"
                  type="password"
                  value={field.value}
                  onChange={(v) => {
                    field.onChange(v);
                    setNtfyTokenDirty(true);
                  }}
                  placeholder={ntfyTokenSet ? "Saved token" : "Enter access token"}
                  isLabelHidden
                  width={260}
                />
              )}
            />
            {ntfyTokenSet && !ntfyTokenDirty && <Badge variant="success" label="Token set" />}
          </HStack>
        </SettingsRow>
        <Divider />

        <SettingsRow
          label="Test notification"
          description="Send a test notification to verify your configuration."
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
            variant={ntfyTestPhase === "error" ? "destructive" : "secondary"}
            icon={
              ntfyTestPhase === "connecting" ? (
                <Spinner size="sm" />
              ) : ntfyTestPhase === "success" ? (
                <Check size={16} color="var(--color-on-success, #fff)" />
              ) : ntfyTestPhase === "error" ? (
                <AlertTriangle size={16} />
              ) : (
                <Plug size={16} />
              )
            }
            onClick={handleTestNtfy}
            isDisabled={ntfyTestDisabled}
            className={ntfyTestPhase !== "idle" ? "test-status-btn" : undefined}
            data-test-success={ntfyTestPhase === "success" ? "true" : undefined}
            data-cuelume-press
          />
        </SettingsRow>
      </VStack>
    </Card>
  );
}
