import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Card } from "@astryxdesign/core/Card";
import { Heading } from "@astryxdesign/core/Heading";
import { Button } from "@astryxdesign/core/Button";
import { Collapsible } from "@astryxdesign/core/Collapsible";
import { RadioList, RadioListItem } from "@astryxdesign/core/RadioList";
import { Text } from "@astryxdesign/core/Text";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Divider } from "@astryxdesign/core/Divider";
import { KeyRound, ShieldCheck } from "lucide-react";
import { useNotificationStore } from "../lib/store";
import { queryKeys, useAuthStatus } from "../lib/queries";
import {
  changePassword,
  errorMessage,
  getAccessSettings,
  updateAccessSettings,
  type AccessSettings,
  type AuthMode,
} from "../lib/api";
import { SettingsRow } from "./SettingsRow";

const MIN_PASSWORD_LENGTH = 8;

const modes: Record<AuthMode, { label: string; description: string; summary: string }> = {
  password: {
    label: "Mizu password",
    description: "Mizu asks for its own password.",
    summary: "Mizu asks for its own password.",
  },
  external: {
    label: "Sign-in proxy",
    description: "tinyauth, Authelia, Authentik or similar already protects Mizu.",
    summary: "Handled by your sign-in proxy.",
  },
  open: {
    label: "No sign-in",
    description: "Something else keeps strangers out, like a VPN or a private network.",
    summary: "Off. Anyone who can reach Mizu can use it.",
  },
};

const savedMessage: Record<AuthMode, string> = {
  password: "Password sign-in is on",
  external: "Your proxy now handles sign-in",
  open: "Sign-in is off",
};

/** The address to suggest trusting: the peer, when it forwarded the request
 *  and isn't trusted yet. */
function suggestedProxy(access: AccessSettings): string | null {
  return access.forwarded && !access.via_trusted_proxy ? access.client_address : null;
}

/** Whether host is a name open mode has to list, rather than an IP address
 *  or localhost, which it always answers. */
function isNamedHost(host: string): boolean {
  return !(
    host === "localhost" ||
    host.endsWith(".localhost") ||
    host.includes(":") ||
    /^\d+\.\d+\.\d+\.\d+$/.test(host)
  );
}

type Panel = "change" | "access" | null;

/** Password and sign-in settings. Changes here apply immediately (they
 *  have their own confirm buttons), not through the page's save bar. */
