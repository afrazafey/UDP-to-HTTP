import type { Configuration, NetworkInterface, Status } from "../api";
import type { StreamSnapshot } from "../Streaming";
export type { NetworkInterface, Status };
export type Stream = StreamSnapshot & { started_at?: string | null };
export interface Client {
  id: number;
  channel_id: string;
  remote_ip: string;
  connected_at: string;
  queued_bytes: number;
  closing: boolean;
  duration_seconds: number;
  bytes_delivered: number;
  output_bitrate: number;
}
export interface Preferences {
  name: string;
  theme: "light" | "dark" | "system";
  refresh_seconds: number;
  default_startup: "on_demand" | "always";
  default_idle_seconds: number;
}
export interface Settings {
  dashboard: Preferences;
  network: Configuration["network"];
  streaming: Configuration["streaming"] & {
    enabled: boolean;
    max_streams: number;
    client_buffer_bytes: number;
    max_buffered_bytes: number;
    inactivity_timeout: string;
    client_write_timeout: string;
    blocked_multicast_networks: string[];
    allow_loopback_unicast: boolean;
  };
  logging: { level: string; retention_count: number; retention_days: number };
  timeouts: {
    read_timeout: string;
    write_timeout: string;
    idle_timeout: string;
    shutdown_timeout: string;
  };
}
export interface SettingsView {
  settings: Settings;
  active: Settings;
  revision: number;
  accepted_revision: number;
  restart_required: boolean;
  last_error: string;
  read_only: Record<string, string | string[]>;
}
export interface Sample {
  time: string;
  input_bitrate: number;
  output_bitrate: number;
  heap_bytes: number;
  runtime_bytes: number;
  rss_bytes: number | null;
  cpu_percent: number | null;
  goroutines: number;
  receivers: number;
  clients: number;
}
export interface Summary {
  recovering?: number;
  offline?: number;
  receiver_restarts?: number;
  total: number;
  running: number;
  stopped: number;
  unhealthy: number;
  clients: number;
  input_bitrate: number;
  output_bitrate: number;
  continuity_errors: number;
  discontinuities: number;
  recent_errors: number;
  inactive: number;
  active_receivers: number;
}
export interface LogEntry {
  id: number;
  time: string;
  level: string;
  component: string;
  stream_id: string;
  message: string;
  context: Record<string, unknown>;
}
export interface Dashboard {
  summary: Summary | null;
  sample: Sample;
  history: Sample[];
  activity: LogEntry[];
  database_ready: boolean;
  version: string;
  uptime_seconds: number;
  sample_interval_seconds: number;
  history_limit: number;
}
export interface SystemInfo {
  database_schema?: number | null;
  version: string;
  commit: string;
  build_date: string;
  go_version: string;
  os: string;
  architecture: string;
  process_id: number;
  started_at: string;
  uptime_seconds: number;
  goroutines: number;
  resources: Sample;
  interfaces: NetworkInterface[];
  configuration_path: string;
  database_ready: boolean;
  http_listeners: {
    kind: string;
    address: string;
    state: string;
    error?: string;
  }[];
  dropped_log_events: number;
  management_policy: string;
  remote_authentication: string;
  trusted_proxies: string[];
  linux_acceptance: string;
}
export interface OperationsData {
  dashboard: Dashboard;
  streams: Stream[];
  clients: Client[];
  interfaces: NetworkInterface[];
  settings: SettingsView;
  system: SystemInfo;
}
