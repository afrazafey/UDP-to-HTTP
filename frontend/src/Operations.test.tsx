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
      case "/api/v1/diagnostics":
        result = { channels: [] };
        break;
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
        } else if (request?.method === "PUT") {
          data.settings.settings = JSON.parse(String(request.body)).settings;
          data.settings.revision++;
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
        } else result = {
          notes: ["Point-in-time check passed"], preview: body.settings,
          requires_all_multicast_confirmation: body.settings.streaming.allowed_multicast_groups.includes("224.0.0.0/4"),
        };
        break;
      }
      case "/api/v1/settings/policy-test":
        result = { allowed: false, reason: "multicast group is blocked", matched: "239.1.0.0/16" };
        break;
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
  it("reviews and confirms multicast allow-all and reloads the saved policy", async () => {
    const { fetch } = setup();
    window.location.hash = "#/settings";
    const app = renderApp();
    fireEvent.click(await screen.findByRole("tab", { name: "Security" }));
    const allow = screen.getByRole("checkbox", { name: /Allow all IPv4/ });
    fireEvent.click(allow);
    expect(screen.getByRole("button", { name: "Remove 224.0.0.0/4" })).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Reset draft" }));
    expect(allow).not.toBeChecked();
    fireEvent.click(allow);
    fireEvent.click(screen.getByRole("button", { name: /Validate & review/ }));
    let review = within(await screen.findByRole("dialog"));
    expect(review.getByRole("button", { name: "Confirm save" })).toBeDisabled();
    fireEvent.click(review.getByRole("button", { name: "Keep editing" }));
    expect(allow).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: /Validate & review/ }));
    review = within(await screen.findByRole("dialog"));
    fireEvent.click(review.getByRole("checkbox", { name: /I confirm/ }));
    fireEvent.click(review.getByRole("button", { name: "Confirm save" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    const save = fetch.mock.calls.find(([path, request]) => path === "/api/v1/settings" && request?.method === "PUT");
    expect(JSON.parse(String(save?.[1]?.body))).toMatchObject({ confirm_all_multicast: true });
    app.unmount();
    renderApp();
    fireEvent.click(await screen.findByRole("tab", { name: "Security" }));
    expect(screen.getByRole("checkbox", { name: /Allow all IPv4/ })).toBeChecked();
  });

  it("shows the backend result for a draft address check", async () => {
    const { fetch } = setup();
    window.location.hash = "#/settings";
    renderApp();
    fireEvent.click(await screen.findByRole("tab", { name: "Security" }));
    fireEvent.change(screen.getByLabelText("IP address"), { target: { value: "239.1.1.61" } });
    fireEvent.change(screen.getByLabelText("Access list"), { target: { value: "multicast" } });
    fireEvent.change(screen.getByLabelText("Policy to evaluate"), { target: { value: "draft" } });
    fireEvent.click(screen.getByRole("button", { name: "Test address" }));
    expect(await screen.findByText("239.1.0.0/16")).toBeVisible();
    const check = fetch.mock.calls.find(([path]) => path === "/api/v1/settings/policy-test");
    expect(JSON.parse(String(check?.[1]?.body))).toMatchObject({ address: "239.1.1.61", kind: "multicast", settings: { streaming: {} } });
  });
  it("keeps deployment branding separate from release maturity and treats channel markup as text", async () => {
    const { data } = setup();
    data.system.environment = "Production";
    data.system.release_maturity = "Release candidate";
    data.streams[0].channel.name = "<b>Untrusted channel name</b>";
    renderApp();
    expect(await screen.findByText("<b>Untrusted channel name</b>")).toBeVisible();
    expect(document.querySelector(".op-channel-identity b")).toBeNull();
    fireEvent.click(screen.getByRole("link", { name: "System Information" }));
    expect(await screen.findByText("Release candidate")).toBeVisible();
    const website = within(screen.getByRole("heading", { name: "About" }).closest("section")!).getByRole("link", { name: "afraaz.xyz" });
    expect(website).toHaveAttribute("href", "https://afraaz.xyz");
    expect(website).toHaveAttribute("rel", "noopener noreferrer");
  });
  it("opens the real channel form from Overview and restores focus on Escape", async () => {
    setup();
    renderApp();
    await screen.findByText("Total Channels");
    const trigger = screen.getByRole("button", { name: "Add Channel" });
    trigger.focus();
    fireEvent.click(trigger);
    const dialog = screen.getByRole("dialog", { name: "Add stream" });
    expect(
      within(dialog).getByText("Source configuration"),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText("Advanced receiver settings"),
    ).toBeInTheDocument();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(trigger).toHaveFocus();
  });
  it("persists the topbar theme against the current revision and preserves staged settings", async () => {
    const { fetch } = setup();
    renderApp();
    await screen.findByText("Total Channels");
    fireEvent.change(screen.getByLabelText("Dashboard theme"), {
      target: { value: "dark" },
    });
    await waitFor(() =>
      expect(
        fetch.mock.calls.some(
          ([path, request]) =>
            path === "/api/v1/settings" && request?.method === "PUT",
        ),
      ).toBe(true),
    );
    const request = fetch.mock.calls.find(
      ([path, request]) =>
        path === "/api/v1/settings" && request?.method === "PUT",
    )?.[1];
    expect(JSON.parse(String(request?.body))).toEqual({
      revision: 0,
      confirm: true,
      settings: {
        ...fixture().settings.settings,
        dashboard: { ...fixture().settings.settings.dashboard, theme: "dark" },
      },
    });
  });
  it("loads real API totals, bounded chart history and unsupported resource states", async () => {
    setup();
    renderApp();
    expect(
      screen.getByRole("status", { name: "Loading service data" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("img", { name: "Network throughput" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("1.50 Mbps").length).toBeGreaterThan(0);
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
    expect(screen.queryByText("Total Channels")).not.toBeInTheDocument();
  });
  it("shows a failed playback listener in service health even when the service is running", async () => {
    const { data } = setup();
    data.system.http_listeners.push({
      kind: "playback",
      address: "127.0.0.1:7781",
      state: "failed",
    });
    renderApp();
    expect(await screen.findByText("Bind failed")).toBeVisible();
    expect(screen.getByText("Degraded")).toBeVisible();
  });
  it("navigates with accessible mobile dialog and keyboard focus restoration", async () => {
    setup();
    renderApp();
    await screen.findByText("Total Channels");
    const button = screen.getByRole("button", { name: "Open navigation" });
    button.focus();
    fireEvent.click(button);
    const dialog = screen.getByRole("dialog", { name: "Navigation" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(
      within(dialog).getByRole("link", { name: "Channels" }),
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
    await screen.findByText("Total Channels");
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
    expect(await screen.findAllByText("2.50 Mbps")).toHaveLength(2);
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
