import { useState } from "react";
import type { Sample } from "../../types/operations";
import { rate, timestamp } from "../../utils/format";
import { Empty } from "../ui";

const ranges = [
  [30, "30 seconds"],
  [60, "1 minute"],
  [300, "5 minutes"],
  [900, "15 minutes"],
] as const;

function axisRate(value: number) {
  if (value === 0) return "0";
  const [scale, unit] = value >= 1e6 ? [1e6, "Mbps"] : [1000, "Kbps"];
  return `${Number((value / scale).toFixed(2))} ${unit}`;
}

function clockTime(time: Date) {
  return time.toLocaleTimeString([], {
    hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false,
  });
}

export function ThroughputChart({
  samples,
  label = "Network throughput",
  heading,
}: {
  samples: Sample[];
  label?: string;
  heading?: { title: string; description: string };
}) {
  const [selected, setSelected] = useState<number | null>(null);
  const [range, setRange] = useState(30);
  const lastTime = Date.parse(samples.at(-1)?.time ?? "");
  const historySeconds = (lastTime - Date.parse(samples[0]?.time ?? "")) / 1000;
  const available = ranges.filter(([seconds]) => historySeconds >= seconds);
  const activeRange = available.some(([seconds]) => seconds === range) ? range : 0;
  if (activeRange) {
    const recent = samples.filter(
      (s) => Date.parse(s.time) >= lastTime - activeRange * 1000,
    );
    if (recent.length >= 2) samples = recent;
  }
  const title = heading && (
    <div>
      <h2>{heading.title}</h2>
      <p>{heading.description}</p>
    </div>
  );
  if (samples.length < 2)
    return (
      <div className="op-chart">
        {title && <div className="op-chart-header">{title}</div>}
        <Empty title="Collecting throughput history">
          {samples.length
            ? `First sample ${timestamp(samples[0].time)}. A chart appears after the next measured sample.`
            : "No historical measurements are available."}
        </Empty>
      </div>
    );
  const width = 600;
  const height = 128;
  const peak = Math.max(
    1000,
    ...samples.flatMap((s) => [s.input_bitrate, s.output_bitrate]),
  );
  const step = 10 ** Math.floor(Math.log10(peak / 5));
  const tickStep = ([1, 2, 2.5, 5, 10].find(value => value * step >= peak / 5) ?? 10) * step;
  const segments = Math.ceil(peak / tickStep);
  const max = tickStep * segments;
  const start = Date.parse(samples[0].time);
  const span = lastTime - start;
  const x = (i: number) =>
    (span > 0
      ? (Date.parse(samples[i].time) - start) / span
      : i / (samples.length - 1)) *
      width;
  const y = (v: number) => height - (v / max) * height;
  const path = (key: "input_bitrate" | "output_bitrate") =>
    samples.map((s, i) => `${i ? "L" : "M"}${x(i)},${y(s[key])}`).join(" ");
  const index = Math.min(selected ?? samples.length - 1, samples.length - 1);
  const point = samples[index];
  const tickTimes = Array.from({ length: 7 }, (_, i) => new Date(start + span * i / 6));
  const ticks = Array.from({ length: segments + 1 }, (_, i) => max - tickStep * i);
  const inputPath = path("input_bitrate");
  return (
    <div className="op-chart">
      <div className="op-chart-header">
        {title}
        <div className="op-chart-ranges" role="group" aria-label="Throughput time range">
          {([...available, [0, "Available history"]] as const).map(([seconds, name]) => (
            <button
              key={seconds}
              type="button"
              aria-pressed={activeRange === seconds}
              aria-label={name}
              onClick={() => {
                setRange(seconds);
                setSelected(null);
              }}
            >
              {seconds === 0 ? "All" : seconds < 60 ? `${seconds}s` : `${seconds / 60}m`}
            </button>
          ))}
        </div>
      </div>
      <div className="op-chart-grid">
        <div className="op-chart-y" aria-hidden="true">
          {ticks.map(value => <span key={value}>{axisRate(value)}</span>)}
        </div>
        <svg
          viewBox={`0 0 ${width} ${height}`}
          preserveAspectRatio="none"
          role="img"
          aria-label={label}
          onPointerMove={(event) => {
            const bounds = event.currentTarget.getBoundingClientRect();
            if (!bounds.width) return;
            const pointer = ((event.clientX - bounds.left) / bounds.width) * width;
            const closest = samples.reduce(
              (best, _, i) =>
                Math.abs(x(i) - pointer) < Math.abs(x(best) - pointer) ? i : best,
              0,
            );
            setSelected(closest);
          }}
          onPointerLeave={() => setSelected(null)}
        >
          <title>{label}: {samples.length} measured samples</title>
          {tickTimes.map((_, i) => {
            const column = width * i / (tickTimes.length - 1);
            return <line key={i} x1={column} x2={column} y1={0} y2={height} className="grid" />;
          })}
          {ticks.map(value => (
            <line key={value} x1={0} x2={width} y1={y(value)} y2={y(value)} className="grid" />
          ))}
          <path d={`${inputPath}L${width},${height}L0,${height}Z`} className="op-chart-area" />
          <path d={path("output_bitrate")} className="output" />
          <path d={inputPath} className="input" />
          {selected != null && (
            <>
              <line x1={x(index)} x2={x(index)} y1={0} y2={height} className="crosshair" />
              <circle cx={x(index)} cy={y(point.input_bitrate)} r={4} className="input-dot" />
              <circle cx={x(index)} cy={y(point.output_bitrate)} r={4} className="output-dot" />
            </>
          )}
        </svg>
        <div className="op-chart-axis" aria-hidden="true">
          {tickTimes.map((time, i) => <span key={i}>{clockTime(time)}</span>)}
        </div>
      </div>
      <div className="op-chart-legend">
        <span className="input">UDP input <strong>{rate(point.input_bitrate)}</strong></span>
        <span className="output">HTTP output <strong>{rate(point.output_bitrate)}</strong></span>
        {selected != null && <time dateTime={point.time}>{clockTime(new Date(point.time))}</time>}
      </div>
      <details className="op-chart-inspect">
        <summary>Inspect samples</summary>
        <label className="op-chart-scrubber">
          Inspect historical sample
          <input
            type="range"
            min={0}
            max={samples.length - 1}
            value={index}
            aria-valuetext={`${timestamp(point.time)}, input ${rate(point.input_bitrate)}, output ${rate(point.output_bitrate)}`}
            onChange={(e) => setSelected(Number(e.target.value))}
          />
        </label>
      </details>
    </div>
  );
}
