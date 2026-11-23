import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const fixture = {
  service: "udphttp-proxy",
  version: "test-version",
  commit: "test-commit",
  build_date: "unknown",
  phase: 1,
  status: "running",
  uptime_seconds: 125,
  platform: "linux/amd64",
  go_version: "go-test",
  started_at: "2026-01-01T00:00:00Z",
  components: { storage: "ready", streaming: "unavailable" },
  configuration: {
    management: {
      host: "127.0.0.1",
      port: 8080,
      read_timeout: "10s",
      write_timeout: "15s",
      idle_timeout: "1m0s",
      shutdown_timeout: "10s",
    },
    network: {
      multicast_interface: "",
      http_interface: "127.0.0.1",
      http_port: 7781,
      secondary_http_port: 7782,
    },
    streaming: {
      udp_receive_buffer: 4194304,
      stream_buffer: 8388608,
      max_clients: 100,
      connect_timeout: "10s",
      allowed_client_networks: [],
      allowed_multicast_groups: [],
      allowed_unicast_sources: [],
    },
    storage: { path: "data/test.db" },
    logging: { level: "info" },
  },
};
function mockAPI() {
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async (path: string) =>
        new Response(
          JSON.stringify(
            path.includes("interfaces")
              ? {
                  interfaces: [
                    {
                      index: 1,
                      name: "testnic",
                      ipv4: ["192.0.2.5"],
                      up: true,
                      multicast: true,
                      loopback: false,
                      mtu: 1500,
                    },
                  ],
                }
              : fixture,
          ),
          { headers: { "Content-Type": "application/json" } },
        ),
    ),
  );
}
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  localStorage.clear();
});
describe("dashboard", () => {
  it("renders API data and explicitly marks streaming unavailable", async () => {
    mockAPI();
    render(<App />);
    expect(await screen.findByText("Online")).toBeInTheDocument();
    expect(screen.getByText("2m 5s")).toBeInTheDocument();
    expect(screen.getByText("testnic")).toBeInTheDocument();
    expect(
      screen.getByText("Streaming is not available yet"),
    ).toBeInTheDocument();
  });
  it("shows an honest disconnected state when the API fails", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
    render(<App />);
    expect(
      await screen.findByText(/The management service could not be reached/),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Unreachable").length).toBeGreaterThan(0);
    expect(screen.queryByText("Online")).not.toBeInTheDocument();
  });
  it("supports interface filtering, read-only settings, and theme selection", async () => {
    mockAPI();
    render(<App />);
    await screen.findByText("Online");
    fireEvent.click(screen.getByRole("button", { name: "Network Interfaces" }));
    const input = screen.getByRole("textbox", { name: "Filter interfaces" });
    fireEvent.change(input, { target: { value: "no-match" } });
    expect(
      screen.getByText("No matching network interfaces."),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Settings" }));
    expect(screen.getByText("Read-only configuration")).toBeInTheDocument();
    expect(screen.getByText("data/test.db")).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Switch to light theme" }),
    );
    await waitFor(() =>
      expect(document.documentElement.dataset.theme).toBe("light"),
    );
  });
});
