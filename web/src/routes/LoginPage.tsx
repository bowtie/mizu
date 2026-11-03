import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { VStack } from "@astryxdesign/core/VStack";
import { Card } from "@astryxdesign/core/Card";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Spinner } from "@astryxdesign/core/Spinner";
import { Waves, LogIn } from "lucide-react";
import { getAuthStatus, login, errorMessage } from "../lib/api";
import type { AuthStatus } from "../lib/api";
import { takeReturnPath } from "../lib/redirect";

// Signing in to a Mizu that has a password. A fresh install never lands
// here: its sign-in choice is the welcome flow's last step.
export function LoginPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [status, setStatus] = useState<AuthStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [password, setPassword] = useState("");

  useEffect(() => {
    let cancelled = false;
    getAuthStatus()
      .then((s) => {
        if (!cancelled) setStatus(s);
      })
      .catch((err) => {
        if (!cancelled) setError(errorMessage(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Already signed in, or nothing to sign in to yet: skip the form.
  useEffect(() => {
    if (!status) return;
    if (!status.configured) navigate("/welcome", { replace: true });
    else if (status.authenticated) navigate(takeReturnPath(), { replace: true });
  }, [status, navigate]);

  const handleSubmit = useCallback(
    async (e?: React.FormEvent) => {
      e?.preventDefault();
      if (!status || submitting) return;

      setSubmitting(true);
      setError(null);
      try {
        await login(password);
        queryClient.setQueryData<AuthStatus>(["authStatus"], {
          configured: true,
          authenticated: true,
          mode: "password",
        });
        navigate(takeReturnPath(), { replace: true });
      } catch (err) {
        setError(errorMessage(err));
        setPassword("");
      } finally {
        setSubmitting(false);
      }
    },
    [status, submitting, password, queryClient, navigate],
  );

  if (loading) {
    return (
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          gap: "var(--spacing-3)",
          minHeight: "100dvh",
        }}
      >
        <Spinner size="lg" />
        <Text color="secondary">Checking authentication…</Text>
      </div>
    );
  }

  // Sign-in is handled by a proxy, but this browser reached Mizu without
  // going through it (or from an address that isn't a trusted proxy).
  if (status?.mode === "external" && !status.authenticated) {
    return (
      <div className="onboarding-screen" style={{ padding: "var(--spacing-5)" }}>
        <Card style={{ width: "100%", maxWidth: 400 }}>
          <VStack gap={3} align="center" style={{ padding: "var(--spacing-5)" }}>
            <Heading level={1}>Sign in through your proxy</Heading>
            <Text size="sm" color="secondary" style={{ textAlign: "center" }}>
              This Mizu sits behind a sign-in proxy. Open it at your proxy's address.
            </Text>
          </VStack>
        </Card>
      </div>
    );
  }

  return (
    <div className="onboarding-screen" style={{ padding: "var(--spacing-5)" }}>
      <Card style={{ width: "100%", maxWidth: 400 }}>
        <VStack gap={5} as="form" onSubmit={handleSubmit} style={{ padding: "var(--spacing-5)" }}>
          <VStack gap={2} align="center">
            <div
              aria-hidden="true"
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                width: 56,
                height: 56,
                borderRadius: "50%",
                background: "color-mix(in srgb, var(--color-text-blue) 16%, transparent)",
                color: "var(--color-text-blue)",
                marginBottom: "var(--spacing-1)",
              }}
            >
              <Waves size={28} />
            </div>
            <Heading level={1}>Sign in to Mizu</Heading>
            <Text size="sm" color="secondary" style={{ textAlign: "center" }}>
              Enter your password.
            </Text>
          </VStack>

          <VStack gap={3}>
            <TextInput
              type="password"
              label="Password"
              placeholder="Your password"
              value={password}
              onChange={setPassword}
              hasAutoFocus
              isDisabled={submitting}
            />
          </VStack>

          {error && (
            <Text size="sm" role="alert" style={{ color: "var(--color-danger)" }}>
              {error}
            </Text>
          )}

          <Button
            type="submit"
            variant="primary"
            label="Sign in"
            isDisabled={submitting || !password}
            isLoading={submitting}
            icon={<LogIn size={16} />}
          >
            Sign in
          </Button>
        </VStack>
      </Card>
    </div>
  );
}
