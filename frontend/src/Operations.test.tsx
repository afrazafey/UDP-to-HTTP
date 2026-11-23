import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { OperationsApp } from "./OperationsApp";
import { ToastProvider } from "./components/ui";
import { NetworkPage } from "./pages/NetworkPage";
import { LogsPage } from "./pages/LogsPage";
import { fixture, statusFixture } from "./test/operations-fixture";
function setup(
  options: { fail?: boolean; saveFail?: boolean; channelFail?: boolean } = {},
) {
  const data = fixture();
  const fetch = vi.fn(async (path: string, request?: RequestInit) => {
    if (options.fail)
      return new Response(JSON.stringify({ error: "Service offline" }), {
        status: 503,
        headers: { "Content-Type": "application/json" },
      });
    let result: unknown = {};
    let code = 200;
    const parsed = new URL(path, "http://localhost");
    switch (parsed.pathname) {
      case "/api/v1/dashboard":
        result = data.dashboard;
        break;
      case "/api/v1/streams":
        result = request?.method
          ? { stream: data.streams[0] }
          : { streams: data.streams };
        break;
      case "/api/v1/clients":
        result = { clients: data.clients };
        break;
      case "/api/v1/interfaces":
        result = { interfaces: data.interfaces };
        break;
      case "/api/v1/system":
        result = data.system;
        break;
      case "/api/v1/settings":
        result = data.settings;
        if (request?.method && options.saveFail) {
          result = { error: "Settings disk write failed" };
          code = 500;
        }
        break;
      case "/api/v1/settings/validate": {
        const body = JSON.parse(String(request?.body));
        if (
          body.settings.network.http_port ===
          body.settings.network.secondary_http_port
        ) {
          result = { error: "Streaming HTTP ports must differ" };
          code = 400;
        } else result = { notes: ["Point-in-time check passed"] };
        break;
      }
      case "/api/v1/csrf":
        result = { token: "test-csrf" };
        break;
      case "/api/v1/logs":
        result = {
          logs: [
            {
              id: 1,
              time: "2026-11-23T08:00:00Z",
              level: "warn",
              component: "stream",
              stream_id: "news",
              message: "Actual fixture event",
              context: { safe: true },
            },
          ],
          total: 1,
        };
        break;
      default:
        result = { history: data.dashboard.history };
    }
    if (
      options.channelFail &&
      request?.method &&
      parsed.pathname.startsWith("/api/v1/streams/")
    ) {
      result = { error: "Channel write failed" };
      code = 500;
    }
    return new Response(JSON.stringify(result), {
      status: code,
      headers: { "Content-Type": "application/json" },
    });
  });
  vi.stubGlobal("fetch", fetch);
  return { data, fetch };
}
function renderApp() {
  return render(
    <ToastProvider>
      <OperationsApp status={statusFixture} />
    </ToastProvider>,
  );
}
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  localStorage.clear();
  window.location.hash = "";
});
describe("Phase 3 dashboard", () => {
  it("loads real API totals, bounded chart history and unsupported resource states", async () => {
    setup();
    renderApp();
    expect(
      screen.getByRole("status", { name: "Loading service data" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("img", { name: "Network throughput" }),
    ).toBeInTheDocument();
    expect(screen.getByText("1.50 Mbps")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("link", { name: "Monitoring" }));
    await screen.findByText("Process resident memory");
    expect(screen.getAllByText("Unavailable").length).toBeGreaterThan(0);
  });
  it("shows a visible API failure instead of invented zeros", async () => {
    setup({ fail: true });
    renderApp();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Service offline",
    );
    expect(screen.queryByText("Configured channels")).not.toBeInTheDocument();
  });
  it("navigates with accessible mobile dialog and keyboard focus restoration", async () => {
    setup();
    renderApp();
    await screen.findByText("Configured channels");
    const button = screen.getByRole("button", { name: "Open navigation" });
    button.focus();
    fireEvent.click(button);
    const dialog = screen.getByRole("dialog", { name: "Navigation" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(
      within(dialog).getByRole("link", { name: "Streams" }),
    ).toBeInTheDocument();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(button).toHaveFocus();
    fireEvent.click(screen.getByRole("button", { name: "Collapse sidebar" }));
    expect(
      screen.getByRole("button", { name: "Expand sidebar" }),
    ).toBeInTheDocument();
  });
  it("pauses polling and supports a confirmed manual refresh", async () => {
    const { fetch } = setup();
    renderApp();
    await screen.findByText("Configured channels");
    fireEvent.click(screen.getByRole("button", { name: "Pause monitoring" }));
    expect(screen.getByText("Monitoring paused")).toBeInTheDocument();
    const before = fetch.mock.calls.length;
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 20));
    });
    expect(fetch.mock.calls.length).toBe(before);
    fireEvent.click(screen.getByRole("button", { name: "Refresh now" }));
    await waitFor(() =>
      expect(fetch.mock.calls.length).toBeGreaterThan(before),
    );
    expect(
      screen.getByRole("button", { name: "Resume monitoring" }),
    ).toBeInTheDocument();
  });
  it("edits supported channel settings through the existing API", async () => {
    const { fetch } = setup();
    window.location.hash = "/streams";
    renderApp();
    await screen.findByText("Test News");
    fireEvent.click(screen.getByRole("button", { name: "Edit Test News" }));
    const dialog = screen.getByRole("dialog", { name: "Edit Test News" });
    expect(within(dialog).getByLabelText("Channel ID")).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText("Channel name"), {
      target: { value: "Edited News" },
    });
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Save channel" }),
    );
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/api/v1/streams/news",
        expect.objectContaining({
          method: "PUT",
          body: expect.stringContaining('"name":"Edited News"'),
        }),
      ),
    );
  });
  it("preserves channel edits when persistence fails", async () => {
    setup({ channelFail: true });
    window.location.hash = "/streams";
    renderApp();
    await screen.findByText("Test News");
    fireEvent.click(screen.getByRole("button", { name: "Edit Test News" }));
    fireEvent.change(screen.getByLabelText("Channel name"), {
      target: { value: "Unsaved edit" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save channel" }));
    await screen.findAllByText("Channel write failed");
    expect(screen.getByLabelText("Channel name")).toHaveValue("Unsaved edit");
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
  it("confirms deletion and sends lifecycle commands from the Phase 3 table", async () => {
    const { fetch } = setup();
    window.location.hash = "/streams";
    renderApp();
    await screen.findByText("Test News");
    fireEvent.click(screen.getByRole("button", { name: "Start Test News" }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/api/v1/streams/news/start",
        expect.objectContaining({ method: "POST" }),
      ),
    );
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Delete Test News" }),
      ).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Delete Test News" }));
    expect(
      screen.getByRole("dialog", { name: "Delete Test News" }),
    ).toHaveTextContent("2 active client(s)");
    fireEvent.click(screen.getByRole("button", { name: "Delete channel" }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/api/v1/streams/news",
        expect.objectContaining({ method: "DELETE" }),
      ),
    );
  });
  it("refreshes measured monitoring values without fabricating history", async () => {
    const { data } = setup();
    renderApp();
    await screen.findByRole("img", { name: "Network throughput" });
    data.dashboard.summary!.input_bitrate = 2500000;
    data.dashboard.history.push({
      ...data.dashboard.sample,
      time: "2026-11-23T08:00:06Z",
      input_bitrate: 2500000,
    });
    fireEvent.click(screen.getByRole("button", { name: "Refresh now" }));
    await screen.findByText("2.50 Mbps");
    expect(
      screen.getByRole("img", { name: "Network throughput" }),
    ).toHaveTextContent("3 measured samples");
  });
});
describe("Network safety and operational logs", () => {
  it("validates listener collisions and requires confirmation before saving", async () => {
    const { data, fetch } = setup();
    render(
      <ToastProvider>
        <NetworkPage data={data} refresh={() => {}} />
      </ToastProvider>,
    );
    fireEvent.change(screen.getByLabelText("Secondary HTTP playback port"), {
      target: { value: "7781" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Validate & preview configuration" }),
    );
    await screen.findAllByText("Streaming HTTP ports must differ");
    expect(
      fetch.mock.calls.some(([, request]) => request?.method === "PUT"),
    ).toBe(false);
    fireEvent.change(screen.getByLabelText("Secondary HTTP playback port"), {
      target: { value: "0" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Validate & preview configuration" }),
    );
    await screen.findByRole("dialog", { name: "Review configuration changes" });
    expect(
      fetch.mock.calls.some(([, request]) => request?.method === "PUT"),
    ).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: "Confirm save" }));
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/api/v1/settings",
        expect.objectContaining({
          method: "PUT",
          body: expect.stringContaining('"confirm":true'),
        }),
      ),
    );
  });
  it("keeps the network draft and review dialog after a failed persistence attempt", async () => {
    const { data } = setup({ saveFail: true });
    render(
      <ToastProvider>
        <NetworkPage data={data} refresh={() => {}} />
      </ToastProvider>,
    );
    fireEvent.change(screen.getByLabelText("Primary HTTP playback port"), {
      target: { value: "7790" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Validate & preview configuration" }),
    );
    await screen.findByRole("dialog");
    fireEvent.click(screen.getByRole("button", { name: "Confirm save" }));
    await waitFor(() =>
      expect(
        screen.getAllByText("Settings disk write failed").length,
      ).toBeGreaterThan(0),
    );
    expect(screen.getByLabelText("Primary HTTP playback port")).toHaveValue(
      7790,
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
  it("uses backend log filtering and shows real structured entries", async () => {
    const { data, fetch } = setup();
    render(
      <ToastProvider>
        <LogsPage data={data} paused={false} interval={5} />
      </ToastProvider>,
    );
    await screen.findByText("Actual fixture event");
    fireEvent.change(screen.getByLabelText("Severity"), {
      target: { value: "warn" },
    });
    fireEvent.change(screen.getByLabelText("Search log messages"), {
      target: { value: "receiver" },
    });
    await waitFor(() =>
      expect(
        fetch.mock.calls.some(
          ([path]) =>
            String(path).includes("level=warn") &&
            String(path).includes("search=receiver"),
        ),
      ).toBe(true),
    );
  });
});
