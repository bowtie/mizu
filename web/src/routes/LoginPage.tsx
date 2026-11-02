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
import { getAuthStatus, login, setupOpen, setupPassword, errorMessage } from "../lib/api";
import type { AuthStatus } from "../lib/api";
import { safeRedirectPath } from "../lib/redirect";

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

  const from = safeRedirectPath(searchParams.get("from"));

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

  const handleSkip = useCallback(async () => {
    if (submitting) return;
    setSubmitting(true);
    setError(null);
    try {
      await setupOpen();
      queryClient.setQueryData(["authStatus"], {
        configured: true,
        authenticated: true,
        open: true,
      });
      navigate(from, { replace: true });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  }, [submitting, queryClient, navigate, from]);

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

  const isSetup = status !== null && !status.configured;

  // Sign-in is handled by a proxy, but this browser reached Mizu without
  // going through it (or from an address that isn't a trusted proxy).
  if (status?.external && !status.authenticated) {
    return (
      <div className="onboarding-screen" style={{ padding: "var(--spacing-5)" }}>
        <Card style={{ width: "100%", maxWidth: 400 }}>
          <VStack gap={3} align="center" style={{ padding: "var(--spacing-5)" }}>
            <Heading level={1}>Sign in through your proxy</Heading>
            <Text size="sm" color="secondary" style={{ textAlign: "center" }}>
              This Mizu is protected by a sign-in proxy and only answers requests that come through
              it. Open Mizu at your proxy's address instead.
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
            <VStack gap={1} align="center">
              <Button
                variant="ghost"
                label="Continue without a password"
                isDisabled={submitting}
                onClick={() => void handleSkip()}
              />
              <Text size="xsm" color="secondary" style={{ textAlign: "center" }}>
                Only if something else already protects Mizu, like a VPN or a private network.
                Anyone who can reach it can then use it. You can add a password later in Settings.
              </Text>
            </VStack>
          )}
        </VStack>
      </Card>
    </div>
  );
}
