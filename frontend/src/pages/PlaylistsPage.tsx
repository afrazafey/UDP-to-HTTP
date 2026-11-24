import { useEffect, useState } from "react";
import { ClipboardList, Download, FileUp } from "lucide-react";
import type { Channel } from "../Streaming";
import type { OperationsData } from "../types/operations";
import { api, download, mutate } from "../services/api";
import {
  Button,
  CopyButton,
  DataTable,
  Dialog,
  ErrorBanner,
  Empty,
  Field,
  Panel,
  useAction,
} from "../components/ui";
import { ChannelIdentity } from "../components/streams/ChannelMenu";

type Entry = {
  line: number;
  channel: Channel;
  error: string;
  duplicate: string;
};
type PlaylistConfig = {
  playlist: { advertised_host: string; remote_enabled: boolean };
  primary_port: number;
  secondary_port: number;
};
export function saveText(text: string, filename: string) {
  const url = URL.createObjectURL(
    new Blob([text], { type: "application/json" }),
  );
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
export function PlaylistsPage({
  data,
  refresh,
  mode = "both",
}: {
  data: OperationsData;
  refresh: () => void;
  mode?: "both" | "import" | "export";
}) {
  const action = useAction();
  const [config, setConfig] = useState<PlaylistConfig>();
  const [configError, setConfigError] = useState("");
  const [text, setText] = useState("");
  const [iface, setIface] = useState(
    data.settings.active.network.multicast_interface,
  );
  const [entries, setEntries] = useState<Entry[]>([]);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [update, setUpdate] = useState(false);
  const [confirm, setConfirm] = useState<Channel[] | null>(null);
  const [group, setGroup] = useState("");
  const [port, setPort] = useState("preferred");
  const [fileError, setFileError] = useState("");
  const [imported, setImported] = useState<number>();
  const [previewed, setPreviewed] = useState(false);
  const loadFile = async (file?: File) => {
    if (!file) return;
    setFileError("");
    if (file.size > 1048576 || !/\.m3u8?$/i.test(file.name)) {
      setFileError("Choose an M3U/M3U8 file up to 1 MiB");
      return;
    }
    try {
      setText(await file.text());
      setEntries([]);
      setSelected(new Set());
      setImported(undefined);
      setPreviewed(false);
    } catch (error) {
      setFileError(String(error));
    }
  };
  useEffect(() => {
    if (mode === "import") return;
    const controller = new AbortController();
    void api<PlaylistConfig>("/api/v1/playlist/config", {}, controller.signal)
      .then(setConfig)
      .catch((e) => {
        if (!controller.signal.aborted) setConfigError(String(e.message));
      });
    return () => controller.abort();
  }, [mode]);
  const query = new URLSearchParams({
    port,
    ...(group ? { group } : {}),
  }).toString();
  const exportPath = `/api/v1/playlist.m3u?${query}`;
  const canSelect = (e: Entry) =>
    !e.error && (e.duplicate !== "existing" || update);
  const playlistURL = config?.playlist.remote_enabled
    ? `http://${config.playlist.advertised_host}:${port === "secondary" ? config.secondary_port : config.primary_port}/playlist.m3u?${query}`
    : `${window.location.origin}${exportPath}`;
  const enabled = data.streams.filter((s) => s.channel.enabled);
  const preview = enabled
    .filter((s) => !group || s.channel.group === group)
    .sort(
      (a, b) =>
        (a.channel.sort_order ?? 0) - (b.channel.sort_order ?? 0) ||
        a.channel.id.localeCompare(b.channel.id),
    );
  return (
    <>
      {(action.error || configError || fileError) && (
        <ErrorBanner>{action.error || configError || fileError}</ErrorBanner>
      )}
      {mode !== "import" && (
        <div className="op-playlist-layout">
          <Panel
            title="Export options"
            className="op-playlist-options"
          >
            <div className="op-playlist-host">
              <span>Advertised host</span>
              <strong className="mono">
                {config?.playlist.advertised_host ||
                  (config ? "Unconfigured" : "Unavailable")}
              </strong>
              <p>{config?.playlist.remote_enabled
                ? "Remote download uses both playlist and playback IP policies."
                : "Playlist URLs require management access."}</p>
              {config && !config.playlist.advertised_host && <p>Set playlist.advertised_host in the server configuration to enable export.</p>}
            </div>
            <div className="op-toolbar">
              <Field label="Export group">
                <select value={group} onChange={(e) => setGroup(e.target.value)}>
                  <option value="">All groups</option>
                  {[
                    ...new Set(
                      data.streams
                        .map((s) => s.channel.group || "")
                        .filter(Boolean),
                    ),
                  ]
                    .sort()
                    .map((g) => (
                      <option key={g}>{g}</option>
                    ))}
                </select>
              </Field>
              <Field label="Playback port">
                <select value={port} onChange={(e) => setPort(e.target.value)}>
                  <option value="preferred">Channel preference</option>
                  <option value="primary">Primary</option>
                  <option value="secondary" disabled={!config?.secondary_port}>
                    Secondary
                  </option>
                </select>
              </Field>
              <Button
                className="primary"
                disabled={action.busy || !config?.playlist.advertised_host}
                onClick={() =>
                  void action.run(
                    () => download(exportPath, "udphttp-channels.m3u"),
                    "Playlist downloaded",
                  )
                }
              >
                <Download size={16} /> Download M3U
              </Button>
              {config?.playlist.advertised_host && (
                <CopyButton value={playlistURL} label="Copy playlist URL" />
              )}
            </div>
          </Panel>
          <Panel
            title="Playlist preview"
            description={`${preview.length} channels shown · ${enabled.length} enabled`}
            action={mode === "export" && <a className="op-button" href="#/import-export"><FileUp size={16} /> Import channels</a>}
          >
            {preview.length ? <DataTable caption="Playlist order for the selected group">
              <thead role="rowgroup">
                <tr role="row">
                  <th role="columnheader" scope="col">Channel</th>
                  <th role="columnheader" scope="col">Group</th>
                  <th role="columnheader" scope="col">Playback port</th>
                </tr>
              </thead>
              <tbody role="rowgroup">
                {preview.map(stream => {
                  const { channel } = stream;
                  const secondary =
                    port === "secondary" ||
                    (port === "preferred" &&
                      channel.preferred_playback_port === "secondary");
                  return (
                    <tr role="row" key={channel.id}>
                      <td role="cell" data-label="Channel" data-primary><ChannelIdentity stream={stream} /></td>
                      <td role="cell" data-label="Group">{channel.group || "Ungrouped"}</td>
                      <td role="cell" data-label="Playback port">
                        {(secondary
                          ? config?.secondary_port
                          : config?.primary_port) || "Unavailable"}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </DataTable> : <Empty title="No enabled channels match this group" />}
          </Panel>
        </div>
      )}
      {mode !== "export" && (
        <Panel
          title="Import channels"
          description="Upload or paste an M3U playlist, then preview the channels before saving."
          action={mode === "import" && <a className="op-button" href="#/import-export?mode=export"><Download size={16} /> Export playlist</a>}
        >
          {previewed && <p className="op-panel-note op-import-status" role="status">
            {imported != null
              ? `Imported ${imported} channel(s).`
              : entries.length
                ? "Review validation and resolve duplicates, then confirm your selection."
                : "No channels were found in the playlist."}
          </p>}
          <div className="op-import-layout">
            <div className="op-import-source">
              <div
                className="op-dropzone"
                onDragOver={(event) => event.preventDefault()}
                onDrop={(event) => {
                  event.preventDefault();
                  if (!action.busy) void loadFile(event.dataTransfer.files[0]);
                }}
              >
                <FileUp size={24} aria-hidden="true" />
                <Field
                  label="M3U or M3U8 file"
                  help="Drop a file here or browse."
                >
                  <input
                    type="file"
                    accept=".m3u,.m3u8"
                    disabled={action.busy}
                    onChange={(e) => void loadFile(e.target.files?.[0])}
                  />
                </Field>
              </div>
              <Field label="Import UDP interface">
                <select
                  value={iface}
                  onChange={(e) => {
                    setIface(e.target.value);
                    setEntries([]);
                    setImported(undefined);
                    setPreviewed(false);
                  }}
                >
                  <option value="">Select interface</option>
                  {data.interfaces
                    .filter((i) => i.up && i.ipv4.length)
                    .map((i) => (
                      <option key={i.index} value={i.name}>
                        {i.name} — {i.ipv4.join(", ")}
                      </option>
                    ))}
                </select>
              </Field>
            </div>
            <Field label="Playlist text">
              <textarea
                rows={8}
                value={text}
                maxLength={1048576}
                onChange={(e) => {
                  setText(e.target.value);
                  setEntries([]);
                  setImported(undefined);
                  setPreviewed(false);
                  setSelected(new Set());
                }}
                placeholder={
                  '#EXTM3U\n#EXTINF:-1 group-title="MUSIC",Channel name\nudp://@239.1.1.61:1234'
                }
              />
            </Field>
          </div>
          <div className="op-toolbar">
            <Button className="primary"
              disabled={action.busy || !text || !iface}
              onClick={() =>
                void action.run(async () => {
                  const result = await mutate<{ entries: Entry[] }>(
                    "/api/v1/import/preview",
                    "POST",
                    { text, interface: iface },
                  );
                  setEntries(result.entries);
                  setPreviewed(true);
                  setImported(undefined);
                  setSelected(
                    new Set(
                      result.entries.flatMap((row, i) =>
                        !row.error && row.duplicate !== "existing" ? [i] : [],
                      ),
                    ),
                  );
                }, "Import preview ready")
              }
            >
              <ClipboardList size={16} /> Preview import
            </Button>
            <label className="op-checkbox-label">
              <input
                type="checkbox"
                checked={update}
                onChange={(e) => {
                  setUpdate(e.target.checked);
                  setSelected(new Set());
                }}
              />{" "}
              Allow selected existing channels to be updated after confirmation
            </label>
          </div>
          {entries.length > 0 && (
            <>
              <DataTable>
                <thead role="rowgroup">
                  <tr role="row">
                    <th role="columnheader" scope="col">Select</th>
                    <th role="columnheader" scope="col">Channel</th>
                    <th role="columnheader" scope="col">Source / port</th>
                    <th role="columnheader" scope="col">Type / interface</th>
                    <th role="columnheader" scope="col">Group</th>
                    <th role="columnheader" scope="col">Duplicate</th>
                    <th role="columnheader" scope="col">Validation</th>
                  </tr>
                </thead>
                <tbody role="rowgroup">
                  {entries.map((row, i) => (
                    <tr role="row" key={`${row.line}-${i}`}>
                      <td role="cell" data-label="Select">
                        <input
                          aria-label={`Select import ${row.channel.name}`}
                          type="checkbox"
                          disabled={!canSelect(row)}
                          checked={selected.has(i)}
                          onChange={(e) =>
                            setSelected((old) => {
                              const next = new Set(old);
                              e.target.checked ? next.add(i) : next.delete(i);
                              return next;
                            })
                          }
                        />
                      </td>
                      <td role="cell" data-label="Channel" data-primary>{row.channel.name}</td>
                      <td role="cell" data-label="Source / port">
                        {row.channel.source_address}:{row.channel.source_port}
                      </td>
                      <td role="cell" data-label="Type / interface">
                        {row.channel.source_type}
                        <small>{row.channel.interface}</small>
                      </td>
                      <td role="cell" data-label="Group">{row.channel.group || "—"}</td>
                      <td role="cell" data-label="Duplicate">{row.duplicate}</td>
                      <td role="cell" data-label="Validation">{row.error || "Valid against current policy"}</td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
              <div className="op-toolbar">
                <Button
                  disabled={action.busy || !selected.size}
                  onClick={() =>
                    setConfirm(
                      entries
                        .filter((e, i) => selected.has(i) && canSelect(e))
                        .map((e) => e.channel),
                    )
                  }
                >
                  Import selected
                </Button>
                <Button
                  disabled={action.busy || !entries.some(canSelect)}
                  onClick={() =>
                    setConfirm(entries.filter(canSelect).map((e) => e.channel))
                  }
                >
                  Import all valid channels
                </Button>
                <Button
                  onClick={() =>
                    saveText(
                      JSON.stringify(
                        entries
                          .filter((e) => e.error)
                          .map((e) => ({
                            line: e.line,
                            id: e.channel.id,
                            error: e.error,
                            duplicate: e.duplicate,
                          })),
                        null,
                        2,
                      ),
                      "import-errors.json",
                    )
                  }
                >
                  Download validation errors
                </Button>
              </div>
              <p className="op-panel-note">
                Rejected source addresses require a separate, explicit allowlist
                review in Security settings. Preview again after the authorized
                policy becomes active.
              </p>
            </>
          )}
        </Panel>
      )}
      {confirm && (
        <Dialog
          title="Confirm atomic channel import"
          onClose={() => !action.busy && setConfirm(null)}
          footer={
            <>
              <Button disabled={action.busy} onClick={() => setConfirm(null)}>
                Cancel
              </Button>
              <Button
                disabled={action.busy}
                onClick={() =>
                  void action.run(async () => {
                    const result = await mutate<{ imported: number }>(
                      "/api/v1/import/commit",
                      "POST",
                      {
                        channels: confirm,
                        update_existing: update,
                        confirm: true,
                      },
                    );
                    setImported(result.imported ?? confirm.length);
                    setConfirm(null);
                    setEntries([]);
                    refresh();
                  }, "Import committed atomically")
                }
              >
                Confirm import
              </Button>
            </>
          }
        >
          <p>
            Save {confirm.length} channels in one transaction.{" "}
            {update
              ? "Selected existing channels may be replaced and their viewers disconnected."
              : "Existing channels are skipped."}{" "}
            If any selected channel fails validation or persistence, nothing is
            imported.
          </p>
          {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
        </Dialog>
      )}
    </>
  );
}
