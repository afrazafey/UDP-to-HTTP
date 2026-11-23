import { useEffect, useState } from "react";
import type { Channel } from "../Streaming";
import type { OperationsData } from "../types/operations";
import { api, download, mutate } from "../services/api";
import {
  Button,
  CopyButton,
  Dialog,
  ErrorBanner,
  Field,
  Panel,
  useAction,
} from "../components/ui";

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
}: {
  data: OperationsData;
  refresh: () => void;
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
  useEffect(() => {
    const controller = new AbortController();
    void api<PlaylistConfig>("/api/v1/playlist/config", {}, controller.signal)
      .then(setConfig)
      .catch((e) => {
        if (!controller.signal.aborted) setConfigError(String(e.message));
      });
    return () => controller.abort();
  }, []);
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
  return (
    <>
      {(action.error || configError || fileError) && (
        <ErrorBanner>{action.error || configError || fileError}</ErrorBanner>
      )}
      <Panel
        title="Playlist export"
        description="Enabled channels only. Advertised host is configured in YAML independently of listener binding."
      >
        <p>
          Advertised host:{" "}
          <strong>
            {config?.playlist.advertised_host ||
              "Not configured — set playlist.advertised_host"}
          </strong>
          .{" "}
          {config?.playlist.remote_enabled
            ? "Remote download uses both playlist and playback IP policies."
            : "Remote downloads are disabled; this URL requires management access."}
        </p>
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
            disabled={action.busy || !config?.playlist.advertised_host}
            onClick={() =>
              void action.run(
                () => download(exportPath, "udphttp-channels.m3u"),
                "Playlist downloaded",
              )
            }
          >
            Download M3U
          </Button>
          {config?.playlist.advertised_host && (
            <CopyButton value={playlistURL} label="Copy playlist URL" />
          )}
        </div>
      </Panel>
      <Panel
        title="Import channels"
        description="Preview before saving. Imports never authorize sources or fetch playlist/logo URLs."
      >
        <div className="op-form-grid">
          <Field label="M3U or M3U8 file">
            <input
              type="file"
              accept=".m3u,.m3u8"
              onChange={async (e) => {
                const file = e.target.files?.[0];
                if (!file) return;
                setFileError("");
                if (file.size > 1048576 || !/\.m3u8?$/i.test(file.name)) {
                  setFileError("Choose an M3U/M3U8 file up to 1 MiB");
                  return;
                }
                try {
                  setText(await file.text());
                  setEntries([]);
                } catch (error) {
                  setFileError(String(error));
                }
              }}
            />
          </Field>
          <Field label="Import UDP interface">
            <select
              value={iface}
              onChange={(e) => {
                setIface(e.target.value);
                setEntries([]);
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
              setSelected(new Set());
            }}
            placeholder={
              '#EXTM3U\n#EXTINF:-1 group-title="MUSIC",Channel name\nudp://@239.1.1.61:1234'
            }
          />
        </Field>
        <div className="op-toolbar">
          <Button
            disabled={action.busy || !text || !iface}
            onClick={() =>
              void action.run(async () => {
                const result = await mutate<{ entries: Entry[] }>(
                  "/api/v1/import/preview",
                  "POST",
                  { text, interface: iface },
                );
                setEntries(result.entries);
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
            Preview import
          </Button>
          <label>
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
            <div className="op-table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Select</th>
                    <th>Channel</th>
                    <th>Source / port</th>
                    <th>Type / interface</th>
                    <th>Group</th>
                    <th>Duplicate</th>
                    <th>Validation</th>
                  </tr>
                </thead>
                <tbody>
                  {entries.map((row, i) => (
                    <tr key={`${row.line}-${i}`}>
                      <td>
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
                      <td>{row.channel.name}</td>
                      <td>
                        {row.channel.source_address}:{row.channel.source_port}
                      </td>
                      <td>
                        {row.channel.source_type}
                        <small>{row.channel.interface}</small>
                      </td>
                      <td>{row.channel.group || "—"}</td>
                      <td>{row.duplicate}</td>
                      <td>{row.error || "Valid against current policy"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
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
            <p>
              Rejected source addresses require a separate, explicit allowlist
              review in Security settings. Preview again after the authorized
              policy becomes active.
            </p>
          </>
        )}
      </Panel>
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
                    await mutate("/api/v1/import/commit", "POST", {
                      channels: confirm,
                      update_existing: update,
                      confirm: true,
                    });
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
