import type { NetworkInterface, Settings, Stream } from "../types/operations";
export const rate = (value: number | null | undefined) =>
  value == null
    ? "Unavailable"
    : value >= 1e6
      ? `${(value / 1e6).toFixed(2)} Mbps`
      : `${(value / 1000).toFixed(1)} Kbps`;
export const bytes = (value: number | null | undefined) =>
  value == null
    ? "Unavailable"
    : value >= 1048576
      ? `${(value / 1048576).toFixed(1)} MiB`
      : `${(value / 1024).toFixed(1)} KiB`;
export const duration = (value: number | null | undefined) =>
  value == null
    ? "—"
    : `${Math.floor(value / 3600)}h ${Math.floor(value / 60) % 60}m ${Math.floor(value) % 60}s`;
export const timestamp = (value: string | null | undefined) =>
  value ? new Date(value).toLocaleString() : "None received";
export function playbackURL(
  stream: Stream,
  settings: Settings,
  interfaces: NetworkInterface[],
) {
  let host =
    settings.network.http_bind_addresses?.[0] ||
    settings.network.http_interface;
  host = interfaces.find((item) => item.name === host)?.ipv4[0] || host;
  if (!host || host === "0.0.0.0") host = window.location.hostname;
  return `http://${host}:${settings.network.http_port}/live/${encodeURIComponent(stream.channel.id)}.ts`;
}
