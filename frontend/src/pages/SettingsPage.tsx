import { useId, useState } from "react";
import { FileText, Globe, ListMusic, Radio, Settings2, ShieldCheck } from "lucide-react";
import type { OperationsData } from "../types/operations";
import { Button, CopyButton, Definition, Field, Panel } from "../components/ui";
import {
  NetworkFields,
  useSettingsEditor,
} from "../components/network/SettingsEditor";
const sections = [
  { name: "General", icon: Settings2 },
  { name: "Streaming", icon: Radio },
  { name: "HTTP", icon: Globe },
  { name: "Security", icon: ShieldCheck },
  { name: "Logging", icon: FileText },
  { name: "Recovery & playlists", icon: ListMusic },
];

export function SettingsPage({
  data,
  refresh,
}: {
  data: OperationsData;
  refresh: () => void;
}) {
  const [selected, setSelected] = useState(0);
  const tabId = useId();
  const tab = sections[selected].name;
  const e = useSettingsEditor(data.settings, refresh);
  const d = e.draft;
  const updateStream = (
    key: string,
    value: string | number | boolean | string[],
  ) => e.setDraft({ ...d, streaming: { ...d.streaming, [key]: value } });
  const numeric = (
    label: string,
    key:
      | "udp_receive_buffer"
      | "client_buffer_bytes"
      | "max_clients"
      | "max_streams"
      | "stream_buffer"
      | "max_buffered_bytes",
    min: number,
    max: number,
  ) => (
    <Field label={label}>
      <input
        type="number"
        required
        min={min}
        max={max}
        value={d.streaming[key]}
        onChange={(event) => updateStream(key, Number(event.target.value))}
      />
    </Field>
  );
  const list = (
    label: string,
    key:
      | "allowed_client_networks"
      | "allowed_multicast_groups"
      | "allowed_unicast_sources"
      | "blocked_multicast_networks",
    help: string,
  ) => (
    <Field label={label} help={help}>
      <input
        value={d.streaming[key].join(", ")}
        onChange={(event) =>
          updateStream(
            key,
            event.target.value.split(",").map((s) => s.trim()),
          )
        }
      />
    </Field>
  );
  return (
    <>
      {e.status}
      <div className="op-settings-layout">
        <div className="op-settings-tabs" role="tablist" aria-label="Settings sections">
          {sections.map(({ name, icon: Icon }, index) => (
            <button
              type="button"
              key={name}
              role="tab"
              id={`${tabId}-tab-${index}`}
              aria-controls={`${tabId}-panel`}
              aria-selected={tab === name}
              tabIndex={index === selected ? 0 : -1}
              onClick={() => setSelected(index)}
              onKeyDown={event => {
                const last = sections.length - 1;
                let next = index;
                switch (event.key) {
                  case "ArrowUp":
                  case "ArrowLeft":
                    next = index === 0 ? last : index - 1;
                    break;
                  case "ArrowDown":
                  case "ArrowRight":
                    next = index === last ? 0 : index + 1;
                    break;
                  case "Home": next = 0; break;
                  case "End": next = last; break;
                  default: return;
                }
                event.preventDefault();
                setSelected(next);
                document.getElementById(`${tabId}-tab-${next}`)?.focus();
              }}
            >
              <Icon size={17} aria-hidden="true" /> {name}
            </button>
          ))}
        </div>
        <div role="tabpanel" id={`${tabId}-panel`} aria-labelledby={`${tabId}-tab-${selected}`} tabIndex={0}>
          <Panel
            title={`${tab} settings`}
            description={tab === "Recovery & playlists" ? undefined : "Validate and review changes before saving."}
          >
            {tab === "Recovery & playlists" ? (
              <Definition
                items={[
                  [
                    "Recovery policy",
                    String(
                      data.settings.read_only.recovery_policy ?? "Unavailable",
                    ),
                  ],
                  [
                    "Advertised playback host",
                    String(
                      data.settings.read_only.advertised_playback_host ||
                      "Unconfigured",
                    ),
                  ],
                  [
                    "Remote playlist access",
                    String(
                      data.settings.read_only.remote_playlist_access ??
                      "Unavailable",
                    ),
                  ],
                  [
                    "Apply changes",
                    "Edit the recovery and playlist sections in the server YAML, validate the configuration, and restart.",
                  ],
                ]}
              />
            ) : (
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  void e.validate();
                }}
              >
                {tab === "General" && (
                  <div className="op-form-grid">
                    <Field label="Application name">
                      <input
                        required
                        maxLength={64}
                        value={d.dashboard.name}
                        onChange={(event) =>
                          e.setDraft({
                            ...d,
                            dashboard: { ...d.dashboard, name: event.target.value },
                          })
                        }
                      />
                    </Field>
                    <Field label="Theme">
                      <select
                        value={d.dashboard.theme}
                        onChange={(event) =>
                          e.setDraft({
                            ...d,
                            dashboard: {
                              ...d.dashboard,
                              theme: event.target.value as typeof d.dashboard.theme,
                            },
                          })
                        }
                      >
                        <option value="system">System</option>
                        <option value="light">Light</option>
                        <option value="dark">Dark</option>
                      </select>
                    </Field>
                    <Field label="Dashboard refresh interval (seconds)">
                      <input
                        required
                        type="number"
                        min={3}
                        max={60}
                        value={d.dashboard.refresh_seconds}
                        onChange={(event) =>
                          e.setDraft({
                            ...d,
                            dashboard: {
                              ...d.dashboard,
                              refresh_seconds: Number(event.target.value),
                            },
                          })
                        }
                      />
                    </Field>
                    <Field label="Default channel startup">
                      <select
                        value={d.dashboard.default_startup}
                        onChange={(event) =>
                          e.setDraft({
                            ...d,
                            dashboard: {
                              ...d.dashboard,
                              default_startup: event.target
                                .value as typeof d.dashboard.default_startup,
                            },
                          })
                        }
                      >
                        <option value="on_demand">On demand</option>
                        <option value="always">Always receive</option>
                      </select>
                    </Field>
                    <div className="op-field">
                      <span>Service process startup • read-only</span>
                      <p>
                      Use the host service manager to control startup and restart
                      the daemon.
                      </p>
                    </div>
                  </div>
                )}
                {tab === "Streaming" && (
                  <>
                    <div className="op-form-grid">
                      <Field label="Streaming engine enabled">
                        <select
                          value={String(d.streaming.enabled)}
                          onChange={(event) =>
                            updateStream("enabled", event.target.value === "true")
                          }
                        >
                          <option value="true">Enabled</option>
                          <option value="false">Disabled</option>
                        </select>
                      </Field>
                      {numeric(
                        "Default UDP receive buffer (bytes)",
                        "udp_receive_buffer",
                        65536,
                        67108864,
                      )}
                      {numeric(
                        "Default client buffer (bytes)",
                        "client_buffer_bytes",
                        564,
                        8388608,
                      )}
                      {numeric("Global maximum clients", "max_clients", 1, 100000)}
                      {numeric(
                        "Maximum configured streams",
                        "max_streams",
                        1,
                        1024,
                      )}
                      {numeric(
                        "Stream queued-byte budget",
                        "stream_buffer",
                        65536,
                        268435456,
                      )}
                      {numeric(
                        "Global queued-byte budget",
                        "max_buffered_bytes",
                        65536,
                        1073741824,
                      )}
                      <Field label="Default new-channel idle timeout (seconds)">
                        <input
                          type="number"
                          required
                          min={0}
                          max={86400}
                          value={d.dashboard.default_idle_seconds}
                          onChange={(event) =>
                            e.setDraft({
                              ...d,
                              dashboard: {
                                ...d.dashboard,
                                default_idle_seconds: Number(event.target.value),
                              },
                            })
                          }
                        />
                      </Field>
                      {(
                        [
                          ["Inactivity threshold", "inactivity_timeout"],
                          ["First-payload startup timeout", "connect_timeout"],
                          ["Client write deadline", "client_write_timeout"],
                        ] as const
                      ).map(([label, key]) => (
                        <Field
                          key={key}
                          label={label}
                          help="Duration such as 10s or 1m"
                        >
                          <input
                            required
                            value={d.streaming[key]}
                            onChange={(event) =>
                              updateStream(key, event.target.value)
                            }
                          />
                        </Field>
                      ))}
                    </div>
                    <div className="op-prose">
                      <strong>Receiver recovery policy • read-only</strong>
                      <p>{String(data.settings.read_only.recovery_policy)}</p>
                      <p>
                        Default idle/startup preferences affect newly created
                        channels. Existing channels keep their saved settings.
                      </p>
                    </div>
                  </>
                )}
                {tab === "HTTP" && (
                  <>
                    <NetworkFields
                      value={d.network}
                      onChange={(network) => e.setDraft({ ...d, network })}
                      interfaces={data.interfaces}
                    />
                    <div className="op-form-grid">
                      {Object.entries(d.timeouts).map(([key, value]) => (
                        <Field
                          key={key}
                          label={`HTTP ${key.replaceAll("_", " ")}`}
                          help="Duration such as 10s; restart required"
                        >
                          <input
                            required
                            value={value}
                            onChange={(event) =>
                              e.setDraft({
                                ...d,
                                timeouts: {
                                  ...d.timeouts,
                                  [key]: event.target.value,
                                },
                              })
                            }
                          />
                        </Field>
                      ))}
                    </div>
                  </>
                )}
                {tab === "Security" && (
                  <>
                    <div className="op-form-grid">
                      {list(
                        "Allowed playback client CIDRs",
                        "allowed_client_networks",
                        "Empty denies all. Actual peer IP only; canonical IPv4 CIDRs.",
                      )}
                      {list(
                        "Allowed multicast groups",
                        "allowed_multicast_groups",
                        "Exact literal group IPv4 addresses.",
                      )}
                      {list(
                        "Allowed unicast sender IPs",
                        "allowed_unicast_sources",
                        "Exact IPv4 sender addresses, not pull URLs.",
                      )}
                      {list(
                        "Blocked multicast networks",
                        "blocked_multicast_networks",
                        "Canonical multicast CIDRs; reserved control range is blocked by default.",
                      )}
                      <Field label="Explicit loopback unicast permission">
                        <select
                          value={String(d.streaming.allow_loopback_unicast)}
                          onChange={(event) =>
                            updateStream(
                              "allow_loopback_unicast",
                              event.target.value === "true",
                            )
                          }
                        >
                          <option value="false">Denied</option>
                          <option value="true">
                            Allowed only for exact allowlisted senders
                          </option>
                        </select>
                      </Field>
                    </div>
                    <Definition
                      items={[
                        [
                          "Management policy • read-only",
                          String(data.settings.read_only.authentication),
                        ],
                        [
                          "Session status • read-only",
                          "Browser sessions are not managed by the daemon",
                        ],
                        [
                          "Management bind • read-only",
                          String(data.settings.read_only.management_binding),
                        ],
                        [
                          "Trusted reverse proxies • read-only",
                          Array.isArray(data.settings.read_only.trusted_proxies) &&
                            data.settings.read_only.trusted_proxies.length
                            ? data.settings.read_only.trusted_proxies.join(", ")
                            : "None; forwarded headers ignored",
                        ],
                        [
                          "Trusted management origins • read-only",
                          Array.isArray(data.settings.read_only.trusted_origins) &&
                            data.settings.read_only.trusted_origins.length
                            ? data.settings.read_only.trusted_origins.join(", ")
                            : "None",
                        ],
                        [
                          "Playback authorization • read-only",
                          String(data.settings.read_only.playback_authorization),
                        ],
                      ]}
                    />
                  </>
                )}
                {tab === "Logging" && (
                  <>
                    <div className="op-form-grid">
                      <Field label="Log level">
                        <select
                          value={d.logging.level}
                          onChange={(event) =>
                            e.setDraft({
                              ...d,
                              logging: { ...d.logging, level: event.target.value },
                            })
                          }
                        >
                          {["debug", "info", "warn", "error"].map((level) => (
                            <option key={level}>{level}</option>
                          ))}
                        </select>
                      </Field>
                      <Field label="Maximum retained operational events">
                        <input
                          required
                          type="number"
                          min={100}
                          max={10000}
                          value={d.logging.retention_count}
                          onChange={(event) =>
                            e.setDraft({
                              ...d,
                              logging: {
                                ...d.logging,
                                retention_count: Number(event.target.value),
                              },
                            })
                          }
                        />
                      </Field>
                      <Field label="Retention age (days)">
                        <input
                          required
                          type="number"
                          min={1}
                          max={30}
                          value={d.logging.retention_days}
                          onChange={(event) =>
                            e.setDraft({
                              ...d,
                              logging: {
                                ...d.logging,
                                retention_days: Number(event.target.value),
                              },
                            })
                          }
                        />
                      </Field>
                    </div>
                    <div className="op-prose">
                      <p>
                        Retention applies to the bounded operational event store.
                        Raw file logging is disabled (read-only). Event queue
                        overflow is measured on System Information.
                      </p>
                      <a className="op-link" href="#/logs">
                        Open sanitized diagnostic exports
                      </a>
                    </div>
                  </>
                )}
                <div className="op-form-actions">
                  <CopyButton
                    label="Copy recovery command"
                    value="UDPHTTP_IGNORE_SAVED_SETTINGS=1 udphttp-proxy serve --config /etc/udphttp-proxy/config.yaml"
                  />
                  <Button
                    type="submit"
                    className="primary"
                    disabled={e.busy || e.revision !== data.settings.revision}
                  >
                    {e.busy ? "Validating…" : "Validate & review changes"}
                  </Button>
                </div>
              </form>
            )}
          </Panel>
        </div>
      </div>
      {e.dialog}
    </>
  );
}
