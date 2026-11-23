import { useMemo, useState } from "react";
import type { OperationsData } from "../types/operations";
import { mutate } from "../services/api";
import { bytes, duration, rate, timestamp } from "../utils/format";
import {
  Badge,
  Button,
  Dialog,
  Empty,
  ErrorBanner,
  Field,
  Panel,
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
      <div className="op-toolbar">
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
        <Button onClick={refresh}>Refresh clients</Button>
      </div>
      <Panel
        title="Connected HTTP clients"
        description="Direct peer addresses; forwarded headers are ignored. Output bitrate is the connection-lifetime average."
      >
        {clients.length ? (
          <div className="op-table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Stream</th>
                  <th>Client IP</th>
                  <th>Connected</th>
                  <th>Duration</th>
                  <th>Delivered</th>
                  <th>Average output</th>
                  <th>State</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {clients.map((c) => (
                  <tr key={c.id}>
                    <td>
                      <a href={`#/streams/${c.channel_id}`}>
                        {names.get(c.channel_id) ?? c.channel_id}
                      </a>
                    </td>
                    <td className="mono">{c.remote_ip}</td>
                    <td>{timestamp(c.connected_at)}</td>
                    <td>{duration(c.duration_seconds)}</td>
                    <td>{bytes(c.bytes_delivered)}</td>
                    <td>{rate(c.output_bitrate)}</td>
                    <td>
                      <Badge state={c.closing ? "closing" : "connected"} />
                    </td>
                    <td>
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
            </table>
          </div>
        ) : (
          <Empty title="No matching connections">
            Active playback clients will appear after a viewer requests a
            stream.
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
