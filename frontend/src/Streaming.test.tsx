import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { StreamingWorkspace, type StreamSnapshot } from "./Streaming";
import type { Configuration } from "./api";

const configuration: Configuration = {
  management: {
    host: "127.0.0.1",
    port: 8080,
    read_timeout: "10s",
    write_timeout: "15s",
    idle_timeout: "60s",
    shutdown_timeout: "10s",
  },
  network: {
    multicast_interface: "eth-test",
    http_interface: "127.0.0.1",
    http_port: 7781,
    secondary_http_port: 0,
  },
  streaming: {
    enabled: true,
    udp_receive_buffer: 4194304,
    stream_buffer: 8388608,
    max_clients: 100,
    connect_timeout: "10s",
    allowed_client_networks: ["127.0.0.0/8"],
    allowed_multicast_groups: ["239.1.1.1"],
    allowed_unicast_sources: [],
  },
  storage: { path: "test.db" },
  logging: { level: "info" },
};
const snapshot: StreamSnapshot = {
  channel: {
    id: "news",
    name: "News",
    source_type: "multicast",
    source_address: "239.1.1.1",
    source_port: 1234,
    interface: "eth-test",
    enabled: true,
    startup: "on_demand",
    idle_timeout_seconds: 2,
    max_clients: 10,
    client_buffer_bytes: 262144,
    udp_receive_buffer: 4194304,
  },
  state: "waiting",
  clients: 2,
  uptime_seconds: 12,
  last_packet_at: null,
  input_bitrate: 1200000,
  output_bitrate: 2400000,
  counters: {
    input_bytes: 150000,
    output_bytes: 300000,
    packets: 700,
    corrupt_packets: 3,
    discarded_bytes: 188,
    continuity_errors: 2,
    discontinuities: 1,
    duplicate_packets: 0,
    slow_client_disconnects: 1,
    retries: 0,
    rejected_datagrams: 0,
  },
  errors: [
    {
      time: "2026-11-23T00:00:00Z",
      operation: "inactivity",
      message: "No valid packets received",
    },
  ],
};
function mockAPI(failActions = false) {
  const mock = vi.fn(async (path: string, options?: RequestInit) => {
    const mutation = !!options?.method && options.method !== "GET";
    const body = path.endsWith("csrf")
      ? { token: "local-token" }
      : mutation && failActions
        ? { error: "Source is not allowed" }
        : path.endsWith("clients")
          ? { clients: [] }
          : { streams: [snapshot] };
    return new Response(JSON.stringify(body), {
      status: mutation && failActions ? 400 : 200,
      headers: { "Content-Type": "application/json" },
    });
  });
  vi.stubGlobal("fetch", mock);
  return mock;
}
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
describe("stream management", () => {
  it("shows actual state, counts, bitrate, and recent errors", async () => {
    mockAPI();
    render(
      <StreamingWorkspace
        view="overview"
        configuration={configuration}
        interfaces={[]}
      />,
    );
    expect(await screen.findByText("News")).toBeInTheDocument();
    expect(screen.getByText("waiting")).toBeInTheDocument();
    expect(screen.getAllByText("1.20 Mbps").length).toBeGreaterThan(0);
    expect(screen.getByText("No valid packets received")).toBeInTheDocument();
    expect(screen.queryByText("running")).not.toBeInTheDocument();
  });
  it("sends CSRF-protected start, stop and restart commands", async () => {
    const fetch = mockAPI();
    render(
      <StreamingWorkspace
        view="streams"
        configuration={configuration}
        interfaces={[]}
      />,
    );
    await screen.findByText("News");
    for (const action of ["Start", "Stop", "Restart"]) {
      fireEvent.click(screen.getByRole("button", { name: `${action} News` }));
      await waitFor(() =>
        expect(fetch).toHaveBeenCalledWith(
          `/api/v1/streams/news/${action.toLowerCase()}`,
          expect.objectContaining({
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              "X-CSRF-Token": "local-token",
            },
          }),
        ),
      );
      await waitFor(() =>
        expect(
          screen.getByRole("button", { name: "Start News" }),
        ).toBeEnabled(),
      );
    }
  });
  it("creates and edits channel definitions through the real API contract", async () => {
    const fetch = mockAPI();
    render(
      <StreamingWorkspace
        view="streams"
        configuration={configuration}
        interfaces={[]}
      />,
    );
    await screen.findByText("News");
    fireEvent.click(screen.getByRole("button", { name: "Edit News" }));
    expect(screen.getByLabelText("Channel ID")).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Channel name"), {
      target: { value: "Updated News" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save channel" }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/api/v1/streams/news",
        expect.objectContaining({
          method: "PUT",
          body: expect.stringContaining('"name":"Updated News"'),
        }),
      ),
    );
    await waitFor(() =>
      expect(screen.queryByLabelText("Channel ID")).not.toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Add channel" }));
    fireEvent.change(screen.getByLabelText("Channel ID"), {
      target: { value: "new" },
    });
    fireEvent.change(screen.getByLabelText("Channel name"), {
      target: { value: "New stream" },
    });
    fireEvent.change(screen.getByLabelText("Multicast group IPv4"), {
      target: { value: "239.1.1.1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save channel" }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/api/v1/streams",
        expect.objectContaining({
          method: "POST",
          body: expect.stringContaining('"id":"new"'),
        }),
      ),
    );
  });
  it("surfaces rejected operations without fabricating success", async () => {
    mockAPI(true);
    render(
      <StreamingWorkspace
        view="streams"
        configuration={configuration}
        interfaces={[]}
      />,
    );
    await screen.findByText("News");
    fireEvent.click(screen.getByRole("button", { name: "Start News" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Source is not allowed",
    );
  });
  it("copies the configured playback URL", async () => {
    mockAPI();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    render(
      <StreamingWorkspace
        view="streams"
        configuration={configuration}
        interfaces={[]}
      />,
    );
    await screen.findByText("News");
    fireEvent.click(screen.getByRole("button", { name: "Copy URL for News" }));
    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith(
        "http://127.0.0.1:7781/live/news.ts",
      ),
    );
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Copied playback URL",
    );
  });
  it("deletes only after the explicit channel confirmation", async () => {
    const fetch = mockAPI();
    render(
      <StreamingWorkspace
        view="streams"
        configuration={configuration}
        interfaces={[]}
      />,
    );
    await screen.findByText("News");
    fireEvent.click(screen.getByRole("button", { name: "Delete News" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete channel" }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/api/v1/streams/news",
        expect.objectContaining({ method: "DELETE" }),
      ),
    );
  });
});
