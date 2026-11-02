import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Card } from "@astryxdesign/core/Card";
import { Heading } from "@astryxdesign/core/Heading";
import { Button } from "@astryxdesign/core/Button";
import { Selector } from "@astryxdesign/core/Selector";
import { Text } from "@astryxdesign/core/Text";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Divider } from "@astryxdesign/core/Divider";
import { KeyRound, ShieldCheck } from "lucide-react";
import { useNotificationStore } from "../lib/store";
import { useAuthStatus } from "../lib/queries";
import {
  changePassword,
  errorMessage,
  getAccessSettings,
  updateAccessSettings,
  type AuthMode,
} from "../lib/api";
import type { SelectOption } from "../lib/types";
import { SettingsRow } from "./SettingsRow";

export const MIN_PASSWORD_LENGTH = 8;

const modeOptions: SelectOption[] = [
  { value: "password", label: "Mizu password" },
  { value: "external", label: "Sign-in proxy" },
];

type Panel = "change" | "access" | null;

/** Password and sign-in settings. Changes here apply immediately (they
 *  have their own confirm buttons), not through the page's save bar. */
export function SecuritySettingsCard() {
  const { data: status } = useAuthStatus();
  const { data: access, refetch: refetchAccess } = useQuery({
    queryKey: ["authAccess"],
    queryFn: getAccessSettings,
    enabled: !!status?.authenticated,
  });
  const addNotification = useNotificationStore((s) => s.addNotification);
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const [panel, setPanel] = useState<Panel>(null);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [mode, setMode] = useState<AuthMode>("password");
  const [trustedProxies, setTrustedProxies] = useState("");
  const [submitting, setSubmitting] = useState(false);

  // Start every panel with empty password fields and the saved values.
  useEffect(() => {
    setCurrentPassword("");
    setNewPassword("");
    setConfirmPassword("");
    if (access) {
      setMode(access.mode);
      setTrustedProxies(access.trusted_proxies);
    }
  }, [panel, access]);

  if (!status) return null;
  const external = !!status.external;

  const run = async (action: () => Promise<void>, failure: string) => {
    if (submitting) return;
    setSubmitting(true);
    try {
      await action();
    } catch (err) {
      addNotification(`${failure}: ${errorMessage(err)}`, "error");
    } finally {
      setSubmitting(false);
    }
  };

  const handleChange = () =>
    run(async () => {
      if (newPassword.length < MIN_PASSWORD_LENGTH) {
        addNotification(`Password must be at least ${MIN_PASSWORD_LENGTH} characters`, "error");
        return;
      }
      if (newPassword !== confirmPassword) {
        addNotification("Passwords don't match", "error");
        return;
      }
      await changePassword(currentPassword, newPassword);
      addNotification("Password changed. Please sign in again.", "success");
      queryClient.clear();
      navigate("/login", { replace: true });
    }, "Couldn't change password");

  const handleAccess = () =>
    run(async () => {
      await updateAccessSettings({
        mode,
        trusted_proxies: trustedProxies,
        current_password: currentPassword,
      });
      setPanel(null);
      void refetchAccess();
      if (mode === "external") {
        queryClient.setQueryData(["authStatus"], {
          configured: true,
          authenticated: true,
          external: true,
        });
        addNotification("Sign-in is now handled by your proxy", "success");
      } else if (external) {
        addNotification("Mizu asks for its own password again. Please sign in.", "success");
        queryClient.clear();
        navigate("/login", { replace: true });
      } else {
        addNotification("Trusted proxies saved", "success");
      }
    }, "Couldn't update sign-in");

  const cancel = (
    <Button variant="ghost" label="Cancel" isDisabled={submitting} onClick={() => setPanel(null)} />
  );

  const currentPasswordField = (
    <TextInput
      type="password"
      label="Current password"
      value={currentPassword}
      onChange={setCurrentPassword}
      isDisabled={submitting}
    />
  );

  return (
    <Card variant="muted" padding={4}>
      <VStack gap={3}>
        <Heading level={2}>Security</Heading>
        <Divider />

        {!external && (
          <SettingsRow
            label="Password"
            description="Mizu asks for this password before showing anything."
          >
            <Button
              variant="secondary"
              label="Change password"
              icon={<KeyRound size={16} />}
              isDisabled={panel === "change" || submitting}
              onClick={() => setPanel("change")}
              data-cuelume-press
            />
          </SettingsRow>
        )}

        {panel === "change" && (
          <VStack gap={3} maxWidth={420}>
            {currentPasswordField}
            <TextInput
              type="password"
              label="New password"
              value={newPassword}
              onChange={setNewPassword}
              placeholder={`At least ${MIN_PASSWORD_LENGTH} characters`}
              isDisabled={submitting}
            />
            <TextInput
              type="password"
              label="Confirm password"
              value={confirmPassword}
              onChange={setConfirmPassword}
              isDisabled={submitting}
            />
            <HStack gap={2}>
              <Button
                variant="primary"
                label="Change password"
                isLoading={submitting}
                isDisabled={submitting || !currentPassword || !newPassword || !confirmPassword}
                onClick={() => void handleChange()}
              />
              {cancel}
            </HStack>
          </VStack>
        )}

        {!external && <Divider />}

        <SettingsRow
          label="Sign-in"
          description={
            external
              ? "Handled by your sign-in proxy. Mizu only answers requests from the trusted proxy addresses."
              : "Mizu's own password. If a sign-in proxy (tinyauth, Authelia, Authentik…) already protects Mizu, it can take over."
          }
        >
          <Button
            variant="secondary"
            label="Change"
            icon={<ShieldCheck size={16} />}
            isDisabled={panel === "access" || submitting || !access}
            onClick={() => setPanel("access")}
            data-cuelume-press
          />
        </SettingsRow>

        {panel === "access" && access && (
          <VStack gap={3} maxWidth={480}>
            <Selector
              label="Sign-in method"
              value={mode}
              options={modeOptions}
              onChange={(v) => setMode(v as AuthMode)}
              isDisabled={submitting}
            />
            <TextInput
              label="Trusted proxy addresses"
              value={trustedProxies}
              onChange={setTrustedProxies}
              placeholder="e.g. 172.18.0.5, 172.18.0.0/16"
              isDisabled={submitting}
            />
            <Text type="supporting" color="secondary">
              Reverse proxies in front of Mizu, as IP addresses or networks. Their forwarded client
              addresses are trusted, and with a sign-in proxy they are the only addresses Mizu
              answers. This browser reaches Mizu from {access.client_address}
              {access.via_trusted_proxy ? ", a trusted proxy." : "."}
            </Text>
            {currentPasswordField}
            <HStack gap={2}>
              <Button
                variant="primary"
                label="Save sign-in"
                isLoading={submitting}
                isDisabled={submitting || !currentPassword}
                onClick={() => void handleAccess()}
              />
              {cancel}
            </HStack>
          </VStack>
        )}
      </VStack>
    </Card>
  );
}
