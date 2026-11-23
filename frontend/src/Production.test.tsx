import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { ToastProvider } from "./components/ui";
import { PlaylistsPage } from "./pages/PlaylistsPage";
import { StreamsPage } from "./pages/StreamsPage";
import { Diagnostics } from "./components/network/Diagnostics";
import { fixture } from "./test/operations-fixture";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
function mockRequests(failImport = false) {
  const data = fixture();
  const good = {
    ...data.streams[0].channel,
    id: "imported",
    name: "Imported",
    group: "MUSIC",
  };
  const fetch = vi.fn(async (path: string, options?: RequestInit) => {
    if (path.startsWith("/api/v1/playlist.m3u"))
      return new Response("#EXTM3U\n", {
        headers: { "Content-Type": "audio/x-mpegurl" },
      });
    let body: unknown = {};
    let status = 200;
    if (path === "/api/v1/csrf") body = { token: "valid-csrf" };
    else if (path === "/api/v1/playlist/config")
      body = {
        playlist: {
          advertised_host: "playback.example",
          remote_enabled: false,
        },
        primary_port: 7781,
        secondary_port: 7782,
      };
    else if (path === "/api/v1/import/preview")
      body = {
        entries: [
          { line: 2, channel: good, error: "", duplicate: "new" },
          {
            line: 4,
            channel: { ...good, id: "blocked", name: "Blocked" },
            error:
              "source address is not in the corresponding server allowlist",
            duplicate: "new",
          },
        ],
      };
    else if (path === "/api/v1/import/commit" && failImport) {
      status = 400;
      body = { error: "source policy changed; preview again" };
    }
    if (path === "/api/v1/diagnostics")
      body = options?.method
        ? { configuration_valid: true, socket_bound: false }
        : { channels: [] };
    return new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  });
  vi.stubGlobal("fetch", fetch);
  return { data, fetch };
}
it("previews policy rejection and commits only selected valid channels after confirmation", async () => {
  const { data, fetch } = mockRequests();
  data.settings.active.network.multicast_interface = data.interfaces[0].name;
  render(
    <ToastProvider>
      <PlaylistsPage data={data} refresh={vi.fn()} />
    </ToastProvider>,
  );
  fireEvent.change(screen.getByLabelText("Playlist text"), {
    target: { value: "#EXTM3U\n#EXTINF:-1,Imported\nudp://239.1.1.61:1234" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Preview import" }));
  await screen.findByText("Imported");
  expect(screen.getByLabelText("Select import Blocked")).toBeDisabled();
  expect(
    fetch.mock.calls.some(([path]) => path === "/api/v1/import/commit"),
  ).toBe(false);
  fireEvent.click(screen.getByRole("button", { name: "Import selected" }));
  const dialog = await screen.findByRole("dialog");
  fireEvent.click(
    within(dialog).getByRole("button", { name: "Confirm import" }),
  );
  await waitFor(() =>
    expect(
      fetch.mock.calls.some(([path]) => path === "/api/v1/import/commit"),
    ).toBe(true),
  );
  const call = fetch.mock.calls.find(
    ([path]) => path === "/api/v1/import/commit",
  )!;
  const body = JSON.parse(String(call[1]?.body));
  expect(body.confirm).toBe(true);
  expect(body.channels.map((c: { id: string }) => c.id)).toEqual(["imported"]);
  expect(call[1]?.headers).toMatchObject({ "X-CSRF-Token": "valid-csrf" });
});
it("exports the selected group and playback port as a named M3U download", async () => {
  const { data, fetch } = mockRequests();
  data.streams[0].channel.group = "MUSIC";
  const OriginalURL = URL;
  class DownloadURL extends OriginalURL {
    static createObjectURL = vi.fn(() => "blob:test-playlist");
    static revokeObjectURL = vi.fn();
  }
  vi.stubGlobal("URL", DownloadURL);
  const filenames: string[] = [];
  vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (
    this: HTMLAnchorElement,
  ) {
    filenames.push(this.download);
  });
  render(
    <ToastProvider>
      <PlaylistsPage data={data} refresh={vi.fn()} />
    </ToastProvider>,
  );
  await screen.findByText("playback.example");
  fireEvent.change(screen.getByLabelText("Export group"), {
    target: { value: "MUSIC" },
  });
  fireEvent.change(screen.getByLabelText("Playback port"), {
    target: { value: "primary" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Download M3U" }));
  await waitFor(() => expect(filenames).toContain("udphttp-channels.m3u"));
  expect(
    fetch.mock.calls.some(
      ([path]) => path === "/api/v1/playlist.m3u?port=primary&group=MUSIC",
    ),
  ).toBe(true);
});
it("retains import preview and selected draft when server policy changes", async () => {
  const { data } = mockRequests(true);
  data.settings.active.network.multicast_interface = data.interfaces[0].name;
  render(
    <ToastProvider>
      <PlaylistsPage data={data} refresh={vi.fn()} />
    </ToastProvider>,
  );
  fireEvent.change(screen.getByLabelText("Playlist text"), {
    target: { value: "#EXTM3U" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Preview import" }));
  await screen.findByText("Imported");
  fireEvent.click(screen.getByRole("button", { name: "Import selected" }));
  fireEvent.click(
    within(await screen.findByRole("dialog")).getByRole("button", {
      name: "Confirm import",
    }),
  );
  await screen.findAllByText("source policy changed; preview again");
  expect(screen.getByRole("dialog")).toBeInTheDocument();
  expect(screen.getByLabelText("Playlist text")).toHaveValue("#EXTM3U");
});
it("rejects oversized playlist uploads before reading or submitting them", async () => {
  const { data, fetch } = mockRequests();
  render(
    <ToastProvider>
      <PlaylistsPage data={data} refresh={vi.fn()} />
    </ToastProvider>,
  );
  fireEvent.change(screen.getByLabelText("M3U or M3U8 file"), {
    target: { files: [new File([new Uint8Array(1048577)], "large.m3u")] },
  });
  expect(await screen.findByRole("alert")).toHaveTextContent("up to 1 MiB");
  expect(
    fetch.mock.calls.some(([, options]) => options?.method === "POST"),
  ).toBe(false);
});
it("confirms bulk deletion and sends the selected stable IDs", async () => {
  const { data, fetch } = mockRequests();
  render(
    <ToastProvider>
      <StreamsPage data={data} refresh={vi.fn()} />
    </ToastProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "Select visible" }));
  fireEvent.click(screen.getByRole("button", { name: "Bulk delete" }));
  const dialog = await screen.findByRole("dialog");
  expect(
    fetch.mock.calls.some(([path]) => path === "/api/v1/streams/bulk"),
  ).toBe(false);
  fireEvent.click(
    within(dialog).getByRole("button", { name: "Confirm bulk action" }),
  );
  await waitFor(() =>
    expect(
      fetch.mock.calls.some(([path]) => path === "/api/v1/streams/bulk"),
    ).toBe(true),
  );
  const body = JSON.parse(
    String(
      fetch.mock.calls.find(([path]) => path === "/api/v1/streams/bulk")![1]
        ?.body,
    ),
  );
  expect(body.ids).toEqual(data.streams.map((s) => s.channel.id));
  expect(body.confirm).toBe(true);
});
it("diagnostics send a channel ID and safe action without an arbitrary destination", async () => {
  const { data, fetch } = mockRequests();
  render(
    <ToastProvider>
      <Diagnostics data={data} />
    </ToastProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "Check source" }));
  await waitFor(() =>
    expect(
      fetch.mock.calls.some(
        ([path, options]) =>
          path === "/api/v1/diagnostics" && options?.method === "POST",
      ),
    ).toBe(true),
  );
  const call = fetch.mock.calls.find(
    ([path, options]) =>
      path === "/api/v1/diagnostics" && options?.method === "POST",
  )!;
  expect(JSON.parse(String(call[1]?.body))).toEqual({
    action: "source",
    channel_id: data.streams[0].channel.id,
  });
  expect(await screen.findByText(/"socket_bound": false/)).toBeInTheDocument();
});
