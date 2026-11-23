import { useState } from "react";
import type { OperationsData, Stream } from "../types/operations";
import { mutate } from "../services/api";
import { rate, timestamp } from "../utils/format";
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
import { ChannelDialog } from "../components/streams/ChannelDialog";
import { StreamControls } from "../components/streams/StreamControls";
export function StreamsPage({
  data,
  refresh,
}: {
  data: OperationsData;
  refresh: () => void;
}) {
  const [search, setSearch] = useState("");
  const [state, setState] = useState("");
  const [source, setSource] = useState("");
  const [sort, setSort] = useState("name");
  const [descending, setDescending] = useState(false);
  const [page, setPage] = useState(0);
  const [editing, setEditing] = useState<Stream | "new" | null>(null);
  const [deleting, setDeleting] = useState<Stream | null>(null);
  const action = useAction();
  const items = data.streams
    .filter(
      (s) =>
        (!state || s.state === state) &&
        (!source || s.channel.source_type === source) &&
        `${s.channel.name} ${s.channel.id} ${s.channel.source_address} ${s.channel.interface}`
          .toLowerCase()
          .includes(search.toLowerCase()),
    )
    .sort((a, b) => {
      let result = 0;
      switch (sort) {
        case "clients":
          result = a.clients - b.clients;
          break;
        case "input":
          result = a.input_bitrate - b.input_bitrate;
          break;
        case "output":
          result = a.output_bitrate - b.output_bitrate;
          break;
        case "state":
          result = a.state.localeCompare(b.state);
          break;
        case "source":
          result = a.channel.source_address.localeCompare(
            b.channel.source_address,
          );
          break;
        default:
          result = a.channel.name.localeCompare(b.channel.name);
      }
      return descending ? -result : result;
    });
  const pages = Math.max(1, Math.ceil(items.length / 20));
  const current = Math.min(page, pages - 1);
  const visible = items.slice(current * 20, current * 20 + 20);
  return (
    <>
      {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
      <div className="op-toolbar">
        <Field label="Search streams">
          <input
            placeholder="Name, source or interface"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(0);
            }}
          />
        </Field>
        <Field label="Filter stream status">
          <select
            value={state}
            onChange={(e) => {
              setState(e.target.value);
              setPage(0);
            }}
          >
            <option value="">All states</option>
            {[
              "running",
              "stopped",
              "disabled",
              "waiting",
              "starting",
              "retrying",
              "error",
            ].map((s) => (
              <option key={s}>{s}</option>
            ))}
          </select>
        </Field>
        <Field label="Source type filter">
          <select
            value={source}
            onChange={(e) => {
              setSource(e.target.value);
              setPage(0);
            }}
          >
            <option value="">All sources</option>
            <option value="multicast">Multicast</option>
            <option value="unicast">Unicast</option>
          </select>
        </Field>
        <Field label="Sort streams">
          <select value={sort} onChange={(e) => setSort(e.target.value)}>
            {["name", "state", "source", "clients", "input", "output"].map(
              (s) => (
                <option key={s}>{s}</option>
              ),
            )}
          </select>
        </Field>
        <Button onClick={() => setDescending(!descending)}>
          {descending ? "Descending" : "Ascending"}
        </Button>
        <Button onClick={refresh}>Refresh streams</Button>
        <Button className="primary" onClick={() => setEditing("new")}>
          Add stream
        </Button>
      </div>
      <Panel
        title="Configured streams"
        description={`${items.length} matching channels • one shared receiver per source identity`}
      >
        {visible.length ? (
          <div className="op-table-wrap">
            <table className="op-stream-table">
              <thead>
                <tr>
                  <th>Channel</th>
                  <th>Source</th>
                  <th>Type / interface</th>
                  <th>Status</th>
                  <th>Clients</th>
                  <th>Input</th>
                  <th>Output</th>
                  <th>Last packet</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {visible.map((s) => (
                  <tr key={s.channel.id}>
                    <td data-label="Channel">
                      <a href={`#/streams/${s.channel.id}`}>
                        <strong>{s.channel.name}</strong>
                      </a>
                      <small>{s.channel.id}</small>
                    </td>
                    <td data-label="Source" className="mono">
                      {s.channel.source_address}:{s.channel.source_port}
                    </td>
                    <td data-label="Type / interface">
                      {s.channel.source_type}
                      <small>{s.channel.interface}</small>
                    </td>
                    <td data-label="Status">
                      <Badge state={s.state} />
                      {s.errors.length > 0 && (
                        <small>{s.errors.length} recent events</small>
                      )}
                    </td>
                    <td data-label="Clients">
                      {s.clients} / {s.channel.max_clients}
                    </td>
                    <td data-label="Input">{rate(s.input_bitrate)}</td>
                    <td data-label="Output">{rate(s.output_bitrate)}</td>
                    <td data-label="Last packet">
                      {timestamp(s.last_packet_at)}
                    </td>
                    <td data-label="Actions">
                      <StreamControls
                        stream={s}
                        data={data}
                        refresh={refresh}
                        edit={() => setEditing(s)}
                        remove={() => setDeleting(s)}
                        action={action}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty
            title={
              data.streams.length
                ? "No matching streams"
                : "No channels configured"
            }
          >
            Add a source approved by the server policy to begin receiving.
          </Empty>
        )}
        <div className="op-pagination">
          <Button disabled={current === 0} onClick={() => setPage(current - 1)}>
            Previous
          </Button>
          <span>
            Page {current + 1} of {pages}
          </span>
          <Button
            disabled={current + 1 >= pages}
            onClick={() => setPage(current + 1)}
          >
            Next
          </Button>
        </div>
      </Panel>
      {editing && (
        <ChannelDialog
          data={data}
          stream={editing === "new" ? undefined : editing}
          onClose={() => setEditing(null)}
          refresh={refresh}
        />
      )}
      {deleting && (
        <Dialog
          title={`Delete ${deleting.channel.name}`}
          onClose={() => !action.busy && setDeleting(null)}
          footer={
            <>
              <Button disabled={action.busy} onClick={() => setDeleting(null)}>
                Cancel
              </Button>
              <Button
                className="danger"
                disabled={action.busy}
                onClick={async () => {
                  if (
                    await action.run(
                      () =>
                        mutate(
                          `/api/v1/streams/${deleting.channel.id}`,
                          "DELETE",
                        ),
                      "Channel deleted and receiver cleanup confirmed",
                    )
                  ) {
                    setDeleting(null);
                    refresh();
                  }
                }}
              >
                Delete channel
              </Button>
            </>
          }
        >
          <p>
            This deletes the persistent channel and disconnects its{" "}
            {deleting.clients} active client(s). The receiver is cancelled and
            cleaned up. A failed save/delete is reported without claiming
            success.
          </p>
          {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
        </Dialog>
      )}
    </>
  );
}
