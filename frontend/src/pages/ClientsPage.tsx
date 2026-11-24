import { useMemo, useState } from "react";
import { Radio, RefreshCw, Users } from "lucide-react";
import type { OperationsData } from "../types/operations";
import { mutate } from "../services/api";
import { bytes, duration, rate, timestamp } from "../utils/format";
import {
  Badge,
  Button,
  DataTable,
  Dialog,
  Empty,
  ErrorBanner,
  Field,
  Panel,
  Stat,
  useAction,
} from "../components/ui";
export function ClientsPage({
  data,
  refresh,
}: {
  data: OperationsData;
  refresh: () => void;
}) {
  const [search, setSearch] = useState("");
  const [stream, setStream] = useState("");
  const [disconnect, setDisconnect] = useState<number | null>(null);
  const action = useAction();
  const names = useMemo(
    () => new Map(data.streams.map((s) => [s.channel.id, s.channel.name])),
    [data.streams],
  );
  const clients = data.clients.filter(
    (c) =>
      (!stream || c.channel_id === stream) &&
      `${c.remote_ip} ${names.get(c.channel_id) ?? c.channel_id}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  return (
    <>
      {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
      <div className="op-page-metrics op-client-metrics">
        <Stat label="Active connections" value={data.clients.length} icon={<Users size={19} />} />
        <Stat label="Channels with clients" value={new Set(data.clients.map(client => client.channel_id)).size} icon={<Radio size={19} />} />
      </div>
      <Panel
        title="Connected HTTP clients"
        description={`${clients.length} matching connections. Bitrate is averaged over each connection.`}
        action={<Button onClick={refresh}><RefreshCw size={15} /> Refresh clients</Button>}
      >
        <div className="op-toolbar op-filter-bar">
          <Field label="Search clients">
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="IP address or channel"
            />
          </Field>
          <Field label="Filter by stream">
            <select value={stream} onChange={(e) => setStream(e.target.value)}>
              <option value="">All streams</option>
              {data.streams.map((s) => (
                <option key={s.channel.id} value={s.channel.id}>
                  {s.channel.name}
                </option>
              ))}
            </select>
          </Field>
        </div>
        {clients.length ? (
          <DataTable>
            <thead role="rowgroup">
              <tr role="row">
                <th role="columnheader" scope="col">Stream</th>
                <th role="columnheader" scope="col">Client IP</th>
                <th role="columnheader" scope="col">Connected</th>
                <th role="columnheader" scope="col">Duration</th>
                <th role="columnheader" scope="col">Delivered</th>
                <th role="columnheader" scope="col">Average output</th>
                <th role="columnheader" scope="col">State</th>
                <th role="columnheader" scope="col">Actions</th>
              </tr>
            </thead>
            <tbody role="rowgroup">
              {clients.map((c) => (
                <tr role="row" key={c.id}>
                  <td role="cell" data-label="Stream" data-primary>
                    <a href={`#/streams/${c.channel_id}`}>
                      {names.get(c.channel_id) ?? c.channel_id}
                    </a>
                  </td>
                  <td role="cell" data-label="Client IP" className="mono">{c.remote_ip}</td>
                  <td role="cell" data-label="Connected">{timestamp(c.connected_at)}</td>
                  <td role="cell" data-label="Duration">{duration(c.duration_seconds)}</td>
                  <td role="cell" data-label="Delivered">{bytes(c.bytes_delivered)}</td>
                  <td role="cell" data-label="Average output">{rate(c.output_bitrate)}</td>
                  <td role="cell" data-label="State">
                    <Badge state={c.closing ? "closing" : "connected"} />
                  </td>
                  <td role="cell" data-label="Actions">
                    <Button
                      disabled={action.busy || c.closing}
                      onClick={() => setDisconnect(c.id)}
                    >
                      Disconnect
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        ) : (
          <Empty title={data.clients.length ? "No matching connections" : "No active connections"}>
            {data.clients.length ? "Try a different IP address or channel." : "Clients appear here when viewers request playback."}
          </Empty>
        )}
      </Panel>
      {disconnect !== null && (
        <Dialog
          title="Disconnect playback client"
          onClose={() => !action.busy && setDisconnect(null)}
          footer={
            <>
              <Button
                disabled={action.busy}
                onClick={() => setDisconnect(null)}
              >
                Cancel
              </Button>
              <Button
                className="danger"
                disabled={action.busy}
                onClick={async () => {
                  if (
                    await action.run(
                      () => mutate(`/api/v1/clients/${disconnect}`, "DELETE"),
                      "Disconnect requested. An in-flight write may wait for its deadline.",
                    )
                  ) {
                    setDisconnect(null);
                    refresh();
                  }
                }}
              >
                Disconnect client
              </Button>
            </>
          }
        >
          <p>
            This closes the selected subscription and clears its queue. Other
            viewers continue streaming.
          </p>
        </Dialog>
      )}
    </>
  );
}
