import { useEffect, useState } from "react";
import { Download } from "lucide-react";
import { APIError, api, download } from "../../services/api";
import type { Stream } from "../../types/operations";
import { ChannelIdentity } from "../streams/ChannelMenu";
import { Button, CopyButton, DataTable, Empty, ErrorBanner, Field, Panel, useAction } from "../ui";

type PlaylistConfig = {
  playlist: { advertised_host: string; remote_enabled: boolean };
  primary_port: number;
  secondary_port: number;
};

export function PlaylistExport({ streams }: { streams: Stream[] }) {
  const action = useAction();
  const [config, setConfig] = useState<PlaylistConfig>();
  const [configError, setConfigError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const [host, setHost] = useState<string>();
  const [group, setGroup] = useState("");
  const [port, setPort] = useState("preferred");

  useEffect(() => {
    const controller = new AbortController();
    void api<PlaylistConfig>("/api/v1/playlist/config", {}, controller.signal)
      .then(setConfig)
      .catch((error: Error) => {
        if (controller.signal.aborted) return;
        setConfigError(error instanceof APIError && error.status === 404
          ? "The running daemon does not support playlist export. Start the current UDPHTTP Proxy backend and retry."
          : error.message);
      });
    return () => controller.abort();
  }, [attempt]);

  const savedHost = config?.playlist.advertised_host ?? "";
  const playbackHost = (host ?? savedHost).trim();
  const hostInvalid = /[/:@?#\s]/.test(playbackHost);
  const override = playbackHost !== savedHost;
  const enabled = streams.filter((stream) => stream.channel.enabled);
  const groups = [...new Set(enabled.map(({ channel }) => channel.group).filter(Boolean))].sort();
  const preview = enabled
    .filter(({ channel }) => !group || channel.group === group)
    .sort((a, b) => (a.channel.sort_order ?? 0) - (b.channel.sort_order ?? 0) ||
      (a.channel.id < b.channel.id ? -1 : a.channel.id > b.channel.id ? 1 : 0));
  const query = new URLSearchParams({
    port,
    ...(group ? { group } : {}),
    ...(override ? { host: playbackHost } : {}),
  }).toString();
  const exportPath = `/api/v1/playlist.m3u?${query}`;
  const remote = config?.playlist.remote_enabled && !override && Boolean(savedHost);
  const unavailablePort = config && !config.secondary_port && (
    port === "secondary" || (port === "preferred" && preview.some(({ channel }) =>
      channel.preferred_playback_port === "secondary"))
  );
  const ready = Boolean(config && playbackHost && !hostInvalid && preview.length && !unavailablePort);
  const playlistURL = remote
    ? `http://${savedHost}:${port === "secondary" ? config?.secondary_port : config?.primary_port}/playlist.m3u?${query}`
    : new URL(exportPath, window.location.origin).toString();

  return (
    <div className="op-playlist-layout">
      <Panel title="Build a playlist" description="Choose where players connect and which channels to include." className="op-playlist-options">
        {configError ? (
          <div className="op-export-builder">
            <ErrorBanner>{configError}</ErrorBanner>
            <Button onClick={() => { setConfigError(""); setAttempt((current) => current + 1); }}>Retry configuration</Button>
          </div>
        ) : !config ? <p className="op-panel-note" role="status">Loading playback configuration…</p> : (
          <form className="op-export-builder" onSubmit={(event) => {
            event.preventDefault();
            if (ready) void action.run(
              () => download(exportPath, "udphttp-channels.m3u", /^(audio\/(x-)?mpegurl|application\/(vnd\.apple\.mpegurl|x-mpegurl))$/i),
              "Playlist downloaded",
            );
          }}>
            <Field label="Playback host" help={savedHost
              ? "Changes here apply to this download only."
              : "Enter the server IP or hostname your player can reach. Leave out the port and scheme."}>
              <input value={host ?? savedHost} spellCheck={false} autoCapitalize="none"
                onChange={(event) => setHost(event.target.value)} placeholder="192.168.1.20"
                aria-invalid={hostInvalid} required />
            </Field>
            {hostInvalid && <p className="op-network-list-error" role="status">Use a hostname or IPv4 address without a scheme, port or path.</p>}
            {override && savedHost && <Button type="button" onClick={() => setHost(undefined)}>Use saved host</Button>}
            <div className="op-export-filters">
              <Field label="Export group">
                <select value={group} onChange={(event) => setGroup(event.target.value)}>
                  <option value="">All groups</option>
                  {group && !groups.includes(group) && <option>{group}</option>}
                  {groups.map((name) => <option key={name}>{name}</option>)}
                </select>
              </Field>
              <Field label="Playback port">
                <select value={port} onChange={(event) => setPort(event.target.value)}>
                  <option value="preferred">Channel preference</option>
                  <option value="primary">Primary · {config.primary_port}</option>
                  <option value="secondary" disabled={!config.secondary_port}>
                    Secondary{config.secondary_port ? ` · ${config.secondary_port}` : " · disabled"}
                  </option>
                </select>
              </Field>
            </div>
            {unavailablePort && <p className="op-network-list-error" role="status">Choose Primary to export channels that prefer the disabled secondary listener.</p>}
            {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
            <Button type="submit" className="primary" disabled={action.busy || !ready}>
              <Download size={16} aria-hidden="true" /> {action.busy ? "Preparing download…" : "Download M3U"}
            </Button>
          </form>
        )}
      </Panel>
      <div className="op-export-preview">
        <Panel title="Playlist preview" description={`${preview.length} of ${enabled.length} enabled channels`}>
          {preview.length ? (
            <DataTable caption="Playlist order for the selected group">
              <thead role="rowgroup">
                <tr role="row">
                  <th role="columnheader" scope="col">Channel</th>
                  <th role="columnheader" scope="col">Group</th>
                  <th role="columnheader" scope="col">Playback port</th>
                </tr>
              </thead>
              <tbody role="rowgroup">
                {preview.map((stream) => {
                  const { channel } = stream;
                  const secondary = port === "secondary" ||
                    (port === "preferred" && channel.preferred_playback_port === "secondary");
                  return (
                    <tr role="row" key={channel.id}>
                      <td role="cell" data-label="Channel" data-primary><ChannelIdentity stream={stream} /></td>
                      <td role="cell" data-label="Group">{channel.group || "Ungrouped"}</td>
                      <td role="cell" data-label="Playback port">
                        {config ? (secondary ? config.secondary_port || "Disabled" : config.primary_port) : "—"}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </DataTable>
          ) : (
            <Empty title={enabled.length ? "No channels in this group" : "No enabled channels"}>
              {enabled.length
                ? <Button onClick={() => setGroup("")}>Show all groups</Button>
                : <a className="op-button" href="#/streams">Manage channels</a>}
            </Empty>
          )}
        </Panel>
        {ready && (
          <Panel title="Playlist link" description={remote
            ? "Download is available to clients allowed by both playback and playlist policies."
            : "This link requires dashboard access. Download the file to use it in a player."}>
            <div className="op-export-link">
              <Field label={remote ? "Remote playlist URL" : "Management download URL"}>
                <input readOnly value={playlistURL} onFocus={(event) => event.target.select()} />
              </Field>
              <CopyButton value={playlistURL} label="Copy playlist URL" />
            </div>
          </Panel>
        )}
      </div>
    </div>
  );
}
