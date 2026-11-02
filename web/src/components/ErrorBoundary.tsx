import { Component, type ErrorInfo, type ReactNode } from "react";
import { useInRouterContext, useLocation, useNavigate } from "react-router-dom";
import { Section } from "@astryxdesign/core/Section";
import { VStack } from "@astryxdesign/core/VStack";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { Button } from "@astryxdesign/core/Button";
import { Icon } from "@astryxdesign/core/Icon";
import { Collapsible } from "@astryxdesign/core/Collapsible";
import { Text } from "@astryxdesign/core/Text";
import { CircleAlert, House, RotateCw } from "lucide-react";

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  error: Error | null;
}

/**
 * Route-level error boundary. Catches render errors from a lazy route chunk
 * (or anything below it) and shows the error as the page content, with the
 * app's navigation still in place. main.tsx keys it by pathname, so moving
 * to another page clears the error.
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: unknown): ErrorBoundaryState {
    return { error: error instanceof Error ? error : new Error(String(error)) };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error("[ErrorBoundary] render error:", error, info.componentStack);
  }

  render() {
    if (this.state.error) return <ErrorPage error={this.state.error} />;
    return this.props.children;
  }
}

function ErrorPage({ error }: { error: Error }) {
  // The boundary also wraps content outside the router in tests; only offer
  // in-app navigation when a router is present.
  const inRouter = useInRouterContext();
  return (
    <Section variant="transparent" padding={4} maxWidth={900} className="section-centered">
      <VStack gap={4} align="center" role="alert">
        <EmptyState
          headingLevel={1}
          title="This page ran into a problem"
          description="Mizu couldn't show this page. Your shows and settings are safe, and reloading usually fixes it."
          icon={<Icon icon={CircleAlert} size="lg" color="error" />}
          actions={
            <>
              <Button
                label="Reload page"
                variant="primary"
                icon={<RotateCw size={16} />}
                onClick={() => window.location.reload()}
              />
              {inRouter && <GoToLibraryButton />}
            </>
          }
        />
        {error.message && (
          <Collapsible
            trigger={<Text type="supporting">Technical details</Text>}
            defaultIsOpen={false}
            style={{ width: "100%", maxWidth: "32rem" }}
          >
            <Text type="code" color="secondary">
              {error.message}
            </Text>
          </Collapsible>
        )}
      </VStack>
    </Section>
  );
}

/** Hidden on the library itself, where it would do nothing. */
function GoToLibraryButton() {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  if (pathname === "/" || pathname === "/shows") return null;
  return (
    <Button
      label="Go to library"
      variant="secondary"
      icon={<House size={16} />}
      onClick={() => navigate("/")}
    />
  );
}
