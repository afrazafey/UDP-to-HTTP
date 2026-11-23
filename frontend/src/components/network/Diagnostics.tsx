import { useState } from "react";
import type { OperationsData } from "../../types/operations";
import { api, mutate } from "../../services/api";
import { usePolling } from "../../hooks/usePolling";
import { Button, ErrorBanner, Field, Panel, useAction } from "../ui";
type Observation = {
  channel_id: string;
  configuration_valid: boolean;
  interface_error: string | null;
  socket_bound: boolean;
  multicast_membership_joined: boolean;
  last_datagram_at: string | null;
  last_valid_ts_at: string | null;
  mpeg_ts_packets: number;
  http_payload_bytes_delivered: number;
  active_viewers: number;
};
export function Diagnostics({ data }: { data: OperationsData }) {
  const poll = usePolling(
    (signal) =>
      api<{ channels: Observation[] }>("/api/v1/diagnostics", {}, signal),
    "network-diagnostics",
    data.settings.active.dashboard.refresh_seconds,
    false,
  );
  const [id, setID] = useState(data.streams[0]?.channel.id ?? "");
  const [result, setResult] = useState<unknown>();
  const action = useAction();
  return (
    <Panel
      title="Production network diagnostics"
      description="Independent observations: configuration, socket, membership, UDP arrival, TS framing, and delivered HTTP payload."
    >
      {(poll.error || action.error) && (
        <ErrorBanner>{poll.error || action.error}</ErrorBanner>
      )}
      <div className="op-toolbar">
        <Field label="Diagnostic channel">
          <select value={id} onChange={(e) => setID(e.target.value)}>
            <option value="">Select channel</option>
            {data.streams.map((s) => (
              <option key={s.channel.id} value={s.channel.id}>
                {s.channel.name}
              </option>
            ))}
          </select>
        </Field>
        {(
          [
            ["interface", "Validate interface"],
            ["source", "Check source"],
            ["multicast_eligibility", "Check multicast eligibility"],
            ["route", "Check interface selection"],
            ["listener_connectivity", "Test listener connectivity"],
            ["playback_ports", "Check playback ports"],
          ] as const
        ).map(([key, label]) => (
          <Button
            key={key}
            disabled={
              action.busy ||
              (!id &&
                key !== "listener_connectivity" &&
                key !== "playback_ports")
            }
            onClick={() =>
              void action.run(
                async () =>
                  setResult(
                    await mutate("/api/v1/diagnostics", "POST", {
                      action: key,
                      channel_id: id,
                    }),
                  ),
                "Diagnostic completed; inspect individual conditions",
              )
            }
          >
            {label}
          </Button>
        ))}
      </div>
      <div className="op-table-wrap">
        <table>
          <thead>
            <tr>
              <th>Channel</th>
              <th>Configuration</th>
              <th>Socket</th>
              <th>Membership</th>
              <th>UDP arrival</th>
              <th>Valid TS</th>
              <th>HTTP payload</th>
            </tr>
          </thead>
          <tbody>
            {poll.data?.channels.map((c) => (
              <tr key={c.channel_id}>
                <td>{c.channel_id}</td>
                <td>
                  {c.configuration_valid ? "Valid" : "Invalid"}
                  {c.interface_error && <small>{c.interface_error}</small>}
                </td>
                <td>{c.socket_bound ? "Bound" : "Not bound"}</td>
                <td>
                  {c.multicast_membership_joined
                    ? "Application joined"
                    : "Not joined / unicast"}
                </td>
                <td>
                  {c.last_datagram_at
                    ? new Date(c.last_datagram_at).toLocaleTimeString()
                    : "None observed"}
                </td>
                <td>{c.mpeg_ts_packets} TS packets</td>
                <td>
                  {c.http_payload_bytes_delivered} bytes to {c.active_viewers}{" "}
                  current viewers
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p>
        Driver error counters appear in the interface table where supported.
        Memberships owned by other applications are unavailable. A socket or
        membership does not prove source packets, valid media, or player
        decoding.
      </p>
      {result !== undefined && (
        <pre className="op-diagnostic-result" tabIndex={0}>
          {JSON.stringify(result, null, 2)}
        </pre>
      )}
    </Panel>
  );
}
