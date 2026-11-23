import { useState } from "react";
import type {
  NetworkInterface,
  Settings,
  SettingsView,
} from "../../types/operations";
import { mutate } from "../../services/api";
import {
  Button,
  CopyButton,
  Dialog,
  ErrorBanner,
  Field,
  useAction,
} from "../ui";
export function NetworkFields({
  value,
  onChange,
  interfaces,
}: {
  value: Settings["network"];
  onChange: (network: Settings["network"]) => void;
  interfaces: NetworkInterface[];
}) {
  const choices = interfaces.filter((i) => i.up && i.ipv4.length);
  const available = (selection: string, multicast = false) =>
    choices.some(
      (i) =>
        (!multicast || i.multicast) &&
        (i.name === selection || i.ipv4.includes(selection)),
    );
  return (
    <div className="op-form-grid">
      <Field
        label="UDP multicast interface"
        help="Default interface selection; existing channels retain their explicit interface."
      >
        <select
          value={value.multicast_interface}
          onChange={(e) =>
            onChange({ ...value, multicast_interface: e.target.value })
          }
        >
          <option value="">No default selected</option>
          {value.multicast_interface &&
            !available(value.multicast_interface, true) && (
              <option value={value.multicast_interface} disabled>
                {value.multicast_interface} (unavailable)
              </option>
            )}
          {choices
            .filter((i) => i.multicast)
            .flatMap((i) => [
              <option key={i.name} value={i.name}>
                {i.name} • {i.ipv4.join(", ")}
              </option>,
              ...i.ipv4.map((ip) => (
                <option key={`${i.name}-${ip}`} value={ip}>
                  {ip} • {i.name}
                </option>
              )),
            ])}
        </select>
      </Field>
      <Field
        label="HTTP playback interface"
        help="Select an actual local interface or assigned IPv4 address."
      >
        <select
          required
          value={value.http_interface}
          onChange={(e) =>
            onChange({
              ...value,
              http_interface: e.target.value,
              http_bind_addresses: [],
            })
          }
        >
          {!available(value.http_interface) && (
            <option value={value.http_interface} disabled>
              {value.http_interface || "Select an interface"} (unavailable)
            </option>
          )}
          {choices.flatMap((i) => [
            <option key={i.name} value={i.name}>
              {i.name} • {i.ipv4.join(", ")}
            </option>,
            ...i.ipv4.map((ip) => (
              <option key={`${i.name}-${ip}`} value={ip}>
                {ip} • {i.name}
              </option>
            )),
          ])}
        </select>
      </Field>
      <Field label="Primary HTTP playback port">
        <input
          type="number"
          min={1}
          max={65535}
          required
          value={value.http_port}
          onChange={(e) =>
            onChange({ ...value, http_port: Number(e.target.value) })
          }
        />
      </Field>
      <Field
        label="Secondary HTTP playback port"
        help="0 disables the secondary listener."
      >
        <input
          type="number"
          min={0}
          max={65535}
          required
          value={value.secondary_http_port}
          onChange={(e) =>
            onChange({ ...value, secondary_http_port: Number(e.target.value) })
          }
        />
      </Field>
      <Field
        label="Explicit playback bind addresses"
        help="Optional comma-separated assigned IPv4 addresses. 0.0.0.0 is an explicit wildcard and must stand alone."
      >
        <input
          value={(value.http_bind_addresses ?? []).join(", ")}
          onChange={(e) =>
            onChange({
              ...value,
              http_bind_addresses: e.target.value
                .split(",")
                .map((s) => s.trim()),
            })
          }
        />
      </Field>
    </div>
  );
}
export function useSettingsEditor(view: SettingsView, refresh: () => void) {
  const [draft, setDraft] = useState<Settings>(() =>
    structuredClone(view.settings),
  );
  const [revision, setRevision] = useState(view.revision);
  const [preview, setPreview] = useState<{
    settings: Settings;
    notes: string[];
  } | null>(null);
  const [rollback, setRollback] = useState(false);
  const action = useAction();
  const clean = () => {
    const copy = structuredClone(draft);
    copy.network.http_bind_addresses =
      copy.network.http_bind_addresses?.filter(Boolean);
    copy.streaming.allowed_client_networks =
      copy.streaming.allowed_client_networks.filter(Boolean);
    copy.streaming.allowed_multicast_groups =
      copy.streaming.allowed_multicast_groups.filter(Boolean);
    copy.streaming.allowed_unicast_sources =
      copy.streaming.allowed_unicast_sources.filter(Boolean);
    copy.streaming.blocked_multicast_networks =
      copy.streaming.blocked_multicast_networks.filter(Boolean);
    return copy;
  };
  const validate = async () => {
    const proposal = clean();
    await action.run(async () => {
      const result = await mutate<{ notes: string[] }>(
        "/api/v1/settings/validate",
        "POST",
        { revision, settings: proposal },
      );
      setPreview({ settings: proposal, notes: result.notes });
    }, "Configuration validation completed");
  };
  const dialog = preview ? (
    <Dialog
      title="Review configuration changes"
      onClose={() => !action.busy && setPreview(null)}
      footer={
        <>
          <Button disabled={action.busy} onClick={() => setPreview(null)}>
            Keep editing
          </Button>
          <Button
            className="primary"
            disabled={action.busy}
            onClick={async () => {
              if (
                await action.run(async () => {
                  const result = await mutate<SettingsView>(
                    "/api/v1/settings",
                    "PUT",
                    { revision, settings: preview.settings, confirm: true },
                  );
                  setRevision(result.revision);
                  setDraft(result.settings);
                }, "Settings saved and confirmed by the daemon")
              ) {
                setPreview(null);
                refresh();
              }
            }}
          >
            {action.busy ? "Saving…" : "Confirm save"}
          </Button>
        </>
      }
    >
      <p>
        Playback listener and streaming changes apply after a process restart.
        Management binding stays unchanged. A failed playback bind restores the
        last accepted settings; local management remains available.
      </p>
      {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
      <ul className="op-notes">
        {preview.notes.map((note) => (
          <li key={note}>{note}</li>
        ))}
      </ul>
      <details>
        <summary>Exact configuration preview</summary>
        <pre>{JSON.stringify(preview.settings, null, 2)}</pre>
      </details>
    </Dialog>
  ) : rollback ? (
    <Dialog
      title="Discard pending configuration"
      onClose={() => !action.busy && setRollback(false)}
      footer={
        <>
          <Button onClick={() => setRollback(false)} disabled={action.busy}>
            Cancel
          </Button>
          <Button
            disabled={action.busy}
            onClick={async () => {
              if (
                await action.run(async () => {
                  const result = await mutate<SettingsView>(
                    "/api/v1/settings/rollback",
                    "POST",
                    { revision: view.revision, confirm: true },
                  );
                  setDraft(result.settings);
                  setRevision(result.revision);
                }, "Last accepted settings restored")
              ) {
                setRollback(false);
                refresh();
              }
            }}
          >
            Restore accepted settings
          </Button>
        </>
      }
    >
      <p>
        This discards pending settings and restores the last successfully
        accepted configuration. It does not stop the service or change host
        networking.
      </p>
      {action.error && <ErrorBanner>{action.error}</ErrorBanner>}
    </Dialog>
  ) : null;
  const status = (
    <>
      {view.restart_required && (
        <div className="op-callout">
          <strong>Restart required</strong>
          <p>
            Saved playback/server settings differ from the running daemon.
            Restart using your service manager after reviewing active clients.
          </p>
          <CopyButton
            label="Copy restart command"
            value="sudo systemctl restart udphttp-proxy"
          />
          <Button onClick={() => setRollback(true)} disabled={action.busy}>
            Discard pending changes
          </Button>
        </div>
      )}
      {view.last_error && (
        <ErrorBanner>
          Previous startup restored accepted settings: {view.last_error}
        </ErrorBanner>
      )}
      {revision !== view.revision && (
        <ErrorBanner>
          Settings changed since this form loaded. Reload the latest revision
          before saving.{" "}
          <Button
            onClick={() => {
              setDraft(structuredClone(view.settings));
              setRevision(view.revision);
            }}
          >
            Reload saved settings
          </Button>
        </ErrorBanner>
      )}
      {action.error && !preview && !rollback && (
        <ErrorBanner>{action.error}</ErrorBanner>
      )}
    </>
  );
  return {
    draft,
    setDraft,
    revision,
    validate,
    dialog,
    status,
    busy: action.busy,
  };
}
