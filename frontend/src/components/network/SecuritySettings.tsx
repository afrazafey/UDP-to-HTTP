import { useId } from "react";
import { ChevronDown, Radio, ShieldCheck, Users } from "lucide-react";
import type { Settings, SettingsView } from "../../types/operations";
import { Definition } from "../ui";
import { allMulticast, analyzeNetworkList, NetworkList, splitNetworkList } from "./NetworkList";
import { PolicyTest } from "./PolicyTest";

export function SecuritySettings({
  draft, readOnly, onChange,
}: {
  draft: Settings;
  readOnly: SettingsView["read_only"];
  onChange: (streaming: Settings["streaming"]) => void;
}) {
  const formatHintId = useId();
  const s = draft.streaming;
  const allowAll = analyzeNetworkList(s.allowed_multicast_groups, "multicast").entries.has(allMulticast);
  function toggleAll(enabled: boolean) {
    const entries = splitNetworkList(s.allowed_multicast_groups)
      .filter((entry) => !analyzeNetworkList([entry], "multicast").entries.has(allMulticast));
    onChange({ ...s, allowed_multicast_groups: enabled ? [...entries, allMulticast] : entries });
  }
  function readValue(key: string, fallback = "Unavailable") {
    const value = readOnly[key];
    return (Array.isArray(value) ? value.join(", ") : value) || fallback;
  }

  return (
    <div className="op-security-form">
      <p id={formatHintId} className="op-security-format">Separate IP addresses and CIDR ranges with commas or line breaks.</p>
      <div className="op-security-rules">
        <section className="op-security-card" aria-labelledby={`${formatHintId}-clients`}>
          <header className="op-security-card-head">
            <Users size={19} aria-hidden="true" />
            <h3 id={`${formatHintId}-clients`}>Playback access</h3>
          </header>
          <NetworkList
            label="Allowed playback clients" kind="client" formatHintId={formatHintId}
            help="Matches the connecting IP. An empty list denies playback."
            example={"10.14.0.5\n192.168.10.0/24"} value={s.allowed_client_networks}
            onChange={(allowed_client_networks) => onChange({ ...s, allowed_client_networks })}
          />
        </section>

        <section className="op-security-card" aria-labelledby={`${formatHintId}-unicast`}>
          <header className="op-security-card-head">
            <Radio size={19} aria-hidden="true" />
            <h3 id={`${formatHintId}-unicast`}>Unicast sources</h3>
          </header>
          <NetworkList
            label="Allowed unicast senders" kind="unicast" formatHintId={formatHintId}
            help="IPv4 sender addresses or networks."
            example="10.14.0.1, 10.20.0.0/24" value={s.allowed_unicast_sources}
            loopback={s.allow_loopback_unicast}
            onChange={(allowed_unicast_sources) => onChange({ ...s, allowed_unicast_sources })}
          />
          <label className="op-security-toggle">
            <input type="checkbox" checked={s.allow_loopback_unicast} onChange={(event) =>
              onChange({ ...s, allow_loopback_unicast: event.currentTarget.checked })} />
            <span><strong>Allow loopback senders</strong><small>Loopback addresses must also match the sender list.</small></span>
          </label>
        </section>

        <section className="op-security-card op-security-multicast" aria-labelledby={`${formatHintId}-multicast`}>
          <header className="op-security-card-head">
            <Radio size={19} aria-hidden="true" />
            <h3 id={`${formatHintId}-multicast`}>Multicast sources</h3>
          </header>
          <div className="op-security-multicast-fields">
            <NetworkList
              label="Allowed multicast groups" kind="multicast" formatHintId={formatHintId}
              help="IPv4 groups and ranges within 224.0.0.0/4."
              example={"239.1.1.61\n239.255.20.0/24"} value={s.allowed_multicast_groups}
              onChange={(allowed_multicast_groups) => onChange({ ...s, allowed_multicast_groups })}
            />
            <NetworkList
              label="Blocked multicast networks" kind="blocked" formatHintId={formatHintId}
              help="Blocked ranges take precedence. Local-control groups are always blocked."
              example="224.0.0.0/24" value={s.blocked_multicast_networks}
              onChange={(blocked_multicast_networks) => onChange({ ...s, blocked_multicast_networks })}
            />
          </div>
          <label className={`op-security-toggle${allowAll ? " is-warning" : ""}`}>
            <input type="checkbox" checked={allowAll} onChange={(event) => toggleAll(event.currentTarget.checked)} />
            <span>
              <strong>Allow all IPv4 multicast groups</strong>
              <small>{allowAll
                ? "Review this broad access rule before saving. Blocked ranges still take precedence."
                : "Adds 224.0.0.0/4. Reserved and blocked ranges still take precedence."}</small>
            </span>
          </label>
        </section>
      </div>

      <PolicyTest draft={draft} />
      <details className="op-security-details">
        <summary><ShieldCheck size={18} aria-hidden="true" /><span>Management access</span><small>Read-only</small><ChevronDown size={16} aria-hidden="true" /></summary>
        <Definition items={[
          ["Management policy", readValue("authentication")],
          ["Session status", "Browser sessions are not managed by the daemon"],
          ["Management bind", readValue("management_binding")],
          ["Trusted reverse proxies", readValue("trusted_proxies", "None")],
          ["Trusted management origins", readValue("trusted_origins", "None")],
          ["Playback authorization", readValue("playback_authorization")],
        ]} />
      </details>
    </div>
  );
}
