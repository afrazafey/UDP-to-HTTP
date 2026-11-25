import { useId, useState } from "react";
import { Check, FileText, Globe, ListMusic, Radio, Settings2, ShieldCheck } from "lucide-react";
import { SecuritySettings } from "../components/network/SecuritySettings";
import type { OperationsData } from "../types/operations";
import { Button, CopyButton, Definition, Field, Panel } from "../components/ui";
import {
  NetworkFields,
  useSettingsEditor,
} from "../components/network/SettingsEditor";
const sections = [
  { name: "General", icon: Settings2, description: "Dashboard appearance, refresh interval, and defaults for new channels." },
  { name: "Streaming", icon: Radio, description: "Receiver limits, buffer budgets, and playback timeouts." },
  { name: "HTTP", icon: Globe, description: "Playback interfaces and ports. Listener changes take effect after a restart." },
  { name: "Security", icon: ShieldCheck, description: "Choose which clients and sources can stream. Saved access rules take effect immediately." },
  { name: "Logging", icon: FileText, description: "Control log detail and how long operational events are retained." },
  { name: "Recovery & playlists", icon: ListMusic, description: "Inspect the active YAML configuration for recovery and remote playlists." },
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
  return (
    <>
      {e.status}
      <div className="op-settings-layout">
        <aside className="op-settings-sidebar">
          <div className="op-settings-nav-title"><Settings2 size={16} aria-hidden="true" /> Configuration</div>
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
        </aside>
        <div role="tabpanel" id={`${tabId}-panel`} aria-labelledby={`${tabId}-tab-${selected}`} tabIndex={0}>
          <Panel
            title={`${tab} settings`}
            description={sections[selected].description}
            action={e.dirty && <span className="op-settings-draft">Unsaved changes</span>}
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
                    "Remote playlist client networks",
                    Array.isArray(data.settings.read_only.playlist_allowed_client_networks)
                      ? data.settings.read_only.playlist_allowed_client_networks.join(", ") || "Empty"
                      : "Unavailable",
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
                  <SecuritySettings
                    key={e.revision}
                    draft={d}
                    readOnly={data.settings.read_only}
                    onChange={(streaming) => e.setDraft({ ...d, streaming })}
                  />
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
                  <span className="op-settings-revision" aria-live="polite">
                    <Check size={15} aria-hidden="true" />
                    Saved revision {e.revision}
                  </span>
                  <div className="op-settings-actions">
                    {e.dirty && <Button type="button" disabled={e.busy} onClick={e.reset}>Reset draft</Button>}
                    <Button
                    type="submit"
                    className="primary"
                    disabled={e.busy || e.revision < data.settings.revision}
                  >
                    {e.busy ? "Validating…" : "Validate & review changes"}
                    </Button>
                  </div>
                </div>
                {tab === "General" && (
                  <details className="op-settings-recovery">
                    <summary>Startup recovery</summary>
                    <p>Start with the server YAML when saved settings prevent normal startup.</p>
                    <CopyButton
                      label="Copy recovery command"
                      value="UDPHTTP_IGNORE_SAVED_SETTINGS=1 udphttp-proxy serve --config /etc/udphttp-proxy/config.yaml"
                    />
                  </details>
                )}
              </form>
            )}
          </Panel>
        </div>
      </div>
      {e.dialog}
    </>
  );
}
