import { useState } from "react";
import { ArrowDownToLine, ArrowUpFromLine, Cpu, HardDrive, Layers, Radio, Users, Workflow } from "lucide-react";
import type { OperationsData, Sample } from "../types/operations";
import { bytes, rate, timestamp } from "../utils/format";
import {
  Badge,
  DataTable,
  Definition,
  ErrorBanner,
  Empty,
  Field,
  Panel,
  Stat,
} from "../components/ui";
import { ThroughputChart } from "../components/dashboard/ThroughputChart";
import { usePolling } from "../hooks/usePolling";
import { api } from "../services/api";
export function MonitoringPage({
  data,
  paused = false,
  interval = 5,
}: {
  data: OperationsData;
  paused?: boolean;
  interval?: number;
}) {
  const sample = data.dashboard.sample;
  const sum = data.dashboard.summary;
  const [channel, setChannel] = useState("");
  const history = usePolling(
    (signal) =>
      channel
        ? api<{ history: Sample[] }>(
          `/api/v1/streams/${encodeURIComponent(channel)}/metrics`,
          {},
          signal,
        )
        : Promise.resolve({ history: [] }),
    channel,
    interval,
    paused,
  );
  return (
    <>
      <div className="op-monitoring-layout">
        <Panel
          className="op-monitoring-throughput"
          title="Measured throughput history"
          description={`Sampled every ${data.dashboard.sample_interval_seconds}s · up to ${data.dashboard.history_limit} points retained`}
        >
          <ThroughputChart samples={sum ? data.dashboard.history : []} />
        </Panel>
        <div className="op-page-metrics op-monitoring-metrics" aria-label="Current measurements">
          <Stat label="UDP input" value={sum ? rate(sum.input_bitrate) : "—"} icon={<ArrowDownToLine size={17} />} />
          <Stat
            label="HTTP output"
            value={sum ? rate(sum.output_bitrate) : "—"}
            icon={<ArrowUpFromLine size={17} />}
          />
          <Stat label="Active receivers" value={sum?.active_receivers} icon={<Radio size={17} />} />
          <Stat label="Active HTTP clients" value={sum?.clients} icon={<Users size={17} />} />
          <Stat label="Go heap allocation" value={bytes(sample.heap_bytes)} icon={<Layers size={17} />} />
          <Stat label="Process resident memory" value={bytes(sample.rss_bytes)} icon={<HardDrive size={17} />} />
          <Stat
            label="Process CPU"
            value={
              sample.cpu_percent == null
                ? "Unavailable"
                : `${sample.cpu_percent.toFixed(1)}%`
            }
            detail="100% represents one CPU core"
            icon={<Cpu size={17} />}
          />
          <Stat label="Goroutines" value={sample.goroutines} icon={<Workflow size={17} />} />
        </div>
      </div>
      <Panel
        title="Per-channel throughput"
        action={
          <Field label="Channel history">
            <select
              value={channel}
              onChange={(e) => setChannel(e.target.value)}
            >
              <option value="">Select a channel</option>
              {data.streams.map((s) => (
                <option key={s.channel.id} value={s.channel.id}>
                  {s.channel.name}
                </option>
              ))}
            </select>
          </Field>
        }
      >
        {history.error && <ErrorBanner>{history.error}</ErrorBanner>}
        {channel && (
          <ThroughputChart
            key={channel}
            samples={history.data?.history ?? []}
            label="Channel throughput"
          />
        )}
        {data.streams.length ? <DataTable>
          <thead role="rowgroup">
            <tr role="row">
              <th role="columnheader" scope="col">Channel</th>
              <th role="columnheader" scope="col">Receiver</th>
              <th role="columnheader" scope="col">Viewers</th>
              <th role="columnheader" scope="col">UDP / HTTP</th>
              <th role="columnheader" scope="col">Restarts</th>
              <th role="columnheader" scope="col">TS sync losses</th>
            </tr>
          </thead>
          <tbody role="rowgroup">
            {data.streams.map((s) => (
              <tr role="row" key={s.channel.id}>
                <td role="cell" data-label="Channel" data-primary>
                  <a href={`#/streams/${s.channel.id}`}>{s.channel.name}</a>
                </td>
                <td role="cell" data-label="Receiver">
                  <Badge state={s.health ?? s.state} />
                </td>
                <td role="cell" data-label="Viewers">{s.clients}</td>
                <td role="cell" data-label="UDP / HTTP">
                  {rate(s.input_bitrate)} / {rate(s.output_bitrate)}
                </td>
                <td role="cell" data-label="Restarts">{s.counters.receiver_restarts ?? "Unavailable"}</td>
                <td role="cell" data-label="TS sync losses">{s.counters.sync_errors ?? "Unavailable"}</td>
              </tr>
            ))}
          </tbody>
        </DataTable> : <Empty title="No channels to monitor" />}
      </Panel>
      <div className="op-two-column">
        <Panel title="Transport observations">
          <Definition
            items={[
              [
                "Continuity counter errors",
                sum?.continuity_errors ?? "Unavailable",
              ],
              [
                "Explicit discontinuities",
                sum?.discontinuities ?? "Unavailable",
              ],
              ["Streams waiting for packets", sum?.inactive ?? "Unavailable"],
              ["Unhealthy receivers", sum?.unhealthy ?? "Unavailable"],
              ["Retained stream events", sum?.recent_errors ?? "Unavailable"],
              ["Sample time", timestamp(sample.time)],
            ]}
          />
        </Panel>
        <Panel title="Measurement scope">
          <div className="op-prose">
            <p>
              Rates measure accepted UDP payloads and payload bytes accepted by
              HTTP writes. They do not measure player acknowledgments.
            </p>
            <p>
              Process CPU and resident memory are sampled on Linux and Windows.
              Go heap and goroutine counts come from the actual runtime.
            </p>
            <p>
              Continuity errors indicate transport discontinuity; they are not
              an exact network packet-loss count. Pause stops dashboard polling
              while receivers continue running.
            </p>
          </div>
        </Panel>
      </div>
      <Panel title="Network interface errors">
        <Definition
          items={data.interfaces.map((i) => [
            i.name,
            i.error_counters
              ? `RX ${i.error_counters.rx_errors} errors / ${i.error_counters.rx_dropped} drops · TX ${i.error_counters.tx_errors} errors / ${i.error_counters.tx_dropped} drops`
              : "Unavailable",
          ])}
        />
      </Panel>
    </>
  );
}
