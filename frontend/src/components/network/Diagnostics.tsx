import { useState } from "react";
import type { OperationsData } from "../../types/operations";
import { api, mutate } from "../../services/api";
import { usePolling } from "../../hooks/usePolling";
import { Button, DataTable, Empty, ErrorBanner, Field, Panel, useAction } from "../ui";
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
export function Diagnostics({
  data,
  paused = false,
}: {
  data: OperationsData;
  paused?: boolean;
}) {
  const poll = usePolling(
    (signal) =>
      api<{ channels: Observation[] }>("/api/v1/diagnostics", {}, signal),
    "network-diagnostics",
    data.settings.active.dashboard.refresh_seconds,
    paused,
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
      <div className="op-toolbar op-diagnostics-tools">
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
        <div className="op-page-actions">{(
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
        ))}</div>
      </div>
      {poll.data?.channels.length ? <DataTable>
        <thead role="rowgroup">
          <tr role="row">
            <th role="columnheader" scope="col">Channel</th>
            <th role="columnheader" scope="col">Configuration</th>
            <th role="columnheader" scope="col">Socket</th>
            <th role="columnheader" scope="col">Membership</th>
            <th role="columnheader" scope="col">UDP arrival</th>
            <th role="columnheader" scope="col">Valid TS</th>
            <th role="columnheader" scope="col">HTTP payload</th>
          </tr>
        </thead>
        <tbody role="rowgroup">
          {poll.data?.channels.map((c) => (
            <tr role="row" key={c.channel_id}>
              <td role="cell" data-label="Channel" data-primary>{c.channel_id}</td>
              <td role="cell" data-label="Configuration">
                {c.configuration_valid ? "Valid" : "Invalid"}
                {c.interface_error && <small>{c.interface_error}</small>}
              </td>
              <td role="cell" data-label="Socket">{c.socket_bound ? "Bound" : "Not bound"}</td>
              <td role="cell" data-label="Membership">
                {c.multicast_membership_joined
                  ? "Application joined"
                  : "Not joined / unicast"}
              </td>
              <td role="cell" data-label="UDP arrival">
                {c.last_datagram_at
                  ? new Date(c.last_datagram_at).toLocaleTimeString()
                  : "None observed"}
              </td>
              <td role="cell" data-label="Valid TS">{c.mpeg_ts_packets} TS packets</td>
              <td role="cell" data-label="HTTP payload">
                {c.http_payload_bytes_delivered} bytes to {c.active_viewers}{" "}
                current viewers
              </td>
            </tr>
          ))}
        </tbody>
      </DataTable> : <Empty title={poll.loading ? "Collecting receiver observations" : "No receiver observations available"} />}
      <p className="op-panel-note">
        Use UDP arrival, valid TS packets, and delivered HTTP payload to verify playback.
        Driver counters appear in the interface table where supported.
      </p>
      {result !== undefined && (
        <pre className="op-diagnostic-result" tabIndex={0}>
          {JSON.stringify(result, null, 2)}
        </pre>
      )}
    </Panel>
  );
}
