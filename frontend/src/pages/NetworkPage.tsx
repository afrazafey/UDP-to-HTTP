import type { OperationsData } from "../types/operations";
import { ArrowDownToLine, ArrowUpFromLine, Radio } from "lucide-react";
import { Diagnostics } from "../components/network/Diagnostics";
import { Badge, Button, DataTable, Empty, Panel } from "../components/ui";
import {
  NetworkFields,
  useSettingsEditor,
} from "../components/network/SettingsEditor";
export function NetworkPage({
  data,
  refresh,
  paused = false,
}: {
  data: OperationsData;
  refresh: () => void;
  paused?: boolean;
}) {
  const editor = useSettingsEditor(data.settings, refresh);
  const value = editor.draft;
  return (
    <>
      {editor.status}
      <Panel title="Network topology" className="op-network-topology">
        <div className="op-topology">
          <div>
            <ArrowDownToLine size={20} aria-hidden="true" />
            <strong>UDP input</strong>
            <span>
              {data.settings.active.network.multicast_interface ||
                "Selected per channel"}
            </span>
          </div>
          <div>
            <Radio size={20} aria-hidden="true" />
            <strong>UDPHTTP Proxy</strong>
            <span>
              {String(
                data.settings.read_only.management_binding ??
                "Loopback management",
              )}
            </span>
          </div>
          <div>
            <ArrowUpFromLine size={20} aria-hidden="true" />
            <strong>HTTP playback</strong>
            <span>
              {data.settings.active.network.http_interface} ·{" "}
              {data.settings.active.network.http_port}
              {data.settings.active.network.secondary_http_port
                ? ` / ${data.settings.active.network.secondary_http_port}`
                : ""}
            </span>
          </div>
        </div>
      </Panel>
      <Panel
        title="Detected network interfaces"
        description="Read from the OS; no addresses, links, routes or firewall rules are modified."
      >
        {data.interfaces.length ? <DataTable>
          <thead role="rowgroup">
            <tr role="row">
              <th role="columnheader" scope="col">Interface / index</th>
              <th role="columnheader" scope="col">IPv4 address</th>
              <th role="columnheader" scope="col">MAC address</th>
              <th role="columnheader" scope="col">Operational state</th>
              <th role="columnheader" scope="col">Multicast</th>
              <th role="columnheader" scope="col">MTU</th>
              <th role="columnheader" scope="col">Configured role</th>
              <th role="columnheader" scope="col">Driver errors / drops</th>
            </tr>
          </thead>
          <tbody role="rowgroup">
            {data.interfaces.map((i) => (
              <tr role="row" key={i.index}>
                <td role="cell" data-label="Interface / index" data-primary>
                  <strong>{i.name}</strong>
                  <small>
                    Index {i.index}
                    {i.loopback ? " • loopback" : ""}
                  </small>
                </td>
                <td role="cell" data-label="IPv4 address" className="mono">
                  {i.ipv4.join(", ") || "No IPv4 address"}
                </td>
                <td role="cell" data-label="MAC address" className="mono">
                  {("mac" in i && String(i.mac)) || "Unavailable"}
                </td>
                <td role="cell" data-label="Operational state">
                  <Badge state={i.up ? "up" : "down"} />
                </td>
                <td role="cell" data-label="Multicast">{i.multicast ? "Supported" : "Unavailable"}</td>
                <td role="cell" data-label="MTU">{i.mtu}</td>
                <td role="cell" data-label="Configured role">
                  {[
                    [
                      data.settings.active.network.multicast_interface,
                      ...data.streams.map((s) => s.channel.interface),
                    ].some(
                      (value) => value === i.name || i.ipv4.includes(value),
                    )
                      ? "UDP input"
                      : "",
                    [
                      data.settings.active.network.http_interface,
                      ...(data.settings.active.network.http_bind_addresses ??
                        []),
                    ].some(
                      (value) =>
                        value === i.name ||
                        value === "0.0.0.0" ||
                        i.ipv4.includes(value),
                    )
                      ? "HTTP playback"
                      : "",
                    i.loopback ? "Loopback management" : "",
                  ]
                    .filter(Boolean)
                    .join(" · ") || "Unassigned"}
                </td>
                <td role="cell" data-label="Driver errors / drops">
                  {i.error_counters
                    ? `RX ${i.error_counters.rx_errors} / ${i.error_counters.rx_dropped}, TX ${i.error_counters.tx_errors} / ${i.error_counters.tx_dropped}`
                    : "Unavailable"}
                </td>
              </tr>
            ))}
          </tbody>
        </DataTable> : <Empty title="No interfaces reported" />}
      </Panel>
      <Diagnostics data={data} paused={paused} />
      <Panel
        title="Network configuration"
        description="Persisted settings for the next service start. Management binding is read-only."
      >
        <form
          onSubmit={(event) => {
            event.preventDefault();
            void editor.validate();
          }}
        >
          <NetworkFields
            value={value.network}
            onChange={(network) => editor.setDraft({ ...value, network })}
            interfaces={data.interfaces}
          />
          <div className="op-callout subtle">
            <strong>Management endpoint preserved</strong>
            <p>
              {String(data.settings.read_only.management_binding)} • Local-only
              management is independent from the playback listeners.
            </p>
            <p>
              Port probes are point-in-time checks. Startup rechecks actual
              binds and restores the last accepted settings if a staged bind
              fails.
            </p>
          </div>
          <div className="op-form-actions">
            <Button
              type="submit"
              className="primary"
              disabled={
                editor.busy || editor.revision !== data.settings.revision
              }
            >
              {editor.busy ? "Validating…" : "Validate & preview configuration"}
            </Button>
          </div>
        </form>
      </Panel>
      {editor.dialog}
    </>
  );
}
