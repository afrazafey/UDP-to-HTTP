import { useState, type ReactNode } from "react";
import {
  Activity,
  ArrowDownToLine,
  ArrowRight,
  ArrowUpFromLine,
  Check,
  CircleAlert,
  CirclePlay,
  Database,
  Info,
  Minus,
  Monitor,
  Network,
  Radio,
  Server,
  Users,
} from "lucide-react";
import type { NetworkInterface, OperationsData, Stream } from "../types/operations";
import { duration, rate, timestamp } from "../utils/format";
import { Badge, Empty, ErrorBanner, Panel, useAction } from "../components/ui";
import { ThroughputChart } from "../components/dashboard/ThroughputChart";
import { ChannelIdentity, ChannelMenu } from "../components/streams/ChannelMenu";
import { ChannelDialog } from "../components/streams/ChannelDialog";

function Metric({
  label,
  value,
  icon,
  children,
  featured = false,
}: {
  label: string;
  value: ReactNode;
  icon: ReactNode;
  children?: ReactNode;
  featured?: boolean;
}) {
  return (
    <div className={`op-overview-metric${featured ? " featured" : ""}`}>
      <span className="op-metric-icon" aria-hidden="true">{icon}</span>
      <div className="op-metric-value">
        <span>{label}</span>
        <strong>{value}</strong>
      </div>
      <div className="op-metric-detail">{children}</div>
    </div>
  );
}

function Sparkline({ values }: { values: number[] }) {
  if (values.length < 2) return <span>Collecting samples</span>;
  const min = Math.min(...values);
  const span = Math.max(Math.max(...values) - min, 1);
  const points = values.map((value, index) =>
    `${index * 180 / (values.length - 1)},${19 - (value - min) * 14 / span}`
  ).join(" ");
  return (
    <svg className="op-sparkline" viewBox="0 0 180 24" preserveAspectRatio="none" aria-hidden="true">
      <polyline points={points} />
    </svg>
  );
}

function interfaceStatus(selection: string, interfaces: NetworkInterface[], multicast = false) {
  if (!selection || selection === "0.0.0.0") {
    return { name: "", state: "Automatic", tone: "neutral" };
  }
  const match = interfaces.find(item => item.name === selection || item.ipv4.includes(selection));
  if (!match) return { name: selection, state: "Unknown", tone: "neutral" };
  const ready = match.up && (!multicast || match.multicast);
  return {
    name: match.name,
    state: ready ? "Up" : match.up ? "No multicast" : "Down",
    tone: ready ? "good" : "warning",
  };
}

function ServiceHealth({ data, serviceState }: { data: OperationsData; serviceState: string }) {
  const { dashboard, system, streams } = data;
  const { network, streaming } = data.settings.active;
  const listeners = system.http_listeners.filter(listener => listener.kind === "playback");
  const listening = listeners.filter(listener => listener.state === "listening").length;
  const listenerFailed = listeners.some(listener => listener.state === "failed");
  const active = dashboard.summary?.active_receivers;
  const receiverIssue = streams.some(stream =>
    stream.channel.enabled && [
      "degraded", "recovering", "unhealthy", "offline",
    ].includes(stream.health ?? stream.state)
  );
  const multicast = interfaceStatus(network.multicast_interface, system.interfaces, true);
  const binds = network.http_bind_addresses?.length
    ? network.http_bind_addresses
    : [network.http_interface];
  const playback = binds.map(bind => interfaceStatus(bind, system.interfaces));
  const playbackStatus = playback.find(item => item.tone === "warning")
    ?? playback.find(item => item.tone === "neutral")
    ?? playback[0];
  const rows = [
    {
      name: "UDP Receiver", icon: Radio,
      state: receiverIssue ? "Stream issue" : active == null ? "Unknown" : active > 0 ? `${active} active` : "Idle",
      tone: receiverIssue ? "warning" : active ? "good" : "neutral",
    },
    {
      name: "HTTP Server", icon: Server,
      state: listenerFailed ? "Bind failed" : listening ? "Listening" : streaming.enabled ? "Unavailable" : "Disabled",
      tone: listenerFailed || (streaming.enabled && !listening) ? "warning" : listening ? "good" : "neutral",
    },
    {
      ...multicast, icon: Network,
      name: multicast.name ? `Multicast (${multicast.name})` : "Multicast Interface",
    },
    {
      ...playbackStatus, icon: Monitor,
      name: playback.length > 1 ? `HTTP Interfaces (${playback.length})`
        : playbackStatus.name ? `HTTP Interface (${playbackStatus.name})` : "HTTP Interface",
    },
    {
      name: "Database (SQLite)", icon: Database,
      state: dashboard.database_ready ? "Ready" : "Unavailable",
      tone: dashboard.database_ready ? "good" : "warning",
    },
  ];
  const stale = serviceState === "stale";
  const warning = serviceState !== "running" || rows.some(row => row.tone === "warning");
  const HealthIcon = warning ? CircleAlert : Check;
  return (
    <Panel title="Service Health" className="op-overview-health">
      <div className="op-health-body">
        <div className={`op-health-indicator ${warning ? "warning" : "good"}`}>
          <span><HealthIcon size={36} aria-hidden="true" /></span>
          <strong>{stale ? "Stale" : warning ? "Degraded" : "Healthy"}</strong>
        </div>
        <ul className="op-health-rows">
          {rows.map(({ name, icon: Icon, state, tone }) => {
            const StateIcon = tone === "good" ? Check : tone === "warning" ? CircleAlert : Minus;
            return (
              <li key={name}>
                <Icon size={15} className={`op-health-icon ${tone}`} aria-hidden="true" />
                <span>
                  {name}
                  {tone !== "good" && <small>{state}</small>}
                </span>
                <span className={`op-health-mark ${tone}`} title={state}>
                  <StateIcon size={12} aria-hidden="true" />
                  {tone === "good" && <span className="sr-only">{state}</span>}
                </span>
              </li>
            );
          })}
        </ul>
      </div>
    </Panel>
  );
}

