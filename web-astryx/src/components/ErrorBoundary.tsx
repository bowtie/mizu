import { Component, type ErrorInfo, type ReactNode } from "react";
import { CircleAlert } from "lucide-react";

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
}

/**
 * Top-level error boundary. Catches render errors from any lazy-loaded
 * route chunk (or a component below it) so a crash shows a recoverable
 * fallback instead of a blank screen. "Try again" resets the boundary
 * and re-renders children; "Reload" does a full page reload to also
 * reset app state (theme, query cache, navigation).
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    // Log the full error + component stack for debugging. The user sees
    // the fallback UI; this keeps the details out of the UI.
    console.error("[ErrorBoundary] render error:", error, info.componentStack);
  }

  private reset = () => {
    this.setState({ hasError: false });
  };

  render() {
    if (this.state.hasError) {
      return (
        <div className="error-boundary" role="alert">
          <div className="error-boundary-card">
            <div className="error-boundary-icon">
              <CircleAlert size={24} />
            </div>
            <h2 className="error-boundary-title">Something went wrong</h2>
            <p className="error-boundary-desc">
              An unexpected error occurred while rendering this page. Your data is safe — try
              again, or reload the app to start fresh.
            </p>
            <div className="error-boundary-actions">
              <button type="button" className="error-boundary-btn" onClick={this.reset}>
                Try again
              </button>
              <button
                type="button"
                className="error-boundary-btn error-boundary-btn-primary"
                onClick={() => window.location.reload()}
              >
                Reload app
              </button>
            </div>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