export function SecuritySettingsCard() {
  const { data: status } = useAuthStatus();
  const { data: access } = useQuery({
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
  const [allowedHosts, setAllowedHosts] = useState("");
  const [submitting, setSubmitting] = useState(false);

  // Start every panel with empty password fields and the saved values.
  useEffect(() => {
    setCurrentPassword("");
    setNewPassword("");
    setConfirmPassword("");
    if (access) {
      setMode(access.mode);
      setTrustedProxies(access.trusted_proxies);
      setAllowedHosts(access.allowed_hosts);
    }
  }, [panel, access]);

  if (!status) return null;
  const savedMode = status.mode;
  // The server asks for the password while one exists, except with
  // sign-in off; switching to password sign-in without one sets a new one.
  const needsCurrentPassword = !!access?.password_set && savedMode !== "open";
  const needsNewPassword = mode === "password" && !access?.password_set;
  const suggestion = access ? suggestedProxy(access) : null;

  const chooseMode = (next: AuthMode) => {
    setMode(next);
    // A sign-in proxy has to be trusted; offer the one this browser came
    // through rather than asking for its address.
    if (next === "external" && !trustedProxies.trim() && suggestion) {
      setTrustedProxies(suggestion);
    }
  };

  const trustSuggestion = () => {
    if (!suggestion) return;
    setTrustedProxies((prev) => (prev.trim() ? `${prev.trim()}, ${suggestion}` : suggestion));
  };

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

  const checkNewPassword = () => {
    if (newPassword.length < MIN_PASSWORD_LENGTH) {
      addNotification(`Password must be at least ${MIN_PASSWORD_LENGTH} characters`, "error");
      return false;
    }
    if (newPassword !== confirmPassword) {
      addNotification("Passwords don't match", "error");
      return false;
    }
    return true;
  };

  const handleChange = () =>
    run(async () => {
      if (!checkNewPassword()) return;
      await changePassword(currentPassword, newPassword);
      addNotification("Password changed. Sign in again.", "success");
      queryClient.clear();
      navigate("/login", { replace: true });
    }, "Couldn't change password");

  const handleAccess = () =>
    run(async () => {
      if (needsNewPassword && !checkNewPassword()) return;
      const { status: nextStatus, ...saved } = await updateAccessSettings({
        mode,
        trusted_proxies: trustedProxies,
        allowed_hosts: mode === "open" ? allowedHosts : (access?.allowed_hosts ?? ""),
        current_password: currentPassword,
        new_password: needsNewPassword ? newPassword : "",
      });
      setPanel(null);
      if (!nextStatus.authenticated) {
        addNotification(`${savedMessage[mode]}. Sign in again.`, "success");
        queryClient.clear();
        navigate("/login", { replace: true });
        return;
      }
      queryClient.setQueryData(queryKeys.authStatus, nextStatus);
      queryClient.setQueryData(["authAccess"], saved);
      addNotification(mode === savedMode ? "Sign-in saved" : savedMessage[mode], "success");
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

  const newPasswordFields = (
    <>
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
    </>
  );

  const trustedProxiesField = (label: string) => (
    <VStack gap={2}>
      <TextInput
        label={label}
        value={trustedProxies}
        onChange={setTrustedProxies}
        placeholder="e.g. 172.18.0.5 or 172.18.0.0/16"
        isDisabled={submitting}
      />
      {suggestion && !trustedProxies.split(/[\s,]+/).includes(suggestion) && (
        <HStack gap={2} vAlign="center" wrap="wrap">
          <Text type="supporting" color="secondary">
            You're coming through a proxy at {suggestion}.
          </Text>
          <Button
            variant="ghost"
            size="sm"
            label="Trust it"
            isDisabled={submitting}
            onClick={trustSuggestion}
          />
        </HStack>
      )}
    </VStack>
  );

  // Each mode shows only what it needs.
  const modeFields = access && {
    password: (
      <>
        {needsNewPassword && newPasswordFields}
        <Collapsible
          trigger={<Text type="supporting">Behind a reverse proxy?</Text>}
          defaultIsOpen={!!access.trusted_proxies || !!suggestion}
        >
          <VStack gap={2}>
            <Text type="supporting" color="secondary">
              Trust it so Mizu sees real client addresses and knows when you're on HTTPS.
            </Text>
            {trustedProxiesField("Trusted proxies")}
          </VStack>
        </Collapsible>
      </>
    ),
    external: (
      <>
        {!access.forwarded && !access.via_trusted_proxy && (
          <Text type="supporting" color="secondary">
            This browser isn't reaching Mizu through a proxy. Open Mizu at your sign-in proxy's
            address, then switch.
          </Text>
        )}
        {trustedProxiesField("Sign-in proxy address")}
        <Text type="supporting" color="secondary">
          Mizu will only answer requests from this address.
        </Text>
      </>
    ),
    open: (
      <>
        <Text type="supporting" color="secondary">
          Mizu keeps answering at
          {isNamedHost(access.host) && ` ${access.host},`} its IP address and localhost.
          {savedMode !== "open" && access.password_set && " Your password will be removed."}
        </Text>
        <TextInput
          label="Other hostnames"
          value={allowedHosts}
          onChange={setAllowedHosts}
          placeholder="e.g. mizu.lan, nas.local"
          isOptional
          isDisabled={submitting}
        />
      </>
    ),
  };

  return (
    <Card variant="muted" padding={4}>
      <VStack gap={3}>
        <Heading level={2}>Security</Heading>
        <Divider />

        <SettingsRow label="Sign-in" description={modes[savedMode].summary}>
          <Button
            variant="secondary"
            label="Change"
            icon={<ShieldCheck size={16} />}
            isDisabled={panel === "access" || submitting || !access}
            onClick={() => setPanel("access")}
            data-cuelume-press
          />
        </SettingsRow>

        {panel === "access" && access && modeFields && (
          <VStack gap={3} maxWidth={480}>
            <RadioList
              label="Sign-in method"
              isLabelHidden
              value={mode}
              onChange={(v) => chooseMode(v as AuthMode)}
              isDisabled={submitting}
            >
              {(Object.keys(modes) as AuthMode[]).map((m) => (
                <RadioListItem
                  key={m}
                  value={m}
                  label={modes[m].label}
                  description={modes[m].description}
                />
              ))}
            </RadioList>
            {modeFields[mode]}
            {needsCurrentPassword && currentPasswordField}
            <HStack gap={2}>
              <Button
                variant="primary"
                label="Save"
                isLoading={submitting}
                isDisabled={
                  submitting ||
                  (needsCurrentPassword && !currentPassword) ||
                  (needsNewPassword && (!newPassword || !confirmPassword)) ||
                  (mode === "external" && !trustedProxies.trim())
                }
                onClick={() => void handleAccess()}
              />
              {cancel}
            </HStack>
          </VStack>
        )}

        {savedMode === "password" && (
          <>
            <Divider />
            <SettingsRow label="Password" description="Needed to open Mizu.">
              <Button
                variant="secondary"
                label="Change password"
                icon={<KeyRound size={16} />}
                isDisabled={panel === "change" || submitting}
                onClick={() => setPanel("change")}
                data-cuelume-press
              />
            </SettingsRow>
          </>
        )}

        {panel === "change" && (
          <VStack gap={3} maxWidth={420}>
            {currentPasswordField}
            {newPasswordFields}
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
      </VStack>
    </Card>
  );
}
