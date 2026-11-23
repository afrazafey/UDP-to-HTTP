import type { OperationsData } from "../types/operations";
import { bytes, duration, rate, timestamp } from "../utils/format";
import { Badge, Definition, Empty, Panel, Stat } from "../components/ui";
import { ThroughputChart } from "../components/dashboard/ThroughputChart";
export function OverviewPage({ data }: { data: OperationsData }) {
  const { dashboard, system } = data;
  const sum = dashboard.summary;
  const errors = data.streams
    .flatMap((stream) =>
      stream.errors.map((error) => ({
        ...error,
        id: stream.channel.id,
        name: stream.channel.name,
      })),
    )
    .sort((a, b) => b.time.localeCompare(a.time))
    .slice(0, 8);
  return (
    <>
      <div className="op-stats-grid">
        <Stat label="Recovering channels" value={sum?.recovering} />
        <Stat label="Offline channels" value={sum?.offline} />
        <Stat label="Receiver restarts" value={sum?.receiver_restarts} />
        <Stat label="Configured channels" value={sum?.total} />
        <Stat label="Running channels" value={sum?.running} />
        <Stat label="Stopped / disabled" value={sum?.stopped} />
        <Stat
          label="Unhealthy channels"
          value={sum?.unhealthy}
          detail={sum ? `${sum.inactive} waiting for valid packets` : undefined}
        />
        <Stat label="HTTP clients" value={sum?.clients} />
        <Stat label="UDP input" value={sum ? rate(sum.input_bitrate) : "—"} />
        <Stat
          label="HTTP output"
          value={sum ? rate(sum.output_bitrate) : "—"}
        />
        <Stat
          label="Server uptime"
          value={duration(dashboard.uptime_seconds)}
          detail={dashboard.version}
        />
      </div>
      <Panel
        title="Live network throughput"
        description="Measured payload rates • bounded process history • no samples before service start"
      >
        <ThroughputChart samples={sum ? dashboard.history : []} />
      </Panel>
      <Panel
        title="Stream health"
        action={
          <a href="#/streams" className="op-link">
            All channels
          </a>
        }
        description="First eight channels; complete searchable health table is available in Streams."
      >
        <div className="op-table-wrap">
          <table>
            <thead>
              <tr>
                <th>Channel / source</th>
                <th>Health</th>
                <th>Input / output</th>
                <th>Viewers</th>
                <th>Uptime</th>
                <th>Last valid TS</th>
              </tr>
            </thead>
            <tbody>
              {data.streams.slice(0, 8).map((s) => (
                <tr key={s.channel.id}>
                  <td>
                    <a href={`#/streams/${s.channel.id}`}>{s.channel.name}</a>
                    <small>
                      {s.channel.source_address}:{s.channel.source_port}
                    </small>
                  </td>
                  <td>
                    <Badge state={s.health ?? s.state} />
                  </td>
                  <td>
                    {rate(s.input_bitrate)} / {rate(s.output_bitrate)}
                  </td>
                  <td>{s.clients}</td>
                  <td>{duration(s.uptime_seconds)}</td>
                  <td>{timestamp(s.last_packet_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>
      <div className="op-two-column">
        <Panel
          title="Service health"
          description="Current daemon and listener availability"
        >
          <Definition
            items={[
              [
                "Database",
                <Badge state={dashboard.database_ready ? "ready" : "error"} />,
              ],
              ["Management", system.management_policy],
              [
                "Playback listeners",
                `${system.http_listeners.filter((l) => l.kind === "playback" && l.state === "listening").length} listening`,
              ],
              ["Active receivers", sum?.active_receivers ?? "Unavailable"],
              ["Go heap", bytes(dashboard.sample.heap_bytes)],
              ["Linux acceptance", system.linux_acceptance],
            ]}
          />
        </Panel>
        <Panel
          title="Recent stream errors"
          description="Actual receiver events; continuity errors are not exact packet loss"
        >
          {errors.length ? (
            <ul className="op-event-list">
              {errors.map((event, index) => (
                <li key={`${event.time}-${index}`}>
                  <a href={`#/streams/${event.id}`}>{event.name}</a>
                  <time>{timestamp(event.time)}</time>
                  <p>
                    {event.operation}: {event.message}
                  </p>
                </li>
              ))}
            </ul>
          ) : (
            <Empty title="No recent stream errors">
              This reflects the currently retained receiver events.
            </Empty>
          )}
        </Panel>
      </div>
      <Panel
        title="Recent activity"
        action={
          <a href="#/logs" className="op-link">
            View activity logs
          </a>
        }
      >
        {dashboard.activity.length ? (
          <ul className="op-event-list">
            {dashboard.activity.map((event) => (
              <li key={event.id}>
                <Badge state={event.level} />
                <time>{timestamp(event.time)}</time>
                <p>{event.message}</p>
                <small>
                  {event.component}
                  {event.stream_id ? ` • ${event.stream_id}` : ""}
                </small>
              </li>
            ))}
          </ul>
        ) : (
          <Empty title="No retained activity yet">
            Confirmed operations and receiver events will appear here.
          </Empty>
        )}
      </Panel>
    </>
  );
}
