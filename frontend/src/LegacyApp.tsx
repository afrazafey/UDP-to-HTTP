import { useEffect, useState } from "react";
import {
  Activity,
  ArrowDownToLine,
  Cable,
  Check,
  ChevronRight,
  CircleHelp,
  Clock3,
  Database,
  ExternalLink,
  FileText,
  Gauge,
  Info,
  LayoutDashboard,
  Menu,
  Moon,
  Network,
  Radio,
  RefreshCw,
  Server,
  Settings2,
  ShieldCheck,
  Sun,
  Users,
  X,
} from "lucide-react";
import { request, type NetworkInterface, type Status } from "./api";
import { StreamingWorkspace } from "./Streaming";

const pages = [
  { name: "Overview", icon: LayoutDashboard },
  { name: "Network Interfaces", icon: Network },
  { name: "Stream Management", icon: Radio },
  { name: "Connected Clients", icon: Users },
  { name: "Monitoring", icon: Activity },
  { name: "Logs", icon: FileText },
  { name: "Settings", icon: Settings2 },
  { name: "About / Version", icon: Info },
] as const;
type Page = (typeof pages)[number]["name"];
const descriptions: Record<Page, string> = {
  Overview: "A clear view of your proxy service and network environment.",
  "Network Interfaces": "IPv4 interfaces discovered directly from this host.",
  "Stream Management": "UDP multicast and unicast stream administration.",
  "Connected Clients": "HTTP playback connections and access policies.",
  Monitoring: "Service health and availability, directly from the daemon.",
  Logs: "Structured service events and diagnostics.",
  Settings: "The configuration loaded when this service started.",
  "About / Version": "Build information and implementation status.",
};
const duration = (seconds: number) => {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor(seconds / 60) % 60;
  return hours ? `${hours}h ${minutes}m` : `${minutes}m ${seconds % 60}s`;
};
const bytes = (value: number) => `${(value / 1048576).toFixed(0)} MiB`;

function Pill({
  children,
  muted = false,
}: {
  children: React.ReactNode;
  muted?: boolean;
}) {
  return (
    <span className={`pill ${muted ? "muted" : ""}`}>
      {!muted && <span className="status-dot" />}
      {children}
    </span>
  );
}
function Empty({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="empty">
      <div className="empty-icon">
        <Radio size={26} />
      </div>
      <h3>{title}</h3>
      <p>{children}</p>
      <span className="phase-label">PLANNED · PHASE 2</span>
    </div>
  );
}
function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="detail-row">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

