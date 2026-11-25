import { useId, useState } from "react";
import { CircleCheck, CircleX, Search } from "lucide-react";
import { Button, ErrorBanner, useAction } from "../ui";
import { mutate } from "../../services/api";
import type { Settings } from "../../types/operations";

type Decision = { allowed: boolean; reason: string; matched?: string };

export function PolicyTest({ draft }: { draft: Settings }) {
  const id = useId();
  const [address, setAddress] = useState("");
  const [kind, setKind] = useState("playback");
  const [scope, setScope] = useState("saved");
  const [result, setResult] = useState<{ decision: Decision; label: string; testedDraft?: Settings } | null>(null);
  const visibleResult = result && (!result.testedDraft || result.testedDraft === draft) ? result : null;
  const action = useAction();
  async function check() {
    setResult(null);
    await action.run(async () => {
      const decision = await mutate<Decision>("/api/v1/settings/policy-test", "POST", {
        address: address.trim(), kind, ...(scope === "draft" ? { settings: draft } : {}),
      });
      setResult({
        decision,
        label: `${address.trim()} · ${scope === "draft" ? "Unsaved policy" : "Active policy"}`,
        testedDraft: scope === "draft" ? draft : undefined,
      });
    }, "Address evaluated");
  }
  return (
    <section className="op-security-card op-policy-test" aria-labelledby={`${id}-title`}>
      <header className="op-security-card-head">
        <Search size={19} aria-hidden="true" />
        <h3 id={`${id}-title`}>Test a network address</h3>
      </header>
      <div className="op-policy-test-fields">
        <label className="op-field">
          IP address
          <input value={address} placeholder="239.1.1.61" maxLength={64}
            disabled={action.busy} spellCheck={false} autoCapitalize="none"
            onChange={(event) => { setAddress(event.target.value); setResult(null); }}
            onKeyDown={(event) => {
              if (event.key !== "Enter") return;
              event.preventDefault();
              if (address.trim() && !action.busy) void check();
            }} />
        </label>
        <label className="op-field">
          Access list
          <select value={kind} disabled={action.busy} onChange={(event) => { setKind(event.target.value); setResult(null); }}>
            <option value="playback">Playback client</option>
            <option value="playlist">Remote playlist client</option>
            <option value="multicast">Multicast source</option>
            <option value="unicast">Unicast sender</option>
          </select>
        </label>
        <label className="op-field">
          Policy to evaluate
          <select value={scope} disabled={action.busy} onChange={(event) => { setScope(event.target.value); setResult(null); }}>
            <option value="saved">Active policy</option>
            <option value="draft">Unsaved policy</option>
          </select>
        </label>
        <Button type="button" onClick={() => void check()} disabled={action.busy || !address.trim()}>
          {action.busy ? "Testing…" : "Test address"}
        </Button>
      </div>
      {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
      {visibleResult && (
        <div role="status" className={`op-policy-result ${visibleResult.decision.allowed ? "is-allowed" : "is-blocked"}`}>
          {visibleResult.decision.allowed ? <CircleCheck size={18} aria-hidden="true" /> : <CircleX size={18} aria-hidden="true" />}
          <div>
            <strong>{visibleResult.decision.allowed ? "Allowed" : "Blocked"}</strong>
            <span>{visibleResult.label}</span>
            <p>{visibleResult.decision.reason}{visibleResult.decision.matched && <> · <code>{visibleResult.decision.matched}</code></>}</p>
          </div>
        </div>
      )}
    </section>
  );
}
