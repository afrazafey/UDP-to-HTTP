import { useEffect, useState } from "react";
import {
  Copy,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Save,
  Square,
  Trash2,
  X,
} from "lucide-react";
import { request, type Configuration, type NetworkInterface } from "./api";

export interface Channel {
  id: string;
  name: string;
  source_type: "multicast" | "unicast";
  source_address: string;
  source_port: number;
  interface: string;
  enabled: boolean;
  startup: "on_demand" | "always";
  idle_timeout_seconds: number;
  max_clients: number;
  client_buffer_bytes: number;
  udp_receive_buffer: number;
}
export interface StreamSnapshot {
  channel: Channel;
  state: string;
  clients: number;
  uptime_seconds: number;
  last_packet_at: string | null;
  input_bitrate: number;
  output_bitrate: number;
  counters: {
    input_bytes: number;
    output_bytes: number;
    packets: number;
    corrupt_packets: number;
    discarded_bytes: number;
    continuity_errors: number;
    discontinuities: number;
    duplicate_packets: number;
    slow_client_disconnects: number;
    retries: number;
    rejected_datagrams: number;
  };
  errors: { time: string; operation: string; message: string }[];
}
interface Client {
  id: number;
  channel_id: string;
  remote_ip: string;
  connected_at: string;
  queued_bytes: number;
  closing: boolean;
}
const bitrate = (value: number) =>
  value >= 1e6
    ? `${(value / 1e6).toFixed(2)} Mbps`
    : `${(value / 1000).toFixed(1)} Kbps`;
const size = (value: number) =>
  value >= 1048576
    ? `${(value / 1048576).toFixed(2)} MiB`
    : `${(value / 1024).toFixed(1)} KiB`;
const uptime = (value: number) =>
  `${Math.floor(value / 3600)}h ${Math.floor(value / 60) % 60}m ${value % 60}s`;

