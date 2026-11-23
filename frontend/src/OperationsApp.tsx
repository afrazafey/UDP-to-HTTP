import { useEffect, useState } from "react";
import {
  Activity,
  Cable,
  ChevronRight,
  FileText,
  LayoutDashboard,
  Menu,
  Monitor,
  PanelLeftClose,
  PanelLeftOpen,
  Pause,
  Play,
  Radio,
  RefreshCw,
  Settings2,
  ShieldCheck,
  Users,
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
const navigation = [
  { path: "/overview", name: "Overview", icon: LayoutDashboard },
  { path: "/streams", name: "Streams", icon: Radio },
  { path: "/network", name: "Network Interfaces", icon: Cable },
  { path: "/clients", name: "Connected Clients", icon: Users },
  { path: "/monitoring", name: "Monitoring", icon: Activity },
  { path: "/logs", name: "Activity Logs", icon: FileText },
  { path: "/settings", name: "Settings", icon: Settings2 },
  { path: "/system", name: "System Information", icon: Monitor },
];
export function OperationsApp({ status }: { status: Status }) {
  const route = useRoute();
  const [paused, setPaused] = useState(false);
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
        case "/overview":
          page = <OverviewPage data={data} />;
          break;
        case "/streams":
          page = <StreamsPage data={data} refresh={poll.refresh} />;
          break;
        case "/network":
          page = <NetworkPage data={data} refresh={poll.refresh} />;
          break;
        case "/clients":
          page = <ClientsPage data={data} refresh={poll.refresh} />;
          break;
        case "/monitoring":
          page = <MonitoringPage data={data} />;
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
              <Radio size={24} />
            </span>
            <span>
              <strong>
                UDP<span>HTTP</span>
              </strong>
              <small>LINUX STREAM PROXY</small>
            </span>
          </a>
          <div className="op-nav-label">OPERATIONS</div>
          {links}
          <div className="op-sidebar-bottom">
            <div>
              <ShieldCheck size={18} />
              <span>
                Local administration<small>Loopback access only</small>
              </span>
            </div>
            <span className="op-dev-version">
              {status.version} · Development
            </span>
            <Button
              onClick={() => setCollapsed(!collapsed)}
              aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            >
              {collapsed ? (
                <PanelLeftOpen size={18} />
              ) : (
                <PanelLeftClose size={18} />
              )}
              <span>{collapsed ? "Expand" : "Collapse sidebar"}</span>
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
              <span>Workspace</span>
              <ChevronRight size={14} />
              <strong>{selected.name}</strong>
            </div>
            <div>
              <ShieldCheck size={15} />
              <span>Local instance</span>
              <a
                className="op-icon"
                href="#/settings"
                aria-label="Theme and application settings"
              >
                <Settings2 size={18} />
              </a>
            </div>
          </header>
          <main className="op-content" id="op-content" tabIndex={-1}>
            <div className="op-page-heading">
              <div>
                <span className="op-eyebrow">
                  {data?.settings.active.dashboard.name ?? "UDPHTTP PROXY"}
                </span>
                <h1>
                  {path.startsWith("/streams/")
                    ? "Stream details"
                    : selected.name}
                </h1>
                <p>
                  {path === "/network"
                    ? "Inspect interfaces and safely stage network configuration."
                    : path === "/streams"
                      ? "Manage approved IPTV sources and shared HTTP playback."
                      : "Live operational data from your proxy service."}
                </p>
              </div>
              <div className="op-heading-actions">
                <Button
                  onClick={() => setPaused(!paused)}
                  aria-pressed={paused}
                >
                  {paused ? <Play size={15} /> : <Pause size={15} />}{" "}
                  {paused ? "Resume monitoring" : "Pause monitoring"}
                </Button>
                <Button onClick={poll.refresh} disabled={poll.loading}>
                  <RefreshCw size={15} />
                  {poll.loading ? "Refreshing…" : "Refresh now"}
                </Button>
              </div>
            </div>
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
            {data && (
              <footer className="op-footer">
                <span>
                  Sample{" "}
                  {new Date(data.dashboard.sample.time).toLocaleTimeString()} ·
                  bounded history
                </span>
                <span>Development release · Linux acceptance pending</span>
                <a href="#/system">{status.version}</a>
              </footer>
            )}
          </main>
        </div>
      </div>
      {mobile && (
        <Dialog title="Navigation" onClose={() => setMobile(false)}>
          {links}
        </Dialog>
      )}
    </>
  );
}
