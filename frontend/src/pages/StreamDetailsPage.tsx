import { useState } from "react";
import type { OperationsData, Sample } from "../types/operations";
import { api } from "../services/api";
import { usePolling } from "../hooks/usePolling";
import { bytes, duration, playbackURL, rate, timestamp } from "../utils/format";
import {
  Badge,
  CopyButton,
  Definition,
  Empty,
  ErrorBanner,
  Panel,
  Stat,
  useAction,
} from "../components/ui";
import { StreamControls } from "../components/streams/StreamControls";
import { ChannelDialog } from "../components/streams/ChannelDialog";
import { ThroughputChart } from "../components/dashboard/ThroughputChart";
export function StreamDetailsPage({
  id,
  data,
  refresh,
  interval,
  paused,
}: {
  id: string;
  data: OperationsData;
  refresh: () => void;
  interval: number;
  paused: boolean;
}) {
  const stream = data.streams.find((s) => s.channel.id === id);
  const poll = usePolling(
    (signal) =>
      api<{ history: Sample[] }>(
        `/api/v1/streams/${encodeURIComponent(id)}/metrics`,
        {},
        signal,
      ),
    id,
    interval,
    paused,
  );
  const [editing, setEditing] = useState(false);
  const action = useAction();
  if (!stream)
    return (
      <Panel>
        <Empty title="Channel not found">
          The channel may have been deleted.{" "}
          <a href="#/streams">Return to streams</a>.
        </Empty>
      </Panel>
    );
  const c = stream.channel;
  const url = playbackURL(
    stream,
    data.settings.active,
    data.interfaces,
    String(data.settings.read_only.advertised_playback_host ?? ""),
  );
  const nativeSupport = document
    .createElement("video")
    .canPlayType("video/mp2t");
  return (
    <>
      <Panel
        title={c.name}
        description={`${c.id} • ${c.source_type} • ${c.source_address}:${c.source_port}`}
        action={<Badge state={stream.health ?? stream.state} />}
      >
        <div className="op-prose">
          <StreamControls
            stream={stream}
            data={data}
            refresh={refresh}
            edit={() => setEditing(true)}
            action={action}
          />
        </div>
      </Panel>
      {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
      <div className="op-stats-grid">
        <Stat label="Connected clients" value={stream.clients} />
        <Stat label="Input bitrate" value={rate(stream.input_bitrate)} />
        <Stat label="Output bitrate" value={rate(stream.output_bitrate)} />
        <Stat label="Stream uptime" value={duration(stream.uptime_seconds)} />
      </div>
      {poll.error && (
        <ErrorBanner>
          Stream history is unavailable or stale: {poll.error}
        </ErrorBanner>
      )}
      <Panel
        title="Channel throughput history"
        description="Measured samples for this configured channel; history begins when the daemon observes it."
      >
        <ThroughputChart
          samples={poll.data?.history ?? []}
          label={`${c.name} throughput`}
        />
      </Panel>
      <div className="op-two-column">
        <Panel title="Source & playback">
          <Definition
            items={[
              ["Channel ID", c.id],
              ["Channel name", c.name],
              ["Group", c.group || "Ungrouped"],
              ["TVG ID", c.tvg_id || "Unconfigured"],
              ["TVG name", c.tvg_name || "Unconfigured"],
              ["Description", c.description || "None"],
              ["UDP source", `${c.source_address}:${c.source_port}`],
              ["Source type", c.source_type],
              ["Receiving interface", c.interface],
              ["HTTP playback URL", <code>{url}</code>],
              [
                "Receiver started",
                stream.started_at
                  ? timestamp(stream.started_at)
                  : "No active receiver",
              ],
              ["Last valid packet", timestamp(stream.last_packet_at)],
              ["Last UDP datagram", timestamp(stream.last_datagram_at)],
              ["Last successful recovery", timestamp(stream.last_recovery_at)],
              [
                "Socket bound",
                stream.socket_bound == null
                  ? "Unavailable"
                  : String(stream.socket_bound),
              ],
              [
                "Multicast joined",
                stream.membership_joined == null
                  ? "Unavailable"
                  : String(stream.membership_joined),
              ],
              ["Enabled", c.enabled ? "Yes" : "No"],
              ["Startup", c.startup],
            ]}
          />
        </Panel>
        <Panel title="Transport health">
          <Definition
            items={[
              ["Received payload bytes", bytes(stream.counters.input_bytes)],
              ["HTTP payload bytes", bytes(stream.counters.output_bytes)],
              ["MPEG-TS packets", stream.counters.packets],
              ["UDP datagrams", stream.counters.datagrams ?? "Unavailable"],
              ["TS sync losses", stream.counters.sync_errors ?? "Unavailable"],
              [
                "Receiver restarts",
                stream.counters.receiver_restarts ?? "Unavailable",
              ],
              [
                "Successful recoveries",
                stream.counters.recoveries ?? "Unavailable",
              ],
              ["Corrupt observations", stream.counters.corrupt_packets],
              ["Discarded bytes", stream.counters.discarded_bytes],
              ["Continuity errors", stream.counters.continuity_errors],
              ["Explicit discontinuities", stream.counters.discontinuities],
              ["Duplicate packets", stream.counters.duplicate_packets],
              ["Receiver retries", stream.counters.retries],
              [
                "Slow-client disconnects",
                stream.counters.slow_client_disconnects,
              ],
            ]}
          />
          <div className="op-prose">
            <p>
              Continuity errors and discontinuities are observed indicators, not
              an exact network packet-loss measurement.
            </p>
          </div>
        </Panel>
      </div>
      <Panel title="Stream preview / external player">
        <div className="op-prose">
          <p>
            {nativeSupport
              ? `This browser reports “${nativeSupport}” MPEG-TS MIME support, but source codecs and direct playback have not been validated.`
              : "This browser does not report native MPEG-TS playback support."}{" "}
            No transcoding or automatic playback is introduced.
          </p>
          <p>
            Open the HTTP URL in VLC or another compatible external player.
            Playback access still follows the server client CIDR policy.
          </p>
          <CopyButton value={url} label="Copy VLC URL" />
        </div>
      </Panel>
      <Panel
        title="Recent channel events"
        action={
          <a className="op-link" href={`#/logs?stream_id=${id}`}>
            View channel logs
          </a>
        }
      >
        {stream.errors.length ? (
          <ul className="op-event-list">
            {stream.errors
              .slice()
              .reverse()
              .map((e, i) => (
                <li key={`${e.time}-${i}`}>
                  <time>{timestamp(e.time)}</time>
                  <strong>{e.operation}</strong>
                  <p>{e.message}</p>
                </li>
              ))}
          </ul>
        ) : (
          <Empty title="No retained receiver errors" />
        )}
      </Panel>
      <Panel
        title="Connected viewers"
        action={
          <a href="#/clients" className="op-link">
            Manage clients
          </a>
        }
      >
        <Definition
          items={data.clients
            .filter((client) => client.channel_id === id)
            .map((client) => [
              `${client.remote_ip} · #${client.id}`,
              `${duration(client.duration_seconds)} · ${bytes(client.bytes_delivered)} · ${rate(client.output_bitrate)}`,
            ])}
        />
        {!data.clients.some((client) => client.channel_id === id) && (
          <Empty title="No connected viewers" />
        )}
      </Panel>
      <Panel title="Recovery history">
        <Definition
          items={[
            ["Last successful recovery", timestamp(stream.last_recovery_at)],
            [
              "Recovery failures",
              stream.counters.recovery_failures ?? "Unavailable",
            ],
            [
              "Receiver restarts",
              stream.counters.receiver_restarts ?? "Unavailable",
            ],
          ]}
        />
        <div className="op-prose">
          <a href={`#/logs?stream_id=${id}&component=receiver`}>
            View receiver events
          </a>
        </div>
      </Panel>
      {editing && (
        <ChannelDialog
          data={data}
          stream={stream}
          onClose={() => setEditing(false)}
          refresh={refresh}
        />
      )}
    </>
  );
}
