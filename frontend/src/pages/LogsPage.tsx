import { useState } from "react";
import type { LogEntry, OperationsData } from "../types/operations";
import { api, download } from "../services/api";
import { usePolling } from "../hooks/usePolling";
import { timestamp } from "../utils/format";
import {
  Badge,
  Button,
  CopyButton,
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
}: {
  data: OperationsData;
  paused: boolean;
  interval: number;
  initialStream?: string;
}) {
  const [search, setSearch] = useState("");
  const [level, setLevel] = useState("");
  const [stream, setStream] = useState(initialStream);
  const [period, setPeriod] = useState("");
  const [offset, setOffset] = useState(0);
  const action = useAction();
  const query = new URLSearchParams({
    search,
    level,
    stream_id: stream,
    limit: "50",
    offset: String(offset),
  });
  if (period)
    query.set(
      "since",
      new Date(Date.now() - Number(period) * 60000).toISOString(),
    );
  const key = `${search}/${level}/${stream}/${period}/${offset}`;
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
      <div className="op-toolbar">
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
        <Button onClick={poll.refresh}>Refresh logs</Button>
        <Button
          disabled={action.busy}
          onClick={() => {
            void action.run(
              () => download(`/api/v1/logs/export?${query}`),
              "Diagnostic log download prepared",
            );
          }}
        >
          Download filtered logs
        </Button>
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
        <Panel
          title="Activity logs"
          description={`Sanitized operational events • ${poll.data?.total ?? "unknown"} matches • export capped at 1,000 entries`}
        >
          {poll.data?.logs.length ? (
            <div className="op-table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Timestamp</th>
                    <th>Severity</th>
                    <th>Component / stream</th>
                    <th>Message & context</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {poll.data.logs.map((e) => (
                    <tr key={e.id}>
                      <td>{timestamp(e.time)}</td>
                      <td>
                        <Badge state={e.level} />
                      </td>
                      <td>
                        {e.component}
                        <small>{e.stream_id || "Service"}</small>
                      </td>
                      <td className="op-log-message">
                        {e.message}
                        <details>
                          <summary>Structured context</summary>
                          <pre>{JSON.stringify(e.context, null, 2)}</pre>
                        </details>
                      </td>
                      <td>
                        <CopyButton
                          label="Copy entry"
                          value={JSON.stringify(e)}
                        />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <Empty title="No matching log entries">
              Adjust the filters or wait for confirmed operations and receiver
              events.
            </Empty>
          )}
          <div className="op-pagination">
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
          </div>
        </Panel>
      )}
    </>
  );
}
