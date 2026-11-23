import { LegacyApp } from "./LegacyApp";
import { OperationsApp } from "./OperationsApp";
import { api } from "./services/api";
import type { Status } from "./types/operations";
import { usePolling } from "./hooks/usePolling";
import {
  Button,
  ErrorBanner,
  ErrorBoundary,
  Panel,
  Skeleton,
  ToastProvider,
} from "./components/ui";
export function App() {
  const status = usePolling(
    (signal) => api<Status>("/api/v1/status", {}, signal),
    "version",
    5,
  );
  if (status.data && (status.data.phase ?? 1) < 3) return <LegacyApp />;
  if (status.error && !status.data)
    return (
      <div className="op-bootstrap">
        <Panel title="Unreachable">
          <ErrorBanner>
            The management service could not be reached. {status.error}
          </ErrorBanner>
          <div className="op-prose">
            <p>
              No service measurements are available. Start the daemon or retry
              the connection.
            </p>
            <Button onClick={status.refresh}>Retry connection</Button>
          </div>
        </Panel>
      </div>
    );
  if (!status.data)
    return (
      <div className="op-bootstrap">
        <Skeleton />
      </div>
    );
  return (
    <ErrorBoundary>
      <ToastProvider>
        <OperationsApp status={status.data} />
      </ToastProvider>
    </ErrorBoundary>
  );
}
