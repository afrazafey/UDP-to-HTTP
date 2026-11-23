export interface Configuration {
  management: {
    host: string;
    port: number;
    read_timeout: string;
    write_timeout: string;
    idle_timeout: string;
    shutdown_timeout: string;
  };
  network: {
    http_bind_addresses?: string[];
    multicast_interface: string;
    http_interface: string;
    http_port: number;
    secondary_http_port: number;
  };
  streaming: {
    enabled?: boolean;
    client_buffer_bytes?: number;
    max_buffered_bytes?: number;
    max_streams?: number;
    inactivity_timeout?: string;
    client_write_timeout?: string;
    blocked_multicast_networks?: string[];
    allow_loopback_unicast?: boolean;
    udp_receive_buffer: number;
    stream_buffer: number;
    max_clients: number;
    connect_timeout: string;
    allowed_client_networks: string[];
    allowed_multicast_groups: string[];
    allowed_unicast_sources: string[];
  };
  storage: { path: string };
  logging: { level: string };
}
export interface Status {
  service: string;
  version: string;
  commit: string;
  build_date: string;
  phase: number;
  status: string;
  started_at: string;
  uptime_seconds: number;
  platform: string;
  go_version: string;
  components: Record<string, string>;
  configuration: Configuration;
}
export interface NetworkInterface {
  index: number;
  name: string;
  mtu: number;
  up: boolean;
  loopback: boolean;
  multicast: boolean;
  ipv4: string[];
}

export async function request<T>(
  path: string,
  signal: AbortSignal,
): Promise<T> {
  const response = await fetch(path, {
    signal,
    headers: { Accept: "application/json" },
  });
  if (!response.ok) throw new Error(`Request failed (${response.status})`);
  if (!response.headers.get("content-type")?.includes("application/json"))
    throw new Error("The management API did not return JSON");
  return response.json() as Promise<T>;
}
