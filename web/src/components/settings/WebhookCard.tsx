import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Card } from "@astryxdesign/core/Card";
import { Heading } from "@astryxdesign/core/Heading";
import { Button } from "@astryxdesign/core/Button";
import { Text } from "@astryxdesign/core/Text";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Divider } from "@astryxdesign/core/Divider";
import { Copy, RefreshCw } from "lucide-react";
import { useNotificationStore } from "../../lib/store";
import { errorMessage, getWebhookSettings, regenerateWebhookToken } from "../../lib/api";
import { useIsMobile } from "../../lib/useIsMobile";

const webhookQueryKey = ["webhookSettings"];
// Wide enough for a LAN URL, narrow enough to fit a phone.
const fieldWidth = "min(100%, 400px)";

/** Sonarr Connect → Webhook setup. The token applies immediately, not
 *  through the page's save bar. */
export function WebhookCard() {
  const { data } = useQuery({ queryKey: webhookQueryKey, queryFn: getWebhookSettings });
  const queryClient = useQueryClient();
  const addNotification = useNotificationStore((s) => s.addNotification);
  const [regenerating, setRegenerating] = useState(false);
  const isMobile = useIsMobile();
  const url = `${window.location.origin}/api/webhook/sonarr`;
  // The clipboard API only exists on HTTPS and localhost; elsewhere the
  // read-only fields can still be selected and copied by hand.
  const canCopy = typeof navigator !== "undefined" && !!navigator.clipboard;

  const copy = async (value: string, what: string) => {
    try {
      await navigator.clipboard.writeText(value);
      addNotification(`${what} copied`, "success");
    } catch (err) {
      addNotification(`Couldn't copy: ${errorMessage(err)}`, "error");
    }
  };

  const regenerate = async () => {
    setRegenerating(true);
    try {
      queryClient.setQueryData(webhookQueryKey, await regenerateWebhookToken());
      addNotification("New password made. Update it in Sonarr.", "success");
    } catch (err) {
      addNotification(`Couldn't make a new password: ${errorMessage(err)}`, "error");
    } finally {
      setRegenerating(false);
    }
  };

  const copyButton = (value: string, what: string) =>
    canCopy && (
      <Button
        variant="secondary"
        label="Copy"
        isIconOnly={isMobile}
        icon={<Copy size={16} />}
        isDisabled={!value}
        onClick={() => void copy(value, what)}
      />
    );

  const newPasswordButton = (
    <Button
      variant="ghost"
      label="New password"
      icon={<RefreshCw size={16} />}
      isLoading={regenerating}
      isDisabled={regenerating || !data}
      onClick={() => void regenerate()}
    />
  );

  return (
    <Card variant="muted" padding={4}>
      <VStack gap={3}>
        <Heading level={2}>Sonarr webhook</Heading>
        <Divider />
        <Text type="supporting" color="secondary" display="block">
          Add a Webhook in Sonarr under Settings → Connect, turn on On Grab and On Series Add, and
          paste these in.
        </Text>
        <HStack gap={2} vAlign="end" wrap="wrap" className="webhook-field-row">
          <TextInput label="URL" value={url} isReadOnly width={fieldWidth} />
          {copyButton(url, "URL")}
        </HStack>
        <HStack gap={2} vAlign="end" wrap="wrap" className="webhook-field-row">
          <TextInput label="Password" value={data?.token ?? ""} isReadOnly width={fieldWidth} />
          {copyButton(data?.token ?? "", "Password")}
          {!isMobile && newPasswordButton}
        </HStack>
        {/* No room beside the field on a phone. */}
        {isMobile && <HStack gap={2}>{newPasswordButton}</HStack>}
      </VStack>
    </Card>
  );
}