export function LegacyApp() {
  const [page, setPage] = useState<Page>("Overview");
  const [dark, setDark] = useState(
    () => localStorage.getItem("udphttp-theme") !== "light",
  );
  const [menu, setMenu] = useState(false);
  const [status, setStatus] = useState<Status | null>(null);
  const [interfaces, setInterfaces] = useState<NetworkInterface[]>([]);
  const [error, setError] = useState("");
  const [interfaceError, setInterfaceError] = useState("");
  const [updated, setUpdated] = useState<Date | null>(null);
  const [loading, setLoading] = useState(true);
  const [refresh, setRefresh] = useState(0);
  const [filter, setFilter] = useState("");
  useEffect(() => {
    document.documentElement.dataset.theme = dark ? "dark" : "light";
    localStorage.setItem("udphttp-theme", dark ? "dark" : "light");
  }, [dark]);
  useEffect(() => {
    let controller: AbortController | null = null;
    let active = true;
    let busy = false;
    const load = async () => {
      if (busy) return;
      busy = true;
      setLoading(true);
      controller = new AbortController();
      const current = controller;
      const timeout = setTimeout(() => current.abort(), 8000);
      const [service, adapters] = await Promise.allSettled([
        request<Status>("/api/v1/status", current.signal),
        request<{ interfaces: NetworkInterface[] }>(
          "/api/v1/interfaces",
          current.signal,
        ),
      ]);
      clearTimeout(timeout);
      busy = false;
      if (!active) return;
      if (service.status === "fulfilled") {
        setStatus(service.value);
        setError("");
        setUpdated(new Date());
      } else {
        setError(
          "The management service could not be reached. Start the daemon and check the listening address.",
        );
      }
      if (adapters.status === "fulfilled") {
        setInterfaces(adapters.value.interfaces);
        setInterfaceError("");
      } else
        setInterfaceError(
          "Network interface discovery is currently unavailable.",
        );
      setLoading(false);
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
  const selectPage = (selected: Page) => {
    setPage(selected);
    setMenu(false);
  };
  const connected = !!status && !error;
  const config = status?.configuration;
  const phase2 = (status?.phase ?? 1) >= 2;
  const streamingReady =
    connected && status.components.streaming === "available";
  const streamView = (
    view: "overview" | "streams" | "clients" | "monitoring",
  ) =>
    config ? (
      <StreamingWorkspace
        key={view}
        view={view}
        configuration={config}
        interfaces={interfaces}
      />
    ) : null;
  const visibleInterfaces = interfaces.filter((item) =>
    `${item.name} ${item.ipv4.join(" ")}`
      .toLowerCase()
      .includes(filter.toLowerCase()),
  );

  function interfaceTable(compact = false) {
    return interfaceError ? (
      <p className="inline-error" role="alert">
        {interfaceError}
      </p>
    ) : (
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th>Interface</th>
              <th>IPv4 address</th>
              <th>Status</th>
              {!compact && (
                <>
                  <th>Multicast</th>
                  <th>MTU</th>
                </>
              )}
            </tr>
          </thead>
          <tbody>
            {(compact ? interfaces.slice(0, 4) : visibleInterfaces).map(
              (item) => (
                <tr key={item.index}>
                  <td>
                    <span className="interface-name">
                      <Cable size={16} />
                      <span>
                        {item.name}
                        {item.loopback && <small>Loopback</small>}
                      </span>
                    </span>
                  </td>
                  <td className="mono">
                    {item.ipv4.length
                      ? item.ipv4.join(", ")
                      : "No IPv4 address"}
                  </td>
                  <td>
                    <Pill muted={!item.up}>{item.up ? "Up" : "Down"}</Pill>
                  </td>
                  {!compact && (
                    <>
                      <td>{item.multicast ? "Supported" : "Unavailable"}</td>
                      <td className="mono">{item.mtu}</td>
                    </>
                  )}
                </tr>
              ),
            )}
            {(compact ? interfaces : visibleInterfaces).length === 0 && (
              <tr>
                <td colSpan={compact ? 3 : 5}>
                  {loading
                    ? "Discovering interfaces…"
                    : "No matching network interfaces."}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    );
  }
  return (
    <div className="app-shell">
      {menu && (
        <button
          className="sidebar-scrim"
          aria-label="Close navigation"
          onClick={() => setMenu(false)}
        />
      )}
      <aside className={`sidebar ${menu ? "open" : ""}`}>
        <a
          className="brand"
          href="#overview"
          onClick={() => selectPage("Overview")}
        >
          <span className="brand-mark">
            <Radio size={23} />
          </span>
          <span>
            UDP<span className="brand-light">HTTP</span>
            <small>LINUX STREAM PROXY</small>
          </span>
        </a>
        <span className="nav-caption">WORKSPACE</span>
        <nav aria-label="Main navigation">
          {pages.map(({ name, icon: Icon }) => (
            <button
              key={name}
              className={`nav-item ${page === name ? "active" : ""}`}
              onClick={() => selectPage(name)}
              aria-current={page === name ? "page" : undefined}
            >
              <Icon size={18} />
              <span>{name}</span>
              {!phase2 &&
                (name === "Stream Management" ||
                  name === "Connected Clients") && (
                  <span className="nav-soon">Soon</span>
                )}
            </button>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="local-note">
            <ShieldCheck size={18} />
            <div>
              Local administration<small>Loopback access only</small>
            </div>
          </div>
          <div className="sidebar-version">
            <span>{phase2 ? "Streaming release" : "Foundation release"}</span>
            <span className="mono">{status?.version ?? "Phase 1"}</span>
          </div>
        </div>
      </aside>
      <div className="main-shell">
        <header className="topbar">
          <div className="breadcrumb">
            <button
              className="icon-button mobile-menu"
              aria-label="Open navigation"
              onClick={() => setMenu(true)}
            >
              <Menu size={20} />
            </button>
            <span>Workspace</span>
            <ChevronRight size={14} />
            <strong>{page}</strong>
          </div>
          <div className="topbar-actions">
            <span className="local-badge">
              <ShieldCheck size={14} /> Local instance
            </span>
            <span className="topbar-divider" />
            <button
              className="icon-button"
              onClick={() => setDark((value) => !value)}
              aria-label={
                dark ? "Switch to light theme" : "Switch to dark theme"
              }
            >
              {dark ? <Sun size={18} /> : <Moon size={18} />}
            </button>
            <button
              className="icon-button"
              aria-label="About this application"
              onClick={() => selectPage("About / Version")}
            >
              <CircleHelp size={18} />
            </button>
          </div>
        </header>
        <main>
          <div className="page-heading">
            <div>
              <div className="eyebrow">PROXY MANAGEMENT</div>
              <h1>{page === "Overview" ? "Service overview" : page}</h1>
              <p>{descriptions[page]}</p>
            </div>
            <button
              className="button"
              onClick={() => setRefresh((value) => value + 1)}
              disabled={loading}
            >
              <RefreshCw size={15} />
              {loading ? "Refreshing…" : "Refresh status"}
            </button>
          </div>
          {error && (
            <div className="error-banner" role="alert">
              <X size={18} />
              <span>
                {error}
                {status && " Showing the last successful response."}
              </span>
            </div>
          )}
          {!status && !error ? (
            <section className="panel loading-panel" role="status">
              Connecting to the management service…
            </section>
          ) : (
            <>
              {page === "Overview" && (
                <>
                  <div className="foundation-banner">
                    <div className="banner-icon">
                      <Info size={19} />
                    </div>
                    <div>
                      <strong>
                        {phase2
                          ? "Streaming administration"
                          : "Your foundation is ready."}
                      </strong>
                      <p>
                        {phase2
                          ? "Manage approved UDP sources below. Playback uses separate listeners and client access restrictions. Linux runtime validation is pending."
                          : "Service administration is available. UDP streaming and playback arrive in Phase 2."}
                      </p>
                    </div>
                    <button onClick={() => selectPage("About / Version")}>
                      Release details <ExternalLink size={14} />
                    </button>
                  </div>
                  <div className="stat-grid">
                    <section className="stat-card">
                      <div className="stat-top">
                        <span>Management service</span>
                        <Server size={18} />
                      </div>
                      <div className="stat-value">
                        {connected
                          ? status.status === "running"
                            ? "Online"
                            : "Degraded"
                          : "Unreachable"}
                        <span
                          className={`service-indicator ${connected && status?.status === "running" ? "healthy" : ""}`}
                        />
                      </div>
                      <small>
                        {connected
                          ? `${config?.management.host}:${config?.management.port}`
                          : "Waiting for a response"}
                      </small>
                    </section>
                    <section className="stat-card">
                      <div className="stat-top">
                        <span>Service uptime</span>
                        <Clock3 size={18} />
                      </div>
                      <div className="stat-value mono">
                        {connected ? duration(status.uptime_seconds) : "—"}
                      </div>
                      <small>
                        {connected
                          ? "Since the current service started"
                          : "Unavailable"}
                      </small>
                    </section>
                    <section className="stat-card">
                      <div className="stat-top">
                        <span>Network interfaces</span>
                        <Network size={18} />
                      </div>
                      <div className="stat-value mono">
                        {interfaceError || !connected ? "—" : interfaces.length}
                        <span className="stat-unit">detected</span>
                      </div>
                      <small>
                        {interfaceError || !connected
                          ? "Discovery unavailable"
                          : `${interfaces.filter((i) => i.up).length} up · ${interfaces.filter((i) => i.multicast).length} multicast capable`}
                      </small>
                    </section>
                    <section className="stat-card">
                      <div className="stat-top">
                        <span>Streaming engine</span>
                        <Radio size={18} />
                      </div>
                      <div className="stat-value unavailable">
                        {phase2
                          ? connected
                            ? status.components.streaming
                            : "Unknown"
                          : "Unavailable"}
                      </div>
                      <small>
                        {phase2
                          ? "See channel states for reception health"
                          : "Scheduled for Phase 2"}
                      </small>
                    </section>
                  </div>
                  <div className="overview-grid">
                    <section className="panel">
                      <div className="panel-heading">
                        <div>
                          <h2>Network interfaces</h2>
                          <p>Discovered on the service host</p>
                        </div>
                        <button
                          className="text-button"
                          onClick={() => selectPage("Network Interfaces")}
                        >
                          View all <ExternalLink size={13} />
                        </button>
                      </div>
                      {interfaceTable(true)}
                      <div className="panel-foot">
                        <ShieldCheck size={14} />
                        No host network settings are modified.
                      </div>
                    </section>
                    <section className="panel">
                      <div className="panel-heading">
                        <div>
                          <h2>Service health</h2>
                          <p>Foundation component availability</p>
                        </div>
                        <Gauge size={18} />
                      </div>
                      <div className="health-list">
                        <Row
                          label="Management API"
                          value={
                            <Pill muted={!connected}>
                              {connected ? "Available" : "Unreachable"}
                            </Pill>
                          }
                        />
                        <Row
                          label="SQLite storage"
                          value={
                            <Pill
                              muted={
                                !connected ||
                                status?.components.storage !== "ready"
                              }
                            >
                              {connected
                                ? status.components.storage === "ready"
                                  ? "Ready"
                                  : "Unavailable"
                                : "Unknown"}
                            </Pill>
                          }
                        />
                        <Row
                          label={phase2 ? "UDP engine" : "UDP reception"}
                          value={
                            <Pill muted={!streamingReady}>
                              {streamingReady
                                ? "Ready"
                                : phase2
                                  ? "Disabled / unknown"
                                  : "Unavailable"}
                            </Pill>
                          }
                        />
                        <Row
                          label="HTTP playback"
                          value={
                            <Pill muted={!streamingReady}>
                              {streamingReady
                                ? "Policy restricted"
                                : phase2
                                  ? "Disabled / unknown"
                                  : "Unavailable"}
                            </Pill>
                          }
                        />
                      </div>
                      <div className="health-note">
                        <Database size={16} />
                        <span>
                          Startup configuration is recorded in local SQLite
                          storage.
                        </span>
                      </div>
                    </section>
                  </div>
                  {phase2 ? (
                    streamView("overview")
                  ) : (
                    <section className="panel stream-preview">
                      <div className="panel-heading">
                        <div>
                          <h2>Stream activity</h2>
                          <p>Playback and traffic monitoring</p>
                        </div>
                        <Pill muted>Phase 2</Pill>
                      </div>
                      <Empty title="Streaming is not available yet">
                        Multicast reception, HTTP playback, and traffic
                        statistics will appear here when the streaming engine is
                        implemented.
                      </Empty>
                    </section>
                  )}
                </>
              )}
              {page === "Network Interfaces" && (
                <section className="panel">
                  <div className="panel-heading">
                    <div>
                      <h2>Host interfaces</h2>
                      <p>
                        Read-only discovery · no elevated privileges required
                      </p>
                    </div>
                    <label className="search-label">
                      <span className="sr-only">Filter interfaces</span>
                      <input
                        placeholder="Filter by name or IPv4…"
                        value={filter}
                        onChange={(event) => setFilter(event.target.value)}
                      />
                    </label>
                  </div>
                  {interfaceTable()}
                  <div className="panel-foot">
                    Choose the multicast interface in your YAML configuration.
                    Restart the service to load changes.
                  </div>
                </section>
              )}
              {(page === "Stream Management" || page === "Connected Clients") &&
                (phase2 ? (
                  streamView(
                    page === "Stream Management" ? "streams" : "clients",
                  )
                ) : (
                  <section className="panel">
                    <Empty
                      title={
                        page === "Stream Management"
                          ? "The streaming engine is planned for Phase 2"
                          : "Client tracking is unavailable"
                      }
                    >
                      {page === "Stream Management"
                        ? "UDP multicast and unicast reception, shared upstream receivers, stream buffering, and playback URLs have not been implemented."
                        : "HTTP playback is not implemented. There are no client counts or traffic measurements to display."}
                    </Empty>
                  </section>
                ))}
              {page === "Monitoring" &&
                (phase2 ? (
                  streamView("monitoring")
                ) : (
                  <section className="panel">
                    <div className="panel-heading">
                      <div>
                        <h2>Service availability</h2>
                        <p>
                          Live component status, refreshed every five seconds
                        </p>
                      </div>
                      <Activity size={20} />
                    </div>
                    <div className="settings-body">
                      <Row
                        label="Management service"
                        value={connected ? status.status : "Unreachable"}
                      />
                      <Row
                        label="Storage"
                        value={
                          connected ? status.components.storage : "Unknown"
                        }
                      />
                      <Row
                        label="Uptime at last refresh"
                        value={
                          connected
                            ? duration(status.uptime_seconds)
                            : "Unavailable"
                        }
                      />
                      <Row
                        label="Traffic statistics"
                        value="Unavailable · Phase 2"
                      />
                    </div>
                    <div className="panel-foot">
                      Traffic charts and stream measurements will be available
                      after the streaming engine is implemented.
                    </div>
                  </section>
                ))}
              {page === "Logs" && (
                <section className="panel">
                  <div className="panel-heading">
                    <div>
                      <h2>Service logs</h2>
                      <p>
                        The browser log viewer is unavailable. Recent stream
                        errors appear with each channel.
                      </p>
                    </div>
                    <FileText size={20} />
                  </div>
                  <div className="log-instructions">
                    <h3>Read structured logs with journald</h3>
                    <p>
                      When installed as a systemd service, JSON logs are
                      collected by the journal.
                    </p>
                    <pre>
                      <code>journalctl -u udphttp-proxy -f -o cat</code>
                    </pre>
                    <p>
                      During development, logs are written to standard error.
                      File logging and browser log streaming are planned.
                    </p>
                  </div>
                </section>
              )}
              {page === "Settings" && (
                <>
                  <div className="foundation-banner">
                    <Info size={20} />
                    <div>
                      <strong>Read-only configuration</strong>
                      <p>
                        {phase2
                          ? "Server settings and access policies are loaded from YAML at startup. Manage persistent channels on the Stream Management page."
                          : "Edit the YAML file and restart the service. Browser configuration writes are unavailable."}
                      </p>
                    </div>
                  </div>
                  {config && (
                    <div className="settings-grid">
                      <section className="panel">
                        <div className="panel-heading">
                          <h2>Management listener</h2>
                          <ShieldCheck size={18} />
                        </div>
                        <div className="settings-body">
                          <Row
                            label="Bind address"
                            value={config.management.host}
                          />
                          <Row label="Port" value={config.management.port} />
                          <Row
                            label="Read / write timeout"
                            value={`${config.management.read_timeout} / ${config.management.write_timeout}`}
                          />
                          <Row
                            label="Idle timeout"
                            value={config.management.idle_timeout}
                          />
                          <Row
                            label="Shutdown deadline"
                            value={config.management.shutdown_timeout}
                          />
                          <Row
                            label="Remote administration"
                            value="Unavailable"
                          />
                        </div>
                      </section>
                      <section className="panel">
                        <div className="panel-heading">
                          <h2>
                            {phase2
                              ? "Streaming server settings"
                              : "Reserved streaming settings"}
                          </h2>
                          <Pill muted>
                            {phase2
                              ? config.streaming.enabled
                                ? "Enabled"
                                : "Disabled"
                              : "Inactive"}
                          </Pill>
                        </div>
                        <div className="settings-body">
                          <Row
                            label="Multicast interface"
                            value={
                              config.network.multicast_interface ||
                              "Not selected"
                            }
                          />
                          <Row
                            label="HTTP interface"
                            value={
                              config.network.http_interface || "Not selected"
                            }
                          />
                          <Row
                            label="HTTP ports"
                            value={`${config.network.http_port}${config.network.secondary_http_port ? `, ${config.network.secondary_http_port}` : " (secondary disabled)"}`}
                          />
                          <Row
                            label="UDP receive / stream buffer"
                            value={`${bytes(config.streaming.udp_receive_buffer)} / ${bytes(config.streaming.stream_buffer)}`}
                          />
                          <Row
                            label="Maximum clients"
                            value={config.streaming.max_clients}
                          />
                          <Row
                            label="Connection timeout"
                            value={config.streaming.connect_timeout}
                          />
                        </div>
                      </section>
                      <section className="panel">
                        <div className="panel-heading">
                          <h2>Persistence & logging</h2>
                          <Database size={18} />
                        </div>
                        <div className="settings-body">
                          <Row
                            label="SQLite path"
                            value={
                              <span className="mono">
                                {config.storage.path}
                              </span>
                            }
                          />
                          <Row label="Log level" value={config.logging.level} />
                          <Row
                            label="Log format"
                            value="JSON · standard error"
                          />
                          <Row
                            label="Configuration source"
                            value="Startup YAML + environment"
                          />
                        </div>
                      </section>
                      <section className="panel">
                        <div className="panel-heading">
                          <h2>
                            {phase2
                              ? "Stream access policy"
                              : "Reserved stream access policy"}
                          </h2>
                          <Pill muted>{phase2 ? "Enforced" : "Inactive"}</Pill>
                        </div>
                        <div className="settings-body">
                          <Row
                            label="Allowed client networks"
                            value={
                              config.streaming.allowed_client_networks.join(
                                ", ",
                              ) || "None · deny all"
                            }
                          />
                          <Row
                            label="Allowed multicast groups"
                            value={
                              config.streaming.allowed_multicast_groups.join(
                                ", ",
                              ) || "None · deny all"
                            }
                          />
                          <Row
                            label="Allowed unicast sources"
                            value={
                              config.streaming.allowed_unicast_sources.join(
                                ", ",
                              ) || "None · deny all"
                            }
                          />
                        </div>
                      </section>
                    </div>
                  )}
                </>
              )}
              {page === "About / Version" && (
                <section className="panel about-panel">
                  <div className="about-brand">
                    <span className="brand-mark">
                      <Radio size={28} />
                    </span>
                    <div>
                      <h2>UDPHTTP Proxy</h2>
                      <p>A standalone Linux IPTV proxy foundation.</p>
                    </div>
                  </div>
                  <div className="settings-body">
                    <Row
                      label="Version"
                      value={status?.version ?? "Unavailable"}
                    />
                    <Row
                      label="Build commit"
                      value={
                        <span className="mono">
                          {status?.commit ?? "Unavailable"}
                        </span>
                      }
                    />
                    <Row
                      label="Build date"
                      value={status?.build_date ?? "Unavailable"}
                    />
                    <Row
                      label="Platform"
                      value={status?.platform ?? "Unavailable"}
                    />
                    <Row
                      label="Go runtime"
                      value={status?.go_version ?? "Unavailable"}
                    />
                    <Row
                      label="Release scope"
                      value={
                        phase2
                          ? "Phase 2 · Streaming engine"
                          : "Phase 1 · Foundation"
                      }
                    />
                  </div>
                  <div className="about-note">
                    <Check size={18} />
                    <p>
                      {phase2
                        ? "Independently implemented MPEG-TS forwarding without transcoding. Linux runtime validation is pending; production readiness has not been established. Legacy Windows URL compatibility has not been verified."
                        : "Independently implemented. No transcoding or UDP forwarding is included in this release. Legacy Windows URL compatibility has not been verified."}
                    </p>
                  </div>
                </section>
              )}
            </>
          )}
          <footer>
            <span>
              <span className={`status-dot ${connected ? "" : "offline"}`} />
              {connected
                ? "Connected to local service"
                : error
                  ? "Service connection unavailable"
                  : "Connecting"}
            </span>
            <span>
              {updated
                ? `Last response ${updated.toLocaleTimeString()}`
                : "Awaiting first response"}
              <span className="footer-separator">·</span>Refreshes every 5s
            </span>
            <span className="footer-brand">
              <ArrowDownToLine size={13} /> UDPHTTP / Phase {phase2 ? "2" : "1"}
            </span>
          </footer>
        </main>
      </div>
    </div>
  );
}
