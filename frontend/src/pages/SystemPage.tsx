import type { OperationsData } from "../types/operations";
import { bytes, duration, timestamp } from "../utils/format";
import { Badge, Definition, Panel } from "../components/ui";
export function SystemPage({ data }: { data: OperationsData }) {
  const s = data.system;
  return (
    <>
      <div className="op-two-column">
        <Panel title="Application & runtime">
          <Definition
            items={[
              ["Application version", s.version],
              ["Build commit", s.commit],
              ["Build date", s.build_date],
              ["Go runtime", s.go_version],
              ["Operating system", s.os],
              ["CPU architecture", s.architecture],
              ["Process ID", s.process_id],
              ["Started", timestamp(s.started_at)],
              ["Process uptime", duration(s.uptime_seconds)],
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
      <Panel title="HTTP listener status">
        <div className="op-table-wrap">
          <table>
            <thead>
              <tr>
                <th>Listener</th>
                <th>Actual address</th>
                <th>State</th>
                <th>Details</th>
              </tr>
            </thead>
            <tbody>
              {s.http_listeners.map((l, i) => (
                <tr key={i}>
                  <td>{l.kind}</td>
                  <td className="mono">{l.address}</td>
                  <td>
                    <Badge state={l.state} />
                  </td>
                  <td>{l.error || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>
      <Panel title="Detected network interfaces">
        <div className="op-table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name / index</th>
                <th>IPv4</th>
                <th>MAC</th>
                <th>State</th>
                <th>Multicast</th>
                <th>MTU</th>
              </tr>
            </thead>
            <tbody>
              {s.interfaces.map((i) => (
                <tr key={i.index}>
                  <td>
                    {i.name} / {i.index}
                  </td>
                  <td>{i.ipv4.join(", ") || "No IPv4"}</td>
                  <td>{("mac" in i && String(i.mac)) || "Unavailable"}</td>
                  <td>{i.up ? "Up" : "Down"}</td>
                  <td>{i.multicast ? "Supported" : "Unavailable"}</td>
                  <td>{i.mtu}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>
    </>
  );
}
