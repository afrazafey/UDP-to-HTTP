import { useState } from "react";
import { CirclePause, CirclePlay, CircleSlash, Download, Ellipsis, ListFilter, Plus, Radio, RefreshCw, Upload } from "lucide-react";
import type { Channel } from "../Streaming";
import type { OperationsData, Stream } from "../types/operations";
import { mutate } from "../services/api";
import { duration, rate, timestamp } from "../utils/format";
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
import { ChannelDialog } from "../components/streams/ChannelDialog";
import { StreamControls } from "../components/streams/StreamControls";
import { ChannelIdentity } from "../components/streams/ChannelMenu";
export function StreamsPage({
  data,
  refresh,
}: {
  data: OperationsData;
  refresh: () => void;
}) {
  const [search, setSearch] = useState("");
  const [group, setGroup] = useState("");
  const [selection, setSelection] = useState<Set<string>>(new Set());
  const [bulk, setBulk] = useState<string | null>(null);
  const [copy, setCopy] = useState<Channel | null>(null);
  const [state, setState] = useState("");
  const [enabled, setEnabled] = useState("");
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
        (!state || (s.health ?? s.state) === state || s.state === state) &&
        (!enabled || s.channel.enabled === (enabled === "enabled")) &&
        (!group || s.channel.group === group) &&
        (!source || s.channel.source_type === source) &&
        `${s.channel.name} ${s.channel.id} ${s.channel.source_address} ${s.channel.interface}`
          .toLowerCase()
          .includes(search.toLowerCase()),
    )
    .sort((a, b) => {
      let result = 0;
      switch (sort) {
        case "order":
          result = (a.channel.sort_order ?? 0) - (b.channel.sort_order ?? 0);
          break;
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
  const advancedFilters = Number(Boolean(enabled)) + Number(Boolean(source));
  return (
    <>
      {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
      <div className="op-page-metrics op-channel-metrics" aria-label="Channel health summary">
        <Stat label="Total channels" value={data.streams.length} icon={<Radio size={18} />} />
        <Stat label="Running" value={data.streams.filter(stream => stream.state === "running").length} icon={<CirclePlay size={18} />} />
        <Stat label="Recovering" value={data.streams.filter(stream => (stream.health ?? stream.state) === "recovering").length} icon={<RefreshCw size={18} />} />
        <Stat label="Offline" value={data.streams.filter(stream => stream.channel.enabled && (stream.health === "offline" || (!stream.health && stream.state === "stopped"))).length} icon={<CirclePause size={18} />} />
        <Stat label="Disabled" value={data.streams.filter(stream => !stream.channel.enabled).length} icon={<CircleSlash size={18} />} />
      </div>
      <Panel
        title="Configured channels"
        description={`${items.length} matching channels`}
        action={<div className="op-page-actions">
          <a className="op-button" href="#/import-export">
            <Upload size={15} /> Import channels
          </a>
          <a className="op-button" href="#/import-export?mode=export">
            <Download size={15} /> Export playlist
          </a>
          <Button className="primary" onClick={() => setEditing("new")}>
            <Plus size={16} /> Add channel
          </Button>
        </div>}
      >
        <div className="op-toolbar op-filter-bar">
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
          <Field label="Filter channel group">
            <select
              value={group}
              onChange={(e) => {
                setGroup(e.target.value);
                setPage(0);
              }}
            >
              <option value="">All groups</option>
              {[
                ...new Set(
                  data.streams.map((s) => s.channel.group || "").filter(Boolean),
                ),
              ]
                .sort()
                .map((g) => (
                  <option key={g}>{g}</option>
                ))}
            </select>
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
                "healthy",
                "recovering",
                "degraded",
                "offline",
                "waiting_for_source",
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
          <details className="op-channel-filters">
            <summary><ListFilter size={16} aria-hidden="true" /> More filters{advancedFilters > 0 ? ` (${advancedFilters})` : ""}</summary>
            <div className="op-filter-options">
              <Field label="Enabled state">
                <select
                  value={enabled}
                  onChange={(e) => {
                    setEnabled(e.target.value);
                    setPage(0);
                  }}
                >
                  <option value="">All channels</option>
                  <option value="enabled">Enabled</option>
                  <option value="disabled">Disabled</option>
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
                  {[
                    "name",
                    "order",
                    "state",
                    "source",
                    "clients",
                    "input",
                    "output",
                  ].map((s) => (
                    <option key={s}>{s}</option>
                  ))}
                </select>
              </Field>
              <Button className="op-sort-direction" onClick={() => setDescending(!descending)}>
                {descending ? "Descending" : "Ascending"}
              </Button>
            </div>
          </details>
        </div>
        {data.streams.length > 0 && (
          <div className="op-selection-bar">
            <strong aria-live="polite">{selection.size} selected</strong>
            <Button disabled={!visible.length} onClick={() => setSelection(new Set(visible.map(s => s.channel.id)))}>
              Select visible
            </Button>
            {selection.size > 0 && (
              <>
                <Button onClick={() => setSelection(new Set())}>Clear selection</Button>
                {(["enable", "disable", "delete"] as const).map(operation => (
                  <Button key={operation} disabled={action.busy} onClick={() => setBulk(operation)}>
                    Bulk {operation}
                  </Button>
                ))}
              </>
            )}
          </div>
        )}
        {visible.length ? (
          <DataTable>
            <thead role="rowgroup">
              <tr role="row">
                <th role="columnheader" scope="col">Select</th>
                <th role="columnheader" scope="col">Channel</th>
                <th role="columnheader" scope="col">Source</th>
                <th role="columnheader" scope="col">Status</th>
                <th role="columnheader" scope="col">Clients</th>
                <th role="columnheader" scope="col">Throughput</th>
                <th role="columnheader" scope="col">Receiver activity</th>
                <th role="columnheader" scope="col">Actions</th>
              </tr>
            </thead>
            <tbody role="rowgroup">
              {visible.map((s) => (
                <tr role="row" key={s.channel.id}>
                  <td role="cell" data-label="Select">
                    <input
                      type="checkbox"
                      aria-label={`Select channel ${s.channel.name}`}
                      checked={selection.has(s.channel.id)}
                      onChange={(e) =>
                        setSelection((old) => {
                          const next = new Set(old);
                          e.target.checked
                            ? next.add(s.channel.id)
                            : next.delete(s.channel.id);
                          return next;
                        })
                      }
                    />
                  </td>
                  <td role="cell" data-label="Channel" data-primary>
                    <ChannelIdentity stream={s} />
                  </td>
                  <td role="cell" data-label="Source">
                    <span className="mono">{s.channel.source_address}:{s.channel.source_port}</span>
                    <small>{s.channel.source_type}{s.channel.interface && ` · ${s.channel.interface}`}</small>
                  </td>
                  <td role="cell" data-label="Status">
                    <Badge state={s.health ?? s.state} />
                    {s.errors.length > 0 && (
                      <small>{s.errors.length} recent events</small>
                    )}
                  </td>
                  <td role="cell" data-label="Clients">
                    {s.clients} / {s.channel.max_clients}
                  </td>
                  <td role="cell" data-label="Throughput">
                    <span className="op-cell-line">UDP {rate(s.input_bitrate)}</span>
                    <span className="op-cell-line">HTTP {rate(s.output_bitrate)}</span>
                  </td>
                  <td role="cell" data-label="Receiver activity">
                    <span className="op-cell-line">Uptime {duration(s.uptime_seconds)}</span>
                    <small>Last packet {timestamp(s.last_packet_at)}</small>
                  </td>
                  <td role="cell" data-label="Actions">
                    <details className="op-channel-actions">
                      <summary aria-label={`Actions for ${s.channel.name}`}><Ellipsis size={17} aria-hidden="true" /> Actions</summary>
                      <StreamControls
                        stream={s}
                        data={data}
                        refresh={refresh}
                        edit={() => setEditing(s)}
                        remove={() => setDeleting(s)}
                        action={action}
                      />
                      <Button
                        onClick={() =>
                          setCopy({
                            ...s.channel,
                            id: `${s.channel.id.slice(0, 57)}-copy`,
                            name: `${s.channel.name.slice(0, 120)} copy`,
                          })
                        }
                      >
                        Duplicate channel
                      </Button>
                    </details>
                  </td>
                </tr>
              ))}
            </tbody>
          </DataTable>
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
        {pages > 1 && <div className="op-pagination">
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
        </div>}
      </Panel>
      {editing && (
        <ChannelDialog
          data={data}
          stream={editing === "new" ? undefined : editing}
          onClose={() => setEditing(null)}
          refresh={refresh}
        />
      )}
      {copy && (
        <ChannelDialog
          data={data}
          initial={copy}
          onClose={() => setCopy(null)}
          refresh={refresh}
        />
      )}
      {bulk && (
        <Dialog
          title={`Confirm bulk ${bulk}`}
          onClose={() => !action.busy && setBulk(null)}
          footer={
            <>
              <Button disabled={action.busy} onClick={() => setBulk(null)}>
                Cancel
              </Button>
              <Button
                className={bulk === "delete" ? "danger" : "primary"}
                disabled={action.busy}
                onClick={() =>
                  void action.run(async () => {
                    await mutate("/api/v1/streams/bulk", "POST", {
                      ids: [...selection],
                      action: bulk,
                      confirm: true,
                    });
                    setBulk(null);
                    setSelection(new Set());
                    refresh();
                  }, "Bulk operation committed atomically")
                }
              >
                Confirm bulk action
              </Button>
            </>
          }
        >
          <p>
            {bulk} {selection.size} selected channels. Changes persist in
            SQLite. Disabling, deleting, or updating a channel disconnects its
            active viewers.
          </p>
          {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
        </Dialog>
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