async function mutate(path: string, method: string, body?: Channel) {
  const signal = AbortSignal.timeout(15000);
  const { token } = await request<{ token: string }>("/api/v1/csrf", signal);
  const response = await fetch(path, {
    method,
    signal,
    headers: { "Content-Type": "application/json", "X-CSRF-Token": token },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    const result = await response.json();
    throw new Error(result.error ?? `Request failed (${response.status})`);
  }
}

export function StreamingWorkspace({
  view,
  configuration,
  interfaces,
}: {
  view: "overview" | "streams" | "clients" | "monitoring";
  configuration: Configuration;
  interfaces: NetworkInterface[];
}) {
  const [streams, setStreams] = useState<StreamSnapshot[] | null>(null);
  const [clients, setClients] = useState<Client[]>([]);
  const [error, setError] = useState("");
  const [actionError, setActionError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [refresh, setRefresh] = useState(0);
  const [form, setForm] = useState<Channel | null>(null);
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  useEffect(() => {
    let active = true;
    let pending = false;
    let controller: AbortController | null = null;
    const load = async () => {
      if (pending) return;
      pending = true;
      controller = new AbortController();
      const current = controller;
      const timeout = setTimeout(() => current.abort(), 8000);
      try {
        const [snapshot, connections] = await Promise.all([
          request<{ streams: StreamSnapshot[] }>(
            "/api/v1/streams",
            current.signal,
          ),
          request<{ clients: Client[] }>("/api/v1/clients", current.signal),
        ]);
        if (active) {
          setStreams(snapshot.streams);
          setClients(connections.clients);
          setError("");
        }
      } catch (failure) {
        if (active)
          setError(
            `Stream data could not be refreshed. ${failure instanceof Error ? failure.message : "Request failed"}. Displayed data may be stale.`,
          );
      } finally {
        clearTimeout(timeout);
        pending = false;
      }
    };
    void load();
    const timer = setInterval(() => {
      void load();
    }, 5000);
    return () => {
      active = false;
      controller?.abort();
      clearInterval(timer);
    };
  }, [refresh]);
  const act = async (path: string, method: string, body?: Channel) => {
    setBusy(true);
    setActionError("");
    setNotice("");
    try {
      await mutate(path, method, body);
      setRefresh((value) => value + 1);
      return true;
    } catch (failure) {
      setActionError(
        failure instanceof Error ? failure.message : "Operation failed",
      );
      return false;
    } finally {
      setBusy(false);
    }
  };
  const newChannel = () => {
    setEditing(false);
    setForm({
      id: "",
      name: "",
      source_type: "multicast",
      source_address: "",
      source_port: 1234,
      interface:
        configuration.network.multicast_interface ||
        interfaces.find((item) => item.up && item.multicast)?.name ||
        "",
      enabled: true,
      startup: "on_demand",
      idle_timeout_seconds: 5,
      max_clients: configuration.streaming.max_clients,
      client_buffer_bytes:
        configuration.streaming.client_buffer_bytes ?? 262144,
      udp_receive_buffer: configuration.streaming.udp_receive_buffer,
    });
  };
  const playbackURL = (channel: Channel) => {
    let host =
      configuration.network.http_bind_addresses?.[0] ||
      configuration.network.http_interface;
    host = interfaces.find((item) => item.name === host)?.ipv4[0] || host;
    if (!host || host === "0.0.0.0") host = window.location.hostname;
    return `http://${host}:${configuration.network.http_port}/live/${channel.id}.ts`;
  };
  const copyURL = async (channel: Channel) => {
    try {
      await navigator.clipboard.writeText(playbackURL(channel));
      setNotice(`Copied playback URL for ${channel.name}`);
    } catch {
      setActionError(
        `Clipboard access is unavailable. Playback URL: ${playbackURL(channel)}`,
      );
    }
  };
  const stats = [
    [
      "Active streams",
      streams?.filter((item) => item.state === "running").length,
    ],
    [
      "Offline / waiting",
      streams?.filter((item) => item.state !== "running").length,
    ],
    [
      "Connected clients",
      streams?.reduce((sum, item) => sum + item.clients, 0),
    ],
    [
      "Input bitrate",
      streams
        ? bitrate(streams.reduce((sum, item) => sum + item.input_bitrate, 0))
        : undefined,
    ],
    [
      "Output bitrate",
      streams
        ? bitrate(streams.reduce((sum, item) => sum + item.output_bitrate, 0))
        : undefined,
    ],
    ["Configured channels", streams?.length],
  ];
  return (
    <div className="stream-workspace">
      {(error || actionError) && (
        <div className="error-banner" role="alert">
          {[error, actionError].filter(Boolean).join(" ")}
        </div>
      )}
      {notice && (
        <div className="operation-notice" role="status">
          {notice}
        </div>
      )}
      {(view === "overview" || view === "monitoring") && (
        <div className="stream-stats">
          {stats.map(([label, value]) => (
            <section className="stat-card" key={label}>
              <div className="stat-top">{label}</div>
              <div className="stat-value mono">{value ?? "—"}</div>
            </section>
          ))}
        </div>
      )}
      {view === "streams" && (
        <div className="stream-toolbar">
          <p>
            Manage approved UDP sources. Playback access uses a separate client
            CIDR allowlist.
          </p>
          <button
            className="button primary-button"
            onClick={newChannel}
            disabled={busy || !!form}
          >
            <Plus size={16} />
            Add channel
          </button>
        </div>
      )}
      {form && (
        <form
          className="panel channel-form"
          onSubmit={async (event) => {
            event.preventDefault();
            if (
              await act(
                editing ? `/api/v1/streams/${form.id}` : "/api/v1/streams",
                editing ? "PUT" : "POST",
                form,
              )
            )
              setForm(null);
          }}
        >
          <div className="panel-heading">
            <h2>{editing ? "Edit channel" : "New channel"}</h2>
            <button
              type="button"
              className="icon-button"
              aria-label="Close channel form"
              onClick={() => setForm(null)}
              disabled={busy}
            >
              <X size={18} />
            </button>
          </div>
          <div className="channel-form-grid">
            <label>
              Channel ID
              <input
                required
                pattern="[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}"
                maxLength={64}
                disabled={editing}
                value={form.id}
                onChange={(event) =>
                  setForm({ ...form, id: event.target.value })
                }
              />
            </label>
            <label>
              Channel name
              <input
                required
                maxLength={128}
                value={form.name}
                onChange={(event) =>
                  setForm({ ...form, name: event.target.value })
                }
              />
            </label>
            <label>
              Source type
              <select
                value={form.source_type}
                onChange={(event) =>
                  setForm({
                    ...form,
                    source_type: event.target.value as Channel["source_type"],
                  })
                }
              >
                <option value="multicast">IPv4 multicast group</option>
                <option value="unicast">IPv4 unicast sender</option>
              </select>
            </label>
            <label>
              {form.source_type === "multicast"
                ? "Multicast group IPv4"
                : "Expected sender IPv4"}
              <input
                required
                value={form.source_address}
                placeholder={
                  form.source_type === "multicast"
                    ? "Approved multicast group"
                    : "Approved sender IP"
                }
                onChange={(event) =>
                  setForm({ ...form, source_address: event.target.value })
                }
              />
            </label>
            <label>
              UDP destination port
              <input
                type="number"
                required
                min={1}
                max={65535}
                value={form.source_port}
                onChange={(event) =>
                  setForm({ ...form, source_port: Number(event.target.value) })
                }
              />
            </label>
            <label>
              Local network interface
              <input
                required
                list="stream-interfaces"
                value={form.interface}
                onChange={(event) =>
                  setForm({ ...form, interface: event.target.value })
                }
              />
              <datalist id="stream-interfaces">
                {interfaces
                  .filter((item) => item.up)
                  .map((item) => (
                    <option key={item.index} value={item.name}>
                      {item.ipv4.join(", ")}
                    </option>
                  ))}
              </datalist>
            </label>
            <label>
              Startup behavior
              <select
                value={form.startup}
                onChange={(event) =>
                  setForm({
                    ...form,
                    startup: event.target.value as Channel["startup"],
                  })
                }
              >
                <option value="on_demand">On demand</option>
                <option value="always">Always receive</option>
              </select>
            </label>
            <label>
              Idle timeout (seconds)
              <input
                type="number"
                required
                min={0}
                max={86400}
                value={form.idle_timeout_seconds}
                onChange={(event) =>
                  setForm({
                    ...form,
                    idle_timeout_seconds: Number(event.target.value),
                  })
                }
              />
            </label>
            <label>
              Maximum clients
              <input
                type="number"
                required
                min={1}
                max={configuration.streaming.max_clients}
                value={form.max_clients}
                onChange={(event) =>
                  setForm({ ...form, max_clients: Number(event.target.value) })
                }
              />
            </label>
            <label>
              Client buffer (bytes)
              <input
                type="number"
                required
                min={564}
                max={8388608}
                value={form.client_buffer_bytes}
                onChange={(event) =>
                  setForm({
                    ...form,
                    client_buffer_bytes: Number(event.target.value),
                  })
                }
              />
            </label>
            <label>
              UDP receive buffer (bytes)
              <input
                type="number"
                required
                min={65536}
                max={67108864}
                value={form.udp_receive_buffer}
                onChange={(event) =>
                  setForm({
                    ...form,
                    udp_receive_buffer: Number(event.target.value),
                  })
                }
              />
            </label>
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(event) =>
                  setForm({ ...form, enabled: event.target.checked })
                }
              />
              Channel enabled
            </label>
          </div>
          <div className="form-policy">
            <p>
              Addresses must be present in the server allowlists. Unicast binds
              locally and accepts datagrams only from the configured sender. An
              idle timeout of zero releases an on-demand receiver after its last
              viewer disconnects. Manual Start pins reception until Stop.
            </p>
          </div>
          <div className="form-actions">
            <button
              type="button"
              className="button"
              onClick={() => setForm(null)}
              disabled={busy}
            >
              Cancel
            </button>
            <button className="button primary-button" disabled={busy}>
              <Save size={16} />
              {busy ? "Saving…" : "Save channel"}
            </button>
          </div>
        </form>
      )}
      {view === "clients" ? (
        <section className="panel">
          <div className="panel-heading">
            <div>
              <h2>Connected playback clients</h2>
              <p>
                Actual subscriptions, including connections awaiting stream
                startup
              </p>
            </div>
          </div>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Connection</th>
                  <th>Channel</th>
                  <th>Client IP</th>
                  <th>Connected</th>
                  <th>Queued data</th>
                  <th>State</th>
                </tr>
              </thead>
              <tbody>
                {clients.map((client) => (
                  <tr key={client.id}>
                    <td className="mono">{client.id}</td>
                    <td>{client.channel_id}</td>
                    <td className="mono">{client.remote_ip}</td>
                    <td>
                      {new Date(client.connected_at).toLocaleTimeString()}
                    </td>
                    <td>{size(client.queued_bytes)}</td>
                    <td>{client.closing ? "Disconnecting" : "Connected"}</td>
                  </tr>
                ))}
                {clients.length === 0 && (
                  <tr>
                    <td colSpan={6}>
                      {streams
                        ? "No connected playback clients."
                        : "Loading client connections…"}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </section>
      ) : (
        <section className="panel">
          <div className="panel-heading">
            <div>
              <h2>{view === "monitoring" ? "Transport health" : "Channels"}</h2>
              <p>
                Real stream state · updates every 5 seconds · bitrate averaged
                between samples
              </p>
            </div>
            <button
              className="text-button"
              onClick={() => setRefresh((value) => value + 1)}
            >
              <RefreshCw size={14} />
              Refresh streams
            </button>
          </div>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Channel</th>
                  <th>State</th>
                  {view === "monitoring" ? (
                    <>
                      <th>Input / output</th>
                      <th>TS packets</th>
                      <th>Corrupt / CC errors</th>
                      <th>Dropped clients</th>
                      <th>Last valid packet</th>
                    </>
                  ) : (
                    <>
                      <th>UDP source</th>
                      <th>Clients</th>
                      <th>Input / output bitrate</th>
                      <th>Uptime</th>
                      <th>Actions</th>
                    </>
                  )}
                </tr>
              </thead>
              <tbody>
                {streams?.map((snapshot) => (
                  <tr key={snapshot.channel.id}>
                    <td>
                      <strong>{snapshot.channel.name}</strong>
                      <small className="channel-id mono">
                        {snapshot.channel.id}
                      </small>
                      {snapshot.errors.length > 0 && (
                        <details className="stream-errors">
                          <summary>
                            {snapshot.errors.length} recent events
                          </summary>
                          <ol>
                            {snapshot.errors
                              .slice()
                              .reverse()
                              .map((event, index) => (
                                <li key={`${event.time}-${index}`}>
                                  <time>
                                    {new Date(event.time).toLocaleTimeString()}
                                  </time>{" "}
                                  · {event.operation}
                                  <p>{event.message}</p>
                                </li>
                              ))}
                          </ol>
                        </details>
                      )}
                    </td>
                    <td>
                      <span className={`stream-state state-${snapshot.state}`}>
                        {snapshot.state}
                      </span>
                      {!snapshot.channel.enabled && (
                        <small className="channel-id">
                          Disabled in configuration
                        </small>
                      )}
                    </td>
                    {view === "monitoring" ? (
                      <>
                        <td>
                          {size(snapshot.counters.input_bytes)} /{" "}
                          {size(snapshot.counters.output_bytes)}
                        </td>
                        <td className="mono">{snapshot.counters.packets}</td>
                        <td>
                          {snapshot.counters.corrupt_packets} /{" "}
                          {snapshot.counters.continuity_errors}
                          <small className="channel-id">
                            {snapshot.counters.discontinuities} discontinuities
                            · {snapshot.counters.retries} retries
                          </small>
                        </td>
                        <td>{snapshot.counters.slow_client_disconnects}</td>
                        <td>
                          {snapshot.last_packet_at
                            ? new Date(
                                snapshot.last_packet_at,
                              ).toLocaleTimeString()
                            : "None received"}
                        </td>
                      </>
                    ) : (
                      <>
                        <td className="mono">
                          {snapshot.channel.source_address}:
                          {snapshot.channel.source_port}
                          <small className="channel-id">
                            {snapshot.channel.source_type} ·{" "}
                            {snapshot.channel.interface}
                          </small>
                        </td>
                        <td>
                          {snapshot.clients} / {snapshot.channel.max_clients}
                        </td>
                        <td className="mono">
                          {bitrate(snapshot.input_bitrate)}
                          <small className="channel-id">
                            {bitrate(snapshot.output_bitrate)}
                          </small>
                        </td>
                        <td className="mono">
                          {uptime(snapshot.uptime_seconds)}
                        </td>
                        <td>
                          <div className="channel-actions">
                            <button
                              className="icon-button"
                              aria-label={`Start ${snapshot.channel.name}`}
                              title="Start and keep receiving"
                              disabled={busy || !snapshot.channel.enabled}
                              onClick={() => {
                                void act(
                                  `/api/v1/streams/${snapshot.channel.id}/start`,
                                  "POST",
                                );
                              }}
                            >
                              <Play size={16} />
                            </button>
                            <button
                              className="icon-button"
                              aria-label={`Stop ${snapshot.channel.name}`}
                              title="Stop and disconnect viewers"
                              disabled={busy}
                              onClick={() => {
                                void act(
                                  `/api/v1/streams/${snapshot.channel.id}/stop`,
                                  "POST",
                                );
                              }}
                            >
                              <Square size={15} />
                            </button>
                            <button
                              className="icon-button"
                              aria-label={`Restart ${snapshot.channel.name}`}
                              title="Restart receiver"
                              disabled={busy || !snapshot.channel.enabled}
                              onClick={() => {
                                void act(
                                  `/api/v1/streams/${snapshot.channel.id}/restart`,
                                  "POST",
                                );
                              }}
                            >
                              <RefreshCw size={15} />
                            </button>
                            <button
                              className="icon-button"
                              aria-label={`Copy URL for ${snapshot.channel.name}`}
                              title="Copy playback URL"
                              onClick={() => {
                                void copyURL(snapshot.channel);
                              }}
                            >
                              <Copy size={16} />
                            </button>
                            {view === "streams" && (
                              <>
                                <button
                                  className="icon-button"
                                  aria-label={`Edit ${snapshot.channel.name}`}
                                  title="Edit channel"
                                  disabled={busy || !!form}
                                  onClick={() => {
                                    setEditing(true);
                                    setForm({ ...snapshot.channel });
                                  }}
                                >
                                  <Pencil size={16} />
                                </button>
                                <button
                                  className="icon-button"
                                  aria-label={`Delete ${snapshot.channel.name}`}
                                  title="Delete channel"
                                  disabled={busy}
                                  onClick={() =>
                                    setDeleting(snapshot.channel.id)
                                  }
                                >
                                  <Trash2 size={16} />
                                </button>
                              </>
                            )}
                          </div>
                          {deleting === snapshot.channel.id && (
                            <div className="delete-confirmation">
                              <span>
                                Delete this channel and disconnect its viewers?
                              </span>
                              <button
                                className="button"
                                disabled={busy}
                                onClick={async () => {
                                  if (
                                    await act(
                                      `/api/v1/streams/${snapshot.channel.id}`,
                                      "DELETE",
                                    )
                                  )
                                    setDeleting(null);
                                }}
                              >
                                Delete channel
                              </button>
                              <button
                                className="text-button"
                                onClick={() => setDeleting(null)}
                              >
                                Cancel
                              </button>
                            </div>
                          )}
                        </td>
                      </>
                    )}
                  </tr>
                ))}
                {(!streams || streams.length === 0) && (
                  <tr>
                    <td colSpan={8}>
                      {streams
                        ? "No channels configured. Add an approved source in Stream Management."
                        : "Loading stream information…"}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </div>
  );
}
