import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Card } from "@astryxdesign/core/Card";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Divider } from "@astryxdesign/core/Divider";
import { KeyRound, LogOut } from "lucide-react";
import { useNotificationStore } from "../lib/store";
import { useAuthStatus } from "../lib/queries";
import { changePassword, logout, errorMessage } from "../lib/api";

const MIN_PASSWORD_LENGTH = 8;

export function SecuritySettingsCard() {
  const { data: status } = useAuthStatus();
  const addNotification = useNotificationStore((s) => s.addNotification);
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);

  // Clear the form when auth is toggled on/off.
  useEffect(() => {
    setCurrentPassword("");
    setNewPassword("");
    setConfirmPassword("");
  }, [status?.configured]);

  if (!status?.configured) return null;

  const handleChangePassword = async () => {
    if (submitting) return;
    if (newPassword.length < MIN_PASSWORD_LENGTH) {
      addNotification(`New password must be at least ${MIN_PASSWORD_LENGTH} characters`, "error");
      return;
    }
    if (newPassword !== confirmPassword) {
      addNotification("New passwords do not match", "error");
      return;
    }
    setSubmitting(true);
    try {
      await changePassword(currentPassword, newPassword);
      addNotification("Password changed — please sign in again", "success");
      queryClient.clear();
      navigate("/login", { replace: true });
    } catch (err) {
      addNotification("Failed to change password: " + errorMessage(err), "error");
    } finally {
      setSubmitting(false);
    }
  };

  const handleLogout = async () => {
    try {
      await logout();
    } catch {
      // Session may already be gone — clearing local state is enough.
    } finally {
      queryClient.clear();
      navigate("/login", { replace: true });
    }
  };

  return (
    <Card variant="muted" padding={4}>
      <VStack gap={3}>
        <Heading level={2}>Security</Heading>
        <Divider />
        <Text type="supporting" color="secondary" display="block">
          Changing the password signs out every device.
        </Text>
        <VStack gap={3} maxWidth={420}>
          <TextInput
            type="password"
            label="Current password"
            value={currentPassword}
            onChange={setCurrentPassword}
            isDisabled={submitting}
          />
          <TextInput
            type="password"
            label="New password"
            value={newPassword}
            onChange={setNewPassword}
            isDisabled={submitting}
          />
          <TextInput
            type="password"
            label="Confirm new password"
            value={confirmPassword}
            onChange={setConfirmPassword}
            isDisabled={submitting}
          />
        </VStack>
        <HStack gap={2} align="center">
          <Button
            variant="primary"
            label="Change password"
            icon={<KeyRound size={16} />}
            isDisabled={submitting || !currentPassword || !newPassword}
            isLoading={submitting}
            onClick={() => void handleChangePassword()}
          >
            Change password
          </Button>
          <Button
            variant="secondary"
            label="Sign out"
            icon={<LogOut size={16} />}
            onClick={() => void handleLogout()}
          >
            Sign out
          </Button>
        </HStack>
      </VStack>
    </Card>
  );
}
