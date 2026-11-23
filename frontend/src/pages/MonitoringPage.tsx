import type { OperationsData } from "../types/operations";
import { bytes, rate, timestamp } from "../utils/format";
import { Definition, Panel, Stat } from "../components/ui";
import { ThroughputChart } from "../components/dashboard/ThroughputChart";
export function MonitoringPage({ data }: { data: OperationsData }) {
  const sample = data.dashboard.sample;
  const sum = data.dashboard.summary;
  return (
    <>
      <div className="op-stats-grid">
        <Stat label="UDP input" value={sum ? rate(sum.input_bitrate) : "—"} />
        <Stat
          label="HTTP output"
          value={sum ? rate(sum.output_bitrate) : "—"}
        />
        <Stat label="Active receivers" value={sum?.active_receivers} />
        <Stat label="Active HTTP clients" value={sum?.clients} />
        <Stat label="Go heap allocation" value={bytes(sample.heap_bytes)} />
        <Stat label="Process resident memory" value={bytes(sample.rss_bytes)} />
        <Stat
          label="Process CPU"
          value={
            sample.cpu_percent == null
              ? "Unavailable"
              : `${sample.cpu_percent.toFixed(1)}%`
          }
          detail="100% represents one CPU core"
        />
        <Stat label="Goroutines" value={sample.goroutines} />
      </div>
      <Panel
        title="Measured throughput history"
        description={`Sampled every ${data.dashboard.sample_interval_seconds}s; retained in memory up to ${data.dashboard.history_limit} points.`}
      >
        <ThroughputChart samples={sum ? data.dashboard.history : []} />
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
              Process CPU and resident memory are supported on Linux. Other
              platforms report unavailable. Go heap and goroutine counts come
              from the actual runtime.
            </p>
            <p>
              Continuity errors indicate transport discontinuity; they are not
              an exact network packet-loss count. Pause stops dashboard polling
              while receivers continue running.
            </p>
          </div>
        </Panel>
      </div>
    </>
  );
}
