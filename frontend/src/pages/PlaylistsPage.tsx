import { useState } from "react";
import { ClipboardList, Download, FileUp } from "lucide-react";
import type { Channel } from "../Streaming";
import type { OperationsData } from "../types/operations";
import { mutate } from "../services/api";
import {
  Button,
  DataTable,
  Dialog,
  ErrorBanner,
  Field,
  Panel,
  useAction,
} from "../components/ui";
import { PlaylistExport } from "../components/playlist/PlaylistExport";

type Entry = {
  line: number;
  channel: Channel;
  error: string;
  duplicate: string;
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
  const [text, setText] = useState("");
  const [iface, setIface] = useState(
    data.settings.active.network.multicast_interface,
  );
  const [entries, setEntries] = useState<Entry[]>([]);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [update, setUpdate] = useState(false);
  const [confirm, setConfirm] = useState<Channel[] | null>(null);
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
  const canSelect = (entry: Entry) =>
    !entry.error && (entry.duplicate !== "existing" || update);
  return (
    <>
      {mode !== "both" && (
        <nav className="op-playlist-modes" aria-label="Playlist tools">
          <a href="#/import-export" aria-current={mode === "import" ? "page" : undefined}>
            <FileUp size={17} aria-hidden="true" /> Import channels
          </a>
          <a href="#/import-export?mode=export" aria-current={mode === "export" ? "page" : undefined}>
            <Download size={17} aria-hidden="true" /> Export M3U
          </a>
        </nav>
      )}
      {mode !== "import" && <PlaylistExport streams={data.streams} />}
      {(action.error || fileError) && <ErrorBanner>{action.error || fileError}</ErrorBanner>}
      {mode !== "export" && (
        <Panel
          title="Import channels"
          description="Upload or paste an M3U playlist, then preview the channels before saving."
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
