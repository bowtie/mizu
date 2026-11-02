import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Card } from "@astryxdesign/core/Card";
import { Heading } from "@astryxdesign/core/Heading";
import { Button } from "@astryxdesign/core/Button";
import { Switch } from "@astryxdesign/core/Switch";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Divider } from "@astryxdesign/core/Divider";
import { KeyRound } from "lucide-react";
import { useNotificationStore } from "../lib/store";
import { useAuthStatus } from "../lib/queries";
import { changePassword, errorMessage } from "../lib/api";
import { SettingsRow } from "./SettingsRow";

export const MIN_PASSWORD_LENGTH = 8;

/** A password change staged on the toggle; applied by the page's save bar. */
export type SecurityIntent = "enable" | "disable" | null;

interface SecuritySettingsCardProps {
  intent: SecurityIntent;
  onIntentChange: (intent: SecurityIntent) => void;
  password: string;
  confirm: string;
  onPasswordChange: (value: string) => void;
  onConfirmChange: (value: string) => void;
  isDisabled?: boolean;
}

type Panel = "change" | null;

export function SecuritySettingsCard({
  intent,
  onIntentChange,
  password,
  confirm,
  onPasswordChange,
  onConfirmChange,
  isDisabled = false,
}: SecuritySettingsCardProps) {
  const { data: status } = useAuthStatus();
  const addNotification = useNotificationStore((s) => s.addNotification);
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const [panel, setPanel] = useState<Panel>(null);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);

  // Start every panel with empty fields.
  useEffect(() => {
    setCurrentPassword("");
    setNewPassword("");
    setConfirmPassword("");
  }, [panel, status?.configured]);

  if (!status) return null;

  const card = (children: React.ReactNode) => (
    <Card variant="muted" padding={4}>
      <VStack gap={3}>
        <Heading level={2}>Security</Heading>
        <Divider />
        {children}
      </VStack>
    </Card>
  );

  const configured = status.configured;

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

  const validNew = () => {
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
      if (!validNew()) return;
      await changePassword(currentPassword, newPassword);
      addNotification("Password changed. Please sign in again.", "success");
      queryClient.clear();
      navigate("/login", { replace: true });
    }, "Couldn't change password");

  const cancel = (
    <Button variant="ghost" label="Cancel" isDisabled={submitting} onClick={() => setPanel(null)} />
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

  const currentPasswordField = (
    <TextInput
      type="password"
      label="Current password"
      value={currentPassword}
      onChange={setCurrentPassword}
      isDisabled={submitting}
    />
  );

  const switchOn = intent === "enable" ? true : intent === "disable" ? false : configured;

  return card(
    <>
      <SettingsRow
        label="Require a password"
        description={
          configured
            ? "Mizu asks for a password before showing anything."
            : "Off: anyone who can reach Mizu can use it. Fine for a local lab."
        }
      >
        <HStack gap={3} align="center">
          {configured && (
            <Button
              variant="secondary"
              label="Change password"
              icon={<KeyRound size={16} />}
              isDisabled={panel === "change" || submitting}
              onClick={() => setPanel("change")}
              data-cuelume-press
            />
          )}
          <Switch
            label="Require a password"
            value={switchOn}
            isLabelHidden
            isDisabled={submitting || isDisabled}
            onChange={(on: boolean) => {
              if (on === configured) onIntentChange(null);
              else onIntentChange(on ? "enable" : "disable");
            }}
            data-cuelume-toggle
          />
        </HStack>
      </SettingsRow>

      {intent === "enable" && (
        <VStack gap={3} maxWidth={420}>
          <TextInput
            type="password"
            label="New password"
            value={password}
            onChange={onPasswordChange}
            placeholder={`At least ${MIN_PASSWORD_LENGTH} characters`}
            isDisabled={isDisabled}
          />
          <TextInput
            type="password"
            label="Confirm password"
            value={confirm}
            onChange={onConfirmChange}
            isDisabled={isDisabled}
          />
        </VStack>
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
    </>,
  );
}
