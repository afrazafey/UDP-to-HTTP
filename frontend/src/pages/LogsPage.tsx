import { useState } from "react";
import { Download, RefreshCw } from "lucide-react";
import type { LogEntry, OperationsData } from "../types/operations";
import { api, download } from "../services/api";
import { usePolling } from "../hooks/usePolling";
import { timestamp } from "../utils/format";
import {
  Badge,
  Button,
  CopyButton,
  DataTable,
  Empty,
  ErrorBanner,
  Field,
  Panel,
  Skeleton,
  useAction,
} from "../components/ui";
export function LogsPage({
  data,
  paused,
  interval,
  initialStream = "",
  initialComponent = "",
}: {
  data: OperationsData;
  paused: boolean;
  interval: number;
  initialStream?: string;
  initialComponent?: string;
}) {
  const [search, setSearch] = useState("");
  const [level, setLevel] = useState("");
  const [component, setComponent] = useState(initialComponent);
  const [stream, setStream] = useState(initialStream);
  const [period, setPeriod] = useState("");
  const [offset, setOffset] = useState(0);
  const action = useAction();
  const query = new URLSearchParams({
    search,
    level,
    component,
    stream_id: stream,
    limit: "50",
    offset: String(offset),
  });
  if (period)
    query.set(
      "since",
      new Date(Date.now() - Number(period) * 60000).toISOString(),
    );
  const key = `${search}/${level}/${component}/${stream}/${period}/${offset}`;
  const poll = usePolling(
    (signal) =>
      api<{ logs: LogEntry[]; total: number }>(
        `/api/v1/logs?${query}`,
        {},
        signal,
      ),
    key,
    interval,
    paused,
  );
  return (
    <>
      <Panel
        title="Event history"
        description={poll.data ? `${poll.data.total} matching events · exports include up to 1,000 entries` : undefined}
        action={
          <div className="op-page-actions">
            <Button onClick={poll.refresh}><RefreshCw size={15} /> Refresh logs</Button>
            <Button
              className="primary"
              disabled={action.busy}
              onClick={() => void action.run(
                () => download(`/api/v1/logs/export?${query}`),
                "Diagnostic log download prepared",
              )}
            >
              <Download size={15} /> Download filtered logs
            </Button>
          </div>
        }
      >
        <div className="op-toolbar op-filter-bar">
          <Field label="Search log messages">
            <input
              maxLength={128}
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setOffset(0);
              }}
              placeholder="Message or context"
            />
          </Field>
          <Field label="Severity">
            <select
              value={level}
              onChange={(e) => {
                setLevel(e.target.value);
                setOffset(0);
              }}
            >
              <option value="">All severities</option>
              {["debug", "info", "warn", "error"].map((l) => (
                <option key={l}>{l}</option>
              ))}
            </select>
          </Field>
          <Field label="Stream">
            <select
              value={stream}
              onChange={(e) => {
                setStream(e.target.value);
                setOffset(0);
              }}
            >
              <option value="">All streams</option>
              {data.streams.map((s) => (
                <option key={s.channel.id} value={s.channel.id}>
                  {s.channel.name}
                </option>
              ))}
            </select>
          </Field>
          <Field
            label="Component"
            help="Exact component name, such as receiver or playback."
          >
            <input
              maxLength={64}
              value={component}
              onChange={(e) => {
                setComponent(e.target.value);
                setOffset(0);
              }}
              placeholder="All components"
            />
          </Field>
          <Field label="Time range">
            <select
              value={period}
              onChange={(e) => {
                setPeriod(e.target.value);
                setOffset(0);
              }}
            >
              <option value="">All retained history</option>
              <option value="15">Last 15 minutes</option>
              <option value="60">Last hour</option>
              <option value="1440">Last 24 hours</option>
            </select>
          </Field>
        </div>
        {(poll.error || action.error) && (
          <ErrorBanner>
            {poll.error
              ? `${poll.error}. Displayed log data may be stale.`
              : action.error}
          </ErrorBanner>
        )}
        {poll.loading && !poll.data ? (
          <Skeleton />
        ) : (
          <>
            {poll.data?.logs.length ? (
              <DataTable>
                <thead role="rowgroup">
                  <tr role="row">
                    <th role="columnheader" scope="col">Timestamp</th>
                    <th role="columnheader" scope="col">Severity</th>
                    <th role="columnheader" scope="col">Component / stream</th>
                    <th role="columnheader" scope="col">Message & context</th>
                    <th role="columnheader" scope="col">Actions</th>
                  </tr>
                </thead>
                <tbody role="rowgroup">
                  {poll.data.logs.map((e) => (
                    <tr role="row" key={e.id}>
                      <td role="cell" data-label="Timestamp">{timestamp(e.time)}</td>
                      <td role="cell" data-label="Severity">
                        <Badge state={e.level} />
                      </td>
                      <td role="cell" data-label="Component / stream">
                        {e.component}
                        <small>{e.stream_id || "Service"}</small>
                      </td>
                      <td role="cell" data-label="Message &amp; context" data-primary className="op-log-message">
                        {e.message}
                        <details>
                          <summary>Structured context</summary>
                          <pre>{JSON.stringify(e.context, null, 2)}</pre>
                        </details>
                      </td>
                      <td role="cell" data-label="Actions">
                        <CopyButton
                          label="Copy entry"
                          value={JSON.stringify(e)}
                        />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            ) : (
              <Empty title="No matching log entries">
                Adjust the filters or wait for confirmed operations and receiver
                events.
              </Empty>
            )}
            {(offset > 0 || (poll.data?.total ?? 0) > 50) && <div className="op-pagination">
              <Button
                disabled={offset === 0}
                onClick={() => setOffset(Math.max(0, offset - 50))}
              >
                Previous
              </Button>
              <span>Page {Math.floor(offset / 50) + 1}</span>
              <Button
                disabled={!poll.data || offset + 50 >= poll.data.total}
                onClick={() => setOffset(offset + 50)}
              >
                Next
              </Button>
            </div>}
          </>
        )}
      </Panel>
    </>
  );
}
