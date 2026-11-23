import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { usePolling } from "./usePolling";
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});
it("polls at the configured interval, pauses and cancels pending work", async () => {
  vi.useFakeTimers();
  let calls = 0;
  const signals: AbortSignal[] = [];
  const loader = vi.fn((signal: AbortSignal): Promise<number> => {
    signals.push(signal);
    calls++;
    if (calls === 3)
      return new Promise((_, reject) =>
        signal.addEventListener("abort", () => reject(new Error("aborted"))),
      );
    return Promise.resolve(calls);
  });
  function View({ paused = false }: { paused?: boolean }) {
    const poll = usePolling(loader, "test", 3, paused);
    return <span>{poll.data ?? "loading"}</span>;
  }
  const view = render(<View />);
  await act(async () => {});
  expect(screen.getByText("1")).toBeInTheDocument();
  await act(async () => {
    await vi.advanceTimersByTimeAsync(3000);
  });
  expect(screen.getByText("2")).toBeInTheDocument();
  view.rerender(<View paused />);
  await act(async () => {
    await vi.advanceTimersByTimeAsync(9000);
  });
  expect(loader).toHaveBeenCalledTimes(2);
  view.rerender(<View />);
  await act(async () => {});
  expect(loader).toHaveBeenCalledTimes(3);
  expect(signals.at(-1)?.aborted).toBe(false);
  view.unmount();
  await act(async () => {});
  expect(signals.at(-1)?.aborted).toBe(true);
});
