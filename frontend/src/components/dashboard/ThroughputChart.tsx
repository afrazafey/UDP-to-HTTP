import { useState } from "react";
import type { Sample } from "../../types/operations";
import { rate, timestamp } from "../../utils/format";
import { Empty } from "../ui";
export function ThroughputChart({
  samples,
  label = "Network throughput",
}: {
  samples: Sample[];
  label?: string;
}) {
  const [selected, setSelected] = useState<number | null>(null);
  if (samples.length < 2)
    return (
      <Empty title="Collecting throughput history">
        {samples.length
          ? `First sample ${timestamp(samples[0].time)}. A chart appears after the next measured sample.`
          : "No historical measurements are available."}
      </Empty>
    );
  const width = 900,
    height = 190,
    pad = 25;
  const max = Math.max(
    1,
    ...samples.flatMap((s) => [s.input_bitrate, s.output_bitrate]),
  );
  const x = (i: number) => pad + (i / (samples.length - 1)) * (width - pad * 2);
  const y = (v: number) => height - pad - (v / max) * (height - pad * 2);
  const path = (key: "input_bitrate" | "output_bitrate") =>
    samples.map((s, i) => `${i ? "L" : "M"}${x(i)},${y(s[key])}`).join(" ");
  const index = Math.min(selected ?? samples.length - 1, samples.length - 1);
  const point = samples[index];
  return (
    <div className="op-chart">
      <div className="op-chart-legend">
        <span className="input">UDP input</span>
        <span className="output">HTTP output</span>
        <small>Scale {rate(max)}</small>
      </div>
      <svg
        viewBox={`0 0 ${width} ${height}`}
        role="img"
        aria-label={label}
        onPointerMove={(event) => {
          const bounds = event.currentTarget.getBoundingClientRect();
          setSelected(
            Math.max(
              0,
              Math.min(
                samples.length - 1,
                Math.round(
                  ((event.clientX - bounds.left) / bounds.width) *
                    (samples.length - 1),
                ),
              ),
            ),
          );
        }}
        onPointerLeave={() => setSelected(null)}
      >
        <title>
          {label}: {samples.length} measured samples
        </title>
        {[0, 0.5, 1].map((t) => (
          <line
            key={t}
            x1={pad}
            x2={width - pad}
            y1={y(max * t)}
            y2={y(max * t)}
            className="grid"
          />
        ))}
        <path d={path("input_bitrate")} className="input" />
        <path d={path("output_bitrate")} className="output" />
        <line
          x1={x(index)}
          x2={x(index)}
          y1={pad}
          y2={height - pad}
          className="crosshair"
        />
        <circle
          cx={x(index)}
          cy={y(point.input_bitrate)}
          r={4}
          className="input-dot"
        />
        <circle
          cx={x(index)}
          cy={y(point.output_bitrate)}
          r={4}
          className="output-dot"
        />
      </svg>
      <div className="op-chart-reading">
        <time>{timestamp(point.time)}</time>
        <span>Input {rate(point.input_bitrate)}</span>
        <span>Output {rate(point.output_bitrate)}</span>
      </div>
      <label className="op-chart-scrubber">
        Inspect historical sample
        <input
          type="range"
          min={0}
          max={samples.length - 1}
          value={index}
          onChange={(e) => setSelected(Number(e.target.value))}
        />
      </label>
    </div>
  );
}
