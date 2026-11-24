import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import { analyzeNetworkList, NetworkList } from "./NetworkList";

function Example() {
  const [value, setValue] = useState<string[]>([]);
  return <NetworkList label="Playback clients" help="Actual client IPs." example="10.14.0.5" kind="client" value={value} onChange={setValue} />;
}

describe("Network list editor", () => {
  it("shows chips, duplicates, masking and field errors from pasted text", () => {
    render(<Example />);
    const input = screen.getByRole("textbox", { name: "Playback clients" });
    fireEvent.change(input, { target: { value: " 10.14.0.5, 10.14.0.5/32 \r\n192.168.10.77/24\ninvalid" } });
    expect(screen.getByText("2 entries")).toBeVisible();
    expect(screen.getByText(/1 duplicate/)).toBeVisible();
    expect(screen.getByText(/Invalid for this field: invalid/)).toBeVisible();
    expect(input).toHaveAttribute("aria-invalid", "true");
    fireEvent.click(screen.getByRole("button", { name: "Remove 10.14.0.5" }));
    expect(screen.queryByRole("button", { name: "Remove 10.14.0.5" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove 192.168.10.0/24" })).toBeVisible();
  });

  it("normalizes IPv6 and IPv4-mapped addresses for preview", () => {
    const parsed = analyzeNetworkList(["2001:0db8::12/64", "::ffff:10.14.0.5", "10.14.0.5/32"], "client");
    expect([...parsed.entries.values()]).toEqual(["2001:db8::/64", "10.14.0.5"]);
    expect(parsed.duplicates).toBe(1);
    expect(parsed.invalid).toEqual([]);
  });
});
