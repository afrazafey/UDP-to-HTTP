import type { OperationsData } from "../types/operations";
import { Clock3, Database, Monitor, Server } from "lucide-react";
import { bytes, duration, timestamp } from "../utils/format";
import { Badge, CopyButton, DataTable, Definition, Empty, Panel, Stat } from "../components/ui";
import { DeveloperAttribution, EnvironmentIndicator } from "../components/ui/Branding";
export function SystemPage({ data }: { data: OperationsData }) {
  const s = data.system;
  const diagnostic = JSON.stringify(
    {
      version: s.version,
      commit: s.commit,
      built: s.build_date,
      os: s.os,
      architecture: s.architecture,
      go: s.go_version,
      uptime_seconds: s.uptime_seconds,
      database_ready: s.database_ready,
      database_schema: s.database_schema ?? null,
      listener_states: s.http_listeners.map((l) => ({
        kind: l.kind,
        state: l.state,
      })),
    },
    null,
    2,
  );
  return (
    <>
      <div className="op-page-metrics op-system-metrics">
        <Stat label="Application version" value={s.version} icon={<Server size={18} />} />
        <Stat label="Platform" value={`${s.os} / ${s.architecture}`} icon={<Monitor size={18} />} />
        <Stat label="Process uptime" value={duration(s.uptime_seconds)} icon={<Clock3 size={18} />} />
        <Stat label="Database status" value={s.database_ready ? "Ready" : "Unavailable"} icon={<Database size={18} />} />
      </div>
      <Panel title="HTTP listener status">
        {s.http_listeners.length ? <DataTable>
          <thead role="rowgroup">
            <tr role="row">
              <th role="columnheader" scope="col">Listener</th>
              <th role="columnheader" scope="col">Actual address</th>
              <th role="columnheader" scope="col">State</th>
              <th role="columnheader" scope="col">Details</th>
            </tr>
          </thead>
          <tbody role="rowgroup">
            {s.http_listeners.map((l, i) => (
              <tr role="row" key={i}>
                <td role="cell" data-label="Listener" data-primary>{l.kind}</td>
                <td role="cell" data-label="Actual address" className="mono">{l.address}</td>
                <td role="cell" data-label="State">
                  <Badge state={l.state} />
                </td>
                <td role="cell" data-label="Details">{l.error || "—"}</td>
              </tr>
            ))}
          </tbody>
        </DataTable> : <Empty title="No HTTP listeners reported" />}
      </Panel>
      <div className="op-workspace-columns">
        <Panel
          title="Application & runtime"
          action={
            <CopyButton
              value={diagnostic}
              label="Copy diagnostic information"
            />
          }
        >
          <Definition
            items={[
              ["Build commit", s.commit],
              ["Build date", Number.isFinite(Date.parse(s.build_date)) ? timestamp(s.build_date) : "Unavailable"],
              ["Go runtime", s.go_version],
              ["Process ID", s.process_id],
              ["Started", timestamp(s.started_at)],
              ["Goroutines", s.goroutines],
            ]}
          />
        </Panel>
        <Panel title="Storage & management">
          <Definition
            items={[
              ["Configuration path", <code>{s.configuration_path}</code>],
              [
                "Database",
                <Badge state={s.database_ready ? "ready" : "error"} />,
              ],
              ["Management policy", s.management_policy],
              ["Database schema", s.database_schema ?? "Unavailable"],
              ["Remote administration", s.remote_authentication],
              [
                "Trusted reverse proxies",
                s.trusted_proxies.length
                  ? s.trusted_proxies.join(", ")
                  : "None; forwarded headers ignored",
              ],
              ["Dropped operational log events", s.dropped_log_events],
              ["Linux acceptance", s.linux_acceptance],
              ["Go heap", bytes(s.resources.heap_bytes)],
              ["Process RSS", bytes(s.resources.rss_bytes)],
            ]}
          />
        </Panel>
      </div>
      <Panel title="Detected network interfaces">
        {s.interfaces.length ? <DataTable>
          <thead role="rowgroup">
            <tr role="row">
              <th role="columnheader" scope="col">Name / index</th>
              <th role="columnheader" scope="col">IPv4</th>
              <th role="columnheader" scope="col">MAC</th>
              <th role="columnheader" scope="col">State</th>
              <th role="columnheader" scope="col">Multicast</th>
              <th role="columnheader" scope="col">MTU</th>
            </tr>
          </thead>
          <tbody role="rowgroup">
            {s.interfaces.map((i) => (
              <tr role="row" key={i.index}>
                <td role="cell" data-label="Name / index" data-primary>
                  {i.name} / {i.index}
                </td>
                <td role="cell" data-label="IPv4">{i.ipv4.join(", ") || "No IPv4"}</td>
                <td role="cell" data-label="MAC">{("mac" in i && String(i.mac)) || "Unavailable"}</td>
                <td role="cell" data-label="State">{i.up ? "Up" : "Down"}</td>
                <td role="cell" data-label="Multicast">{i.multicast ? "Supported" : "Unavailable"}</td>
                <td role="cell" data-label="MTU">{i.mtu}</td>
              </tr>
            ))}
          </tbody>
        </DataTable> : <Empty title="No interfaces reported" />}
      </Panel>
      <Panel title="About">
        <Definition items={[
          ["Application", s.application ?? "UDPHTTP Proxy"],
          ["Release maturity", s.release_maturity ?? (s.version.includes("-") ? "Development or candidate build" : "Versioned build")],
          ["Deployment environment", <EnvironmentIndicator value={s.environment} />],
          ["Developer", <DeveloperAttribution />],
          ["Year", new Date().getFullYear()],
        ]} />
      </Panel>
    </>
  );
}
