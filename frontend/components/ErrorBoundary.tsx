import * as preact from "preact";
import { Header, Footer } from "../layout";
import { ErrorDisplay } from "./ErrorDisplay";
import { reportClientError } from "../lib/clientErrors";
import type { DisplayableError } from "../lib/errorDisplay";

interface ErrorBoundaryProps {
  children: preact.ComponentChildren;
}

interface ErrorBoundaryState {
  crashed: boolean;
}

const crashError: DisplayableError = {
  kind: "server",
  title: "This page stopped working",
  message: "Something went wrong while drawing the page. Reloading usually clears it.",
  reference: null,
  retryable: true,
};

// setErrorView covers a route whose fetch failed. This covers a component that
// threw while rendering, which would otherwise leave a blank page and no report.
export class ErrorBoundary extends preact.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { crashed: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { crashed: true };
  }

  componentDidCatch(error: unknown) {
    reportClientError({
      message: error instanceof Error ? error.message : String(error),
      stack: error instanceof Error ? error.stack : undefined,
      source: "render",
    });
  }

  render() {
    if (!this.state.crashed) {
      return this.props.children;
    }

    return (
      <div>
        <Header isHome={false} />
        <main id="app">
          <ErrorDisplay error={crashError} />
        </main>
        <Footer />
      </div>
    );
  }
}
