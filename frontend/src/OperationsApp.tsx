import { useEffect, useState } from "react";
import {
  Activity,
  Bell,
  CirclePlay,
  Clock3,
  Cpu,
  Database,
  Download,
  ChevronRight,
  FileText,
  House,
  ListMusic,
  Menu,
  Monitor,
  Network,
  PanelLeftClose,
  PanelLeftOpen,
  Pause,
  Play,
  Radio,
  RefreshCw,
  Settings,
  Users,
  Plus,
} from "lucide-react";
import type { Status } from "./types/operations";
import { useOperations } from "./hooks/useOperations";
import { useRoute } from "./hooks/usePolling";
import { mutate } from "./services/api";
import {
  Button,
  Dialog,
  ErrorBanner,
  Skeleton,
  useAction,
} from "./components/ui";
import { OverviewPage } from "./pages/OverviewPage";
import { StreamsPage } from "./pages/StreamsPage";
import { StreamDetailsPage } from "./pages/StreamDetailsPage";
import { NetworkPage } from "./pages/NetworkPage";
import { ClientsPage } from "./pages/ClientsPage";
import { MonitoringPage } from "./pages/MonitoringPage";
import { LogsPage } from "./pages/LogsPage";
import { SettingsPage } from "./pages/SettingsPage";
import { SystemPage } from "./pages/SystemPage";
import { PlaylistsPage } from "./pages/PlaylistsPage";
import { ChannelDialog } from "./components/streams/ChannelDialog";
import { Badge } from "./components/ui";
import { DeveloperAttribution, EnvironmentIndicator } from "./components/ui/Branding";
import { bytes, duration } from "./utils/format";
const navigation = [
  { path: "/overview", name: "Overview", icon: House },
  { path: "/streams", name: "Channels", icon: CirclePlay },
  { path: "/playlists", name: "Playlists", icon: ListMusic },
  { path: "/import-export", name: "Import / Export", icon: Download },
  { path: "/network", name: "Network", icon: Network },
  { path: "/monitoring", name: "Monitoring", icon: Activity },
  { path: "/logs", name: "Activity Logs", icon: FileText },
  { path: "/settings", name: "Settings", icon: Settings },
  { path: "/clients", name: "Connected Clients", icon: Users },
  { path: "/system", name: "System Information", icon: Monitor },
];
const pageDescriptions: Record<string, string> = {
  "/overview": "Live operational data from your UDPHTTP Proxy service.",
  "/streams": "Manage channel sources, receivers, and HTTP playback.",
  "/playlists": "Preview and download the M3U playlist for enabled channels.",
  "/import-export": "Import channel lists or export a playback playlist.",
  "/network": "Inspect network interfaces, test playback, and stage configuration.",
  "/monitoring": "Track throughput, receiver health, and process resources.",
  "/logs": "Search retained events and export filtered diagnostics.",
  "/settings": "Configure the dashboard, streaming engine, and playback policies.",
  "/clients": "Inspect and manage active playback connections.",
  "/system": "Inspect the running build, listeners, and storage status.",
};
export function OperationsApp({ status }: { status: Status }) {
  const route = useRoute();
  const [paused, setPaused] = useState(false);
  const [addChannel, setAddChannel] = useState(false);
  const [mobile, setMobile] = useState(false);
  const [collapsed, setCollapsed] = useState(
    () => localStorage.getItem("udphttp-sidebar") === "collapsed",
  );
  const poll = useOperations(paused);
  const action = useAction();
  const data = poll.data;
  const path = route.split("?")[0];
  const selected =
    navigation.find((n) => path === n.path || path.startsWith(`${n.path}/`)) ??
    navigation[0];
  const theme = data?.settings.active.dashboard.theme ?? "system";
  const serviceState = poll.error
    ? "stale"
    : data
      ? status.status === "degraded" || !data.dashboard.database_ready
        ? "degraded"
        : "running"
      : status.status;
  const sample = data?.dashboard.sample;
  const environment = data?.system.environment ?? status.environment;
  useEffect(() => {
    const media = window.matchMedia?.("(prefers-color-scheme: dark)");
    const apply = () => {
      document.documentElement.dataset.theme =
        theme === "system" ? (media?.matches ? "dark" : "light") : theme;
    };
    apply();
    media?.addEventListener?.("change", apply);
    return () => media?.removeEventListener?.("change", apply);
  }, [theme]);
  useEffect(() => {
    localStorage.setItem(
      "udphttp-sidebar",
      collapsed ? "collapsed" : "expanded",
    );
  }, [collapsed]);
  useEffect(() => {
    document.title = `${selected.name} · ${data?.settings.active.dashboard.name ?? "UDPHTTP"}`;
  }, [selected.name, data?.settings.active.dashboard.name]);
  const links = (
    <nav aria-label="Main navigation">
      {navigation.map(({ path: target, name, icon: Icon }) => (
        <a
          key={target}
          href={`#${target}`}
          className={selected.path === target ? "active" : ""}
          aria-current={selected.path === target ? "page" : undefined}
          aria-label={collapsed ? name : undefined}
          title={collapsed ? name : undefined}
          onClick={() => setMobile(false)}
        >
          <Icon size={19} />
          <span>{name}</span>
        </a>
      ))}
    </nav>
  );
  let page: React.ReactNode = null;
  if (data) {
    if (path.startsWith("/streams/")) {
      page = (
        <StreamDetailsPage
          key={path}
          id={path.slice(9)}
          data={data}
          refresh={poll.refresh}
          interval={poll.interval}
          paused={paused}
        />
      );
    } else
      switch (path) {
        case "/playlists":
          page = (
            <PlaylistsPage data={data} refresh={poll.refresh} mode="export" />
          );
          break;
        case "/import-export":
          page = (
            <PlaylistsPage
              key={route}
              data={data}
              refresh={poll.refresh}
              mode={route.endsWith("?mode=export") ? "export" : "import"}
            />
          );
          break;
        case "/overview":
          page = (
            <OverviewPage
              data={data}
              refresh={poll.refresh}
              serviceState={serviceState}
            />
          );
          break;
        case "/streams":
          page = <StreamsPage data={data} refresh={poll.refresh} />;
          break;
        case "/network":
          page = (
            <NetworkPage data={data} refresh={poll.refresh} paused={paused} />
          );
          break;
        case "/clients":
          page = <ClientsPage data={data} refresh={poll.refresh} />;
          break;
        case "/monitoring":
          page = (
            <MonitoringPage
              data={data}
              paused={paused}
              interval={poll.interval}
            />
          );
          break;
        case "/logs":
          page = (
            <LogsPage
              key={route}
              data={data}
              paused={paused}
              interval={poll.interval}
              initialStream={
                new URLSearchParams(route.split("?")[1]).get("stream_id") ?? ""
              }
              initialComponent={
                new URLSearchParams(route.split("?")[1]).get("component") ?? ""
              }
            />
          );
          break;
        case "/settings":
          page = <SettingsPage data={data} refresh={poll.refresh} />;
          break;
        case "/system":
          page = <SystemPage data={data} />;
          break;
        default:
          page = (
            <div className="op-prose">
              <h2>Page not found</h2>
              <a href="#/overview">Return to overview</a>
            </div>
          );
      }
  }
  return (
    <>
      <div
        id="ops-shell"
        className={`op-shell ${collapsed ? "is-collapsed" : ""}`}
      >
        <a
          className="op-skip"
          href="#op-content"
          onClick={(event) => {
            event.preventDefault();
            document.getElementById("op-content")?.focus();
          }}
        >
          Skip to content
        </a>
        <aside className="op-sidebar">
          <a className="op-brand" href="#/overview">
            <span className="op-brand-symbol">
              <Radio size={24} aria-hidden="true" />
            </span>
            <span>
              <strong>UDPHTTP</strong>
              <small>Stream Proxy</small>
            </span>
          </a>
          {links}
          <div className="op-sidebar-bottom">
            <section className="op-service-summary" aria-label="Service status">
              <div className="op-service-title">
                <span className={`op-service-light op-state-${serviceState}`} aria-hidden="true" />
                <div>
                  <strong>Service {serviceState}</strong>
                  <small>{status.version}</small>
                </div>
              </div>
              <dl>
                <div>
                  <dt><Clock3 size={15} aria-hidden="true" /> Uptime</dt>
                  <dd>
                    {data
                      ? duration(data.dashboard.uptime_seconds)
                      : "Unavailable"}
                  </dd>
                </div>
                <div>
                  <dt><Cpu size={15} aria-hidden="true" /> CPU</dt>
                  {sample?.cpu_percent != null && (
                    <meter aria-hidden="true" min={0} max={100} value={sample.cpu_percent} />
                  )}
                  <dd>
                    {sample?.cpu_percent == null
                      ? "Unavailable"
                      : `${sample.cpu_percent.toFixed(1)}%`}
                  </dd>
                </div>
                <div>
                  <dt><Database size={15} aria-hidden="true" /> Memory</dt>
                  <dd>{bytes(sample?.rss_bytes)}</dd>
                </div>
              </dl>
              <div className="op-sidebar-attribution">
                <EnvironmentIndicator value={environment} />
                <DeveloperAttribution />
              </div>
            </section>
            <Button
              className="op-sidebar-toggle"
              onClick={() => setCollapsed(!collapsed)}
              aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
              title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            >
              {collapsed ? (
                <PanelLeftOpen size={18} />
              ) : (
                <PanelLeftClose size={18} />
              )}
            </Button>
          </div>
        </aside>
        <div className="op-main">
          <header className="op-topbar">
            <div>
              <button
                className="op-icon op-mobile-toggle"
                onClick={() => setMobile(true)}
                aria-label="Open navigation"
                aria-expanded={mobile}
              >
                <Menu size={21} />
              </button>
              <a href="#/overview" className="op-breadcrumb-root">
                Workspace
              </a>
              <ChevronRight size={14} />
              <strong>{selected.name}</strong>
            </div>
            <div>
              <span className="op-topbar-status"><span className="op-dot" aria-hidden="true" /><Badge state={serviceState} /></span>
              <a href="#/system" className="op-instance"><Monitor size={16} aria-hidden="true" /> Local instance</a>
              <a href="#/logs" className="op-icon op-topbar-logs" aria-label="Open activity logs" title="Activity logs">
                <Bell size={17} aria-hidden="true" />
              </a>
              <select
                aria-label="Dashboard theme"
                className="op-theme-control"
                value={theme}
                disabled={action.busy || !data}
                onChange={(event) => {
                  if (!data) return;
                  const next = event.target.value;
                  void action.run(async () => {
                    await mutate("/api/v1/settings", "PUT", {
                      revision: data.settings.revision,
                      confirm: true,
                      settings: {
                        ...data.settings.settings,
                        dashboard: {
                          ...data.settings.settings.dashboard,
                          theme: next,
                        },
                      },
                    });
                    poll.refresh();
                  }, "Theme saved");
                }}
              >
                <option value="system">System</option>
                <option value="light">Light</option>
                <option value="dark">Dark</option>
              </select>
            </div>
          </header>
          <main className={`op-content ${path === "/overview" ? "op-content-overview" : "op-workspace"}`} id="op-content" tabIndex={-1}>
            <div className="op-page-heading">
              <div>
                <h1>
                  {path.startsWith("/streams/")
                    ? "Stream details"
                    : selected.name}
                </h1>
                {pageDescriptions[path] && <p>{pageDescriptions[path]}</p>}
              </div>
              <div className="op-heading-actions">
                <Button
                  onClick={() => setPaused(!paused)}
                  aria-pressed={paused}
                >
                  {paused ? <Play size={15} /> : <Pause size={15} />}{" "}
                  {paused ? "Resume monitoring" : "Pause monitoring"}
                </Button>
                <Button className="op-refresh-button" onClick={poll.refresh} disabled={poll.loading} aria-busy={poll.loading}>
                  <RefreshCw size={15} />
                  {poll.loading ? "Refreshing…" : "Refresh now"}
                </Button>
                {path === "/overview" && (
                  <Button
                    className="primary"
                    disabled={!data}
                    onClick={() => setAddChannel(true)}
                  >
                    <Plus size={16} />
                    Add Channel
                  </Button>
                )}
              </div>
            </div>
            {poll.error && (
              <ErrorBanner>
                {poll.error}.{" "}
                {data
                  ? "Showing the last successful response; measurements may be stale."
                  : "No operations data is available. Check the daemon and retry."}
              </ErrorBanner>
            )}
            {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
            {!data && poll.loading ? <Skeleton /> : page}
            <div className="op-monitor-bar">
              <span
                className={`op-dot ${poll.error ? "error" : paused ? "paused" : ""}`}
              />
              <strong>
                {poll.error
                  ? "Disconnected / stale"
                  : paused
                    ? "Monitoring paused"
                    : "Live monitoring"}
              </strong>
              <span>
                {poll.updated
                  ? `Last response ${poll.updated.toLocaleTimeString()}`
                  : "Awaiting service data"}
              </span>
              <label>
                Refresh interval
                <select
                  aria-label="Monitoring refresh interval"
                  value={poll.interval}
                  disabled={action.busy || !data}
                  onChange={(event) => {
                    if (!data) return;
                    const seconds = Number(event.target.value);
                    void action.run(async () => {
                      await mutate("/api/v1/settings", "PUT", {
                        revision: data.settings.revision,
                        settings: {
                          ...data.settings.settings,
                          dashboard: {
                            ...data.settings.settings.dashboard,
                            refresh_seconds: seconds,
                          },
                        },
                        confirm: true,
                      });
                      poll.refresh();
                    }, "Dashboard refresh interval saved");
                  }}
                >
                  {[3, 5, 10, 30, 60].includes(poll.interval) ? null : (
                    <option value={poll.interval}>{poll.interval}s</option>
                  )}
                  {[3, 5, 10, 30, 60].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {seconds}s
                    </option>
                  ))}
                </select>
              </label>
            </div>
            {data && (
              <footer className="op-footer">
                <span>
                  Sample{" "}
                  {new Date(data.dashboard.sample.time).toLocaleTimeString()} ·
                  bounded history
                </span>
                <EnvironmentIndicator value={environment} />
                <a href="#/system">{status.version}</a>
              </footer>
            )}
          </main>
        </div>
      </div>
      {mobile && (
        <Dialog
          title="Navigation"
          variant="drawer"
          onClose={() => setMobile(false)}
          footer={
            <div className="op-drawer-status">
              <span><Badge state={serviceState} /> {status.version}</span>
              <EnvironmentIndicator value={environment} />
              <DeveloperAttribution />
            </div>
          }
        >
          {links}
        </Dialog>
      )}
      {addChannel && data && (
        <ChannelDialog
          data={data}
          onClose={() => setAddChannel(false)}
          refresh={poll.refresh}
        />
      )}
    </>
  );
}
