import { useId, useState } from "react";
import { Button, ErrorBanner, useAction } from "../ui";
import { mutate } from "../../services/api";
import type { Settings } from "../../types/operations";

type Decision = { allowed: boolean; reason: string; matched?: string };

export function PolicyTest({ draft }: { draft: Settings }) {
  const id = useId();
  const [address, setAddress] = useState("");
  const [kind, setKind] = useState("playback");
  const [scope, setScope] = useState("saved");
  const [result, setResult] = useState<{ decision: Decision; label: string } | null>(null);
  const action = useAction();
  async function check() {
    setResult(null);
    await action.run(async () => {
      const decision = await mutate<Decision>("/api/v1/settings/policy-test", "POST", {
        address: address.trim(), kind, ...(scope === "draft" ? { settings: draft } : {}),
      });
      setResult({ decision, label: `${address.trim()} · ${scope === "draft" ? "Unsaved policy" : "Active policy"}` });
    }, "Address evaluated");
  }
  return (
    <section className="op-policy-test" aria-labelledby={`${id}-title`}>
      <h3 id={`${id}-title`}>Test a network address</h3>
      <div className="op-form-grid">
        <label className="op-field">
          IP address
          <input value={address} placeholder="239.1.1.61" maxLength={64}
            onChange={(event) => { setAddress(event.target.value); setResult(null); }}
            onKeyDown={(event) => {
              if (event.key !== "Enter") return;
              event.preventDefault();
              if (address.trim() && !action.busy) void check();
            }} />
        </label>
        <label className="op-field">
          Access list
          <select value={kind} onChange={(event) => { setKind(event.target.value); setResult(null); }}>
            <option value="playback">Playback client</option>
            <option value="playlist">Remote playlist client</option>
            <option value="multicast">Multicast source</option>
            <option value="unicast">Unicast sender</option>
          </select>
        </label>
        <label className="op-field">
          Policy to evaluate
          <select value={scope} onChange={(event) => { setScope(event.target.value); setResult(null); }}>
            <option value="saved">Active policy</option>
            <option value="draft">Unsaved policy</option>
          </select>
        </label>
      </div>
      <Button type="button" onClick={() => void check()} disabled={action.busy || !address.trim()}>
        {action.busy ? "Testing…" : "Test address"}
      </Button>
      {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
      {result && (
        <p role="status" className="op-callout">
          <strong>{result.decision.allowed ? "Allowed" : "Blocked"}</strong> · {result.label}
          <br />{result.decision.reason}{result.decision.matched && <> · <code>{result.decision.matched}</code></>}
        </p>
      )}
    </section>
  );
}
