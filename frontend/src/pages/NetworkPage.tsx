import type { OperationsData } from "../types/operations";
import { Diagnostics } from "../components/network/Diagnostics";
import { Badge, Button, Panel } from "../components/ui";
import {
  NetworkFields,
  useSettingsEditor,
} from "../components/network/SettingsEditor";
export function NetworkPage({
  data,
  refresh,
}: {
  data: OperationsData;
  refresh: () => void;
}) {
  const editor = useSettingsEditor(data.settings, refresh);
  const value = editor.draft;
  return (
    <>
      {editor.status}
      <Diagnostics data={data} />
      <Panel
        title="Detected network interfaces"
        description="Read from the OS; no addresses, links, routes or firewall rules are modified."
      >
        <div className="op-table-wrap">
          <table>
            <thead>
              <tr>
                <th>Interface / index</th>
                <th>IPv4 address</th>
                <th>MAC address</th>
                <th>Operational state</th>
                <th>Multicast</th>
                <th>MTU</th>
                <th>Driver errors / drops</th>
              </tr>
            </thead>
            <tbody>
              {data.interfaces.map((i) => (
                <tr key={i.index}>
                  <td>
                    <strong>{i.name}</strong>
                    <small>
                      Index {i.index}
                      {i.loopback ? " • loopback" : ""}
                    </small>
                  </td>
                  <td className="mono">
                    {i.ipv4.join(", ") || "No IPv4 address"}
                  </td>
                  <td className="mono">
                    {("mac" in i && String(i.mac)) || "Unavailable"}
                  </td>
                  <td>
                    <Badge state={i.up ? "up" : "down"} />
                  </td>
                  <td>{i.multicast ? "Supported" : "Unavailable"}</td>
                  <td>{i.mtu}</td>
                  <td>
                    {i.error_counters
                      ? `RX ${i.error_counters.rx_errors} / ${i.error_counters.rx_dropped}, TX ${i.error_counters.tx_errors} / ${i.error_counters.tx_dropped}`
                      : "Unavailable"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>
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