export function OverviewPage({
  data,
  refresh,
  serviceState,
}: {
  data: OperationsData;
  refresh: () => void;
  serviceState: string;
}) {
  const [editing, setEditing] = useState<Stream>();
  const action = useAction();
  const { dashboard, streams } = data;
  const sum = dashboard.summary;
  const recent = sum ? dashboard.history.slice(-24) : [];

  return (
    <>
      {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
      <div className="op-overview">
        <div className="op-overview-metrics">
          <Metric label="Total Channels" value={sum?.total ?? "Unavailable"} icon={<CirclePlay size={21} />} featured>
            {sum ? <><span className={`op-dot${sum.running ? "" : " idle"}`} aria-hidden="true" /> {sum.running} Running</> : "Configured channels"}
          </Metric>
          <Metric label="HTTP Clients" value={sum?.clients ?? "Unavailable"} icon={<Users size={21} />}>
            Active viewers
          </Metric>
          <Metric label="UDP Input" value={rate(sum?.input_bitrate)} icon={<ArrowDownToLine size={21} />}>
            <Sparkline values={recent.map(sample => sample.input_bitrate)} />
          </Metric>
          <Metric label="HTTP Output" value={rate(sum?.output_bitrate)} icon={<ArrowUpFromLine size={21} />}>
            <Sparkline values={recent.map(sample => sample.output_bitrate)} />
          </Metric>
          <Metric label="Uptime" value={duration(dashboard.uptime_seconds)} icon={<Server size={21} />}>
            {dashboard.version}
          </Metric>
        </div>
        <div className="op-overview-columns">
          <div className="op-overview-primary">
            <Panel className="op-overview-throughput">
              <ThroughputChart
                samples={sum ? dashboard.history : []}
                heading={{ title: "Live Network Throughput", description: "Real-time UDP input and HTTP output bitrate" }}
              />
            </Panel>
            <Panel
              title="Channel Status"
              className="op-overview-channels"
              action={<a href="#/streams" className="op-link">View All <ArrowRight size={14} aria-hidden="true" /></a>}
            >
              {streams.length ? (
                <div className="op-table-wrap">
                  <table className="op-stream-table">
                    <thead>
                      <tr>
                        <th>#</th>
                        <th>Channel</th>
                        <th>Source</th>
                        <th>Bitrate</th>
                        <th>Viewers</th>
                        <th>Status</th>
                        <th><span className="sr-only">Actions</span></th>
                      </tr>
                    </thead>
                    <tbody>
                      {streams.slice(0, 8).map((stream, index) => (
                        <tr key={stream.channel.id}>
                          <td data-label="#">{index + 1}</td>
                          <td data-label="Channel"><ChannelIdentity stream={stream} /></td>
                          <td data-label="Source">
                            <span className="op-overview-source">
                              <Radio size={14} aria-hidden="true" />
                              {stream.channel.source_address}:{stream.channel.source_port}
                            </span>
                          </td>
                          <td data-label="Bitrate">{rate(stream.input_bitrate)}</td>
                          <td data-label="Viewers">{stream.clients}</td>
                          <td data-label="Status">
                            <Badge state={stream.health === "healthy" ? stream.state : stream.health ?? stream.state} />
                          </td>
                          <td data-label="Actions">
                            <ChannelMenu stream={stream} data={data} edit={() => setEditing(stream)}
                              run={action.run} busy={action.busy} refresh={refresh} />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <Empty title="No channels configured">Add a channel to begin monitoring reception.</Empty>
              )}
            </Panel>
          </div>
          <div className="op-overview-secondary">
            <ServiceHealth data={data} serviceState={serviceState} />
            <Panel
              title="Recent Activity"
              className="op-overview-activity"
              action={<a href="#/logs" className="op-link">View Logs <ArrowRight size={14} aria-hidden="true" /></a>}
            >
              {dashboard.activity.length ? (
                <ul className="op-activity-list">
                  {dashboard.activity.slice(0, 4).map((event) => {
                    const warning = ["error", "warn", "warning"].includes(event.level);
                    const Icon = warning ? CircleAlert
                      : event.component === "receiver" ? Radio
                      : ["client_connect", "client_disconnect"].includes(String(event.context?.event_type)) ? Users
                      : event.component === "stream" ? Activity
                      : Info;
                    const tone = warning ? "warning" : event.component === "receiver" ? "good" : "info";
                    return (
                      <li key={event.id}>
                        <span className={`op-activity-icon ${tone}`}><Icon size={16} aria-hidden="true" /></span>
                        <span>{event.message}</span>
                        <time dateTime={event.time} title={timestamp(event.time)}>
                          {new Date(event.time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false })}
                        </time>
                      </li>
                    );
                  })}
                </ul>
              ) : <Empty title="No retained activity" />}
            </Panel>
          </div>
        </div>
      </div>
      {editing && (
        <ChannelDialog data={data} stream={editing} onClose={() => setEditing(undefined)} refresh={refresh} />
      )}
    </>
  );
}
