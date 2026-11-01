import { useEffect, useState, useCallback } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { VStack } from "@astryxdesign/core/VStack";
import { Card } from "@astryxdesign/core/Card";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Spinner } from "@astryxdesign/core/Spinner";
import { Waves, LogIn, ShieldCheck } from "lucide-react";
import { getAuthStatus, login, setupPassword, errorMessage } from "../lib/api";
import type { AuthStatus } from "../lib/api";

const MIN_PASSWORD_LENGTH = 8;

export function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const queryClient = useQueryClient();

  const [status, setStatus] = useState<AuthStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const from = searchParams.get("from") || "/";

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

  // Already signed in — skip the form entirely.
  useEffect(() => {
    if (status?.authenticated) navigate(from, { replace: true });
  }, [status, from, navigate]);

  const handleSubmit = useCallback(
    async (e?: React.FormEvent) => {
      e?.preventDefault();
      if (!status || submitting) return;

      if (!status.configured) {
        if (password.length < MIN_PASSWORD_LENGTH) {
          setError(`Password must be at least ${MIN_PASSWORD_LENGTH} characters`);
          return;
        }
        if (password !== confirmPassword) {
          setError("Passwords do not match");
          return;
        }
      }

      setSubmitting(true);
      setError(null);
      try {
        if (status.configured) {
          await login(password);
        } else {
          await setupPassword(password);
        }
        queryClient.setQueryData(["authStatus"], {
          configured: true,
          authenticated: true,
          mode: "builtin",
        });
        navigate(from, { replace: true });
      } catch (err) {
        setError(errorMessage(err));
        setPassword("");
        setConfirmPassword("");
      } finally {
        setSubmitting(false);
      }
    },
    [status, submitting, password, confirmPassword, queryClient, navigate, from],
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

  // MIZU_AUTH=proxy and not authenticated: there is no password to type.
  // The reverse proxy should have signed the visitor in before reaching us.
  if (status?.mode === "proxy") {
    return (
      <div className="onboarding-screen" style={{ padding: "var(--spacing-5)" }}>
        <Card style={{ width: "100%", maxWidth: 400 }}>
          <VStack gap={4} align="center">
            <Waves size={24} aria-hidden="true" />
            <Heading level={1}>Sign-in handled by your proxy</Heading>
            <Text size="sm" color="secondary" style={{ textAlign: "center" }}>
              Mizu didn't receive a signed-in user from your reverse proxy. Open it through the
              proxy's address, or check that the proxy forwards the identity header and is listed in
              MIZU_TRUSTED_PROXIES.
            </Text>
            <Button variant="primary" label="Try again" onClick={() => window.location.reload()} />
          </VStack>
        </Card>
      </div>
    );
  }

  const isSetup = status !== null && !status.configured;

  return (
    <div className="onboarding-screen" style={{ padding: "var(--spacing-5)" }}>
      <Card style={{ width: "100%", maxWidth: 400 }}>
        <VStack gap={5} as="form" onSubmit={handleSubmit}>
          <VStack gap={2} align="center">
            <div
              aria-hidden="true"
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                width: 48,
                height: 48,
                borderRadius: "var(--radius-container)",
                background: "var(--color-surface-raised)",
                color: "var(--color-icon-primary)",
              }}
            >
              <Waves size={24} />
            </div>
            <Heading level={1}>{isSetup ? "Set up Mizu" : "Sign in to Mizu"}</Heading>
            <Text size="sm" color="secondary" style={{ textAlign: "center" }}>
              {isSetup
                ? "Create a password to protect your dashboard. You'll sign in with it from now on."
                : "Enter your dashboard password to continue."}
            </Text>
          </VStack>

          <VStack gap={3}>
            <TextInput
              type="password"
              label="Password"
              placeholder={isSetup ? "Choose a password" : "Your password"}
              value={password}
              onChange={setPassword}
              hasAutoFocus
              isDisabled={submitting}
            />
            {isSetup && (
              <TextInput
                type="password"
                label="Confirm password"
                placeholder="Repeat your password"
                value={confirmPassword}
                onChange={setConfirmPassword}
                isDisabled={submitting}
              />
            )}
          </VStack>

          {error && (
            <Text size="sm" role="alert" style={{ color: "var(--color-danger)" }}>
              {error}
            </Text>
          )}

          <Button
            type="submit"
            variant="primary"
            label={isSetup ? "Create password" : "Sign in"}
            isDisabled={submitting || !password}
            isLoading={submitting}
            icon={isSetup ? <ShieldCheck size={16} /> : <LogIn size={16} />}
          >
            {isSetup ? "Create password" : "Sign in"}
          </Button>

          {isSetup && (
            <Text size="xsm" color="secondary" style={{ textAlign: "center" }}>
              Until a password is set, anyone with network access can change your settings.
            </Text>
          )}
        </VStack>
      </Card>
    </div>
  );
}
